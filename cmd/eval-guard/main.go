package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/evals"
)

const defaultGuardOutDir = "tests/evals/results"

type config struct {
	RunArtifactPath string
	OutPath         string
	Category        string
	Limit           int
	AllowFailures   bool
}

type baselineRunArtifact struct {
	Dataset string             `json:"dataset"`
	Cases   []baselineCaseItem `json:"cases"`
}

type baselineCaseItem struct {
	ID            string           `json:"id"`
	Category      string           `json:"category"`
	Message       string           `json:"message"`
	Expected      any              `json:"expected"`
	Success       bool             `json:"success"`
	Error         string           `json:"error"`
	AssistantText string           `json:"assistant_text"`
	ToolResults   int              `json:"tool_results"`
	ToolErrors    int              `json:"tool_errors"`
	ToolTrace     []map[string]any `json:"tool_trace"`
}

type guardReport struct {
	SchemaVersion string                  `json:"schema_version"`
	GeneratedAt   string                  `json:"generated_at"`
	Dataset       string                  `json:"dataset"`
	RunArtifact   string                  `json:"run_artifact"`
	Rules         []string                `json:"rules"`
	Totals        evals.GuardTotals       `json:"totals"`
	Cases         []evals.GuardCaseResult `json:"cases"`
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.RunArtifactPath, "run-artifact", "", "Path to baseline run artifact JSON (required)")
	flag.StringVar(&cfg.OutPath, "out", "", "Output guard report path (default: tests/evals/results/guard-<timestamp>.json)")
	flag.StringVar(&cfg.Category, "category", "", "Only evaluate one category from run artifact")
	flag.IntVar(&cfg.Limit, "limit", 0, "Max number of cases to evaluate (0 = all)")
	flag.BoolVar(&cfg.AllowFailures, "allow-failures", false, "Do not fail command when guard violations are found")
	flag.Parse()
	return cfg
}

func run(cfg config) error {
	if cfg.RunArtifactPath == "" {
		return errors.New("--run-artifact is required")
	}
	if cfg.Limit < 0 {
		return errors.New("--limit must be >= 0")
	}

	artifact, err := loadRunArtifact(cfg.RunArtifactPath)
	if err != nil {
		return err
	}
	selected := selectCases(artifact.Cases, cfg.Category, cfg.Limit)
	if len(selected) == 0 {
		return fmt.Errorf("no cases selected (category=%q limit=%d)", cfg.Category, cfg.Limit)
	}

	if cfg.OutPath == "" {
		cfg.OutPath = filepath.Join(defaultGuardOutDir, fmt.Sprintf("guard-%s.json", time.Now().Format("20060102-150405")))
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	fmt.Println("Running deterministic eval guards")
	fmt.Printf("  run_artifact: %s\n", cfg.RunArtifactPath)
	fmt.Printf("  cases:        %d\n", len(selected))
	if cfg.Category != "" {
		fmt.Printf("  category:     %s\n", cfg.Category)
	}
	fmt.Printf("  out_file:     %s\n", cfg.OutPath)

	results := make([]evals.GuardCaseResult, 0, len(selected))
	for i, c := range selected {
		in := evals.GuardCaseInput{
			ID:            c.ID,
			Category:      c.Category,
			Message:       c.Message,
			AssistantText: c.AssistantText,
			Success:       c.Success,
			Error:         c.Error,
			ToolResults:   c.ToolResults,
			ToolErrors:    c.ToolErrors,
			ExpectedTools: expectedToolsFromCase(c.Expected),
			UsedTools:     evals.UniqueToolNamesFromTrace(c.ToolTrace),
		}
		res := evals.EvaluateDeterministicGuards(in)
		results = append(results, res)

		if res.Passed {
			fmt.Printf("[%d/%d] %s (%s): pass\n", i+1, len(selected), c.ID, c.Category)
		} else {
			fmt.Printf("[%d/%d] %s (%s): fail (%d violation(s))\n", i+1, len(selected), c.ID, c.Category, len(res.Violations))
		}
	}

	totals := evals.BuildGuardTotals(results)
	report := guardReport{
		SchemaVersion: evals.GuardReportSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Dataset:       artifact.Dataset,
		RunArtifact:   cfg.RunArtifactPath,
		Rules: []string{
			evals.GuardRuleNoFabricatedMetricValues,
			evals.GuardRuleToolErrorHandling,
			evals.GuardRuleDocsEvidenceSignals,
		},
		Totals: totals,
		Cases:  results,
	}
	if err := writeJSON(cfg.OutPath, report); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Deterministic guard evaluation complete")
	fmt.Printf("  output:            %s\n", cfg.OutPath)
	fmt.Printf("  passed/failed:     %d/%d\n", totals.PassedCases, totals.FailedCases)
	fmt.Printf("  violations total:  %d\n", totals.ViolationsTotal)
	for _, rule := range report.Rules {
		fmt.Printf("  - %s: %d\n", rule, totals.ViolationsByRule[rule])
	}

	if totals.FailedCases > 0 && !cfg.AllowFailures {
		return fmt.Errorf("guard checks failed for %d case(s); rerun with --allow-failures to inspect without failing", totals.FailedCases)
	}
	return nil
}

func loadRunArtifact(path string) (baselineRunArtifact, error) {
	var out baselineRunArtifact
	if err := readJSON(path, &out); err != nil {
		return out, fmt.Errorf("load run artifact %s: %w", path, err)
	}
	if len(out.Cases) == 0 {
		return out, fmt.Errorf("run artifact has no cases: %s", path)
	}
	return out, nil
}

func selectCases(in []baselineCaseItem, category string, limit int) []baselineCaseItem {
	out := make([]baselineCaseItem, 0, len(in))
	for _, c := range in {
		if category != "" && c.Category != category {
			continue
		}
		out = append(out, c)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func expectedToolsFromCase(expected any) []string {
	m, ok := expected.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := m["expected_tools"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
