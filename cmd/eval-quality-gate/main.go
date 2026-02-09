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

	"github.com/brownster/grafana-assistant/internal/evals"
)

const defaultOutDir = "tests/evals/results"

type config struct {
	JudgeReportPath string
	GuardReportPath string
	OutPath         string
	SummaryMDPath   string
	AllowFailures   bool
	JudgeGateMode   bool
	Thresholds      evals.QualityGateThresholds
}

type qualityGateReport struct {
	SchemaVersion string                  `json:"schema_version"`
	GeneratedAt   string                  `json:"generated_at"`
	JudgeReport   string                  `json:"judge_report"`
	GuardReport   string                  `json:"guard_report"`
	JudgeGateMode bool                    `json:"judge_gate_mode"`
	Result        evals.QualityGateResult `json:"result"`
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func parseFlags() config {
	thresholds := evals.DefaultQualityGateThresholds()
	judgeGateMode := parseFeatureFlagEnv("ASSISTANT_FEATURE_JUDGE_GATE_MODE", true)
	cfg := config{
		Thresholds:    thresholds,
		JudgeGateMode: judgeGateMode,
	}

	flag.StringVar(&cfg.JudgeReportPath, "judge-report", "", "Path to judge report JSON (required)")
	flag.StringVar(&cfg.GuardReportPath, "guard-report", "", "Path to guard report JSON (required)")
	flag.StringVar(&cfg.OutPath, "out", "", "Output quality gate report path (default: tests/evals/results/quality-gate-<timestamp>.json)")
	flag.StringVar(&cfg.SummaryMDPath, "summary-md", "", "Optional markdown summary output path")
	flag.BoolVar(&cfg.AllowFailures, "allow-failures", false, "Do not fail command when threshold checks fail")
	flag.BoolVar(&cfg.JudgeGateMode, "judge-gate-mode", judgeGateMode, "Enable judge/guard quality gate enforcement (env: ASSISTANT_FEATURE_JUDGE_GATE_MODE)")

	flag.Float64Var(&cfg.Thresholds.MinPassRate, "min-pass-rate", thresholds.MinPassRate, "Minimum judge aggregate pass_rate (0..1)")
	flag.IntVar(&cfg.Thresholds.MinCaseCount, "min-case-count", thresholds.MinCaseCount, "Minimum judge aggregate case_count")
	flag.IntVar(&cfg.Thresholds.MaxHallucinationHardFailures, "max-hallucination-hard-failures", thresholds.MaxHallucinationHardFailures, "Maximum allowed hallucination_risk hard failures")
	flag.IntVar(&cfg.Thresholds.MaxJudgeErrorCases, "max-judge-error-cases", thresholds.MaxJudgeErrorCases, "Maximum allowed judge_error case count")
	flag.IntVar(&cfg.Thresholds.MaxGuardFailedCases, "max-guard-failed-cases", thresholds.MaxGuardFailedCases, "Maximum allowed deterministic guard failed cases")
	flag.IntVar(&cfg.Thresholds.MaxGuardViolationsTotal, "max-guard-violations-total", thresholds.MaxGuardViolationsTotal, "Maximum allowed deterministic guard total violations")
	flag.BoolVar(&cfg.Thresholds.RequireDatasetMatch, "require-dataset-match", thresholds.RequireDatasetMatch, "Require judge and guard dataset fields to match")
	flag.BoolVar(&cfg.Thresholds.RequireRunArtifactMatch, "require-run-artifact-match", thresholds.RequireRunArtifactMatch, "Require judge and guard run_artifact fields to match")
	flag.BoolVar(&cfg.Thresholds.RequireCaseCountMatch, "require-case-count-match", thresholds.RequireCaseCountMatch, "Require judge and guard case counts to match")
	flag.Parse()
	return cfg
}

func run(cfg config) error {
	if strings.TrimSpace(cfg.JudgeReportPath) == "" {
		return errors.New("--judge-report is required")
	}
	if strings.TrimSpace(cfg.GuardReportPath) == "" {
		return errors.New("--guard-report is required")
	}

	var judge evals.JudgeScoreReport
	if err := readJSON(cfg.JudgeReportPath, &judge); err != nil {
		return fmt.Errorf("load judge report %s: %w", cfg.JudgeReportPath, err)
	}
	var guard evals.GuardCheckReport
	if err := readJSON(cfg.GuardReportPath, &guard); err != nil {
		return fmt.Errorf("load guard report %s: %w", cfg.GuardReportPath, err)
	}

	result, err := evals.EvaluateQualityGate(judge, guard, cfg.Thresholds)
	if err != nil {
		return err
	}

	if cfg.OutPath == "" {
		cfg.OutPath = filepath.Join(defaultOutDir, fmt.Sprintf("quality-gate-%s.json", time.Now().Format("20060102-150405")))
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	report := qualityGateReport{
		SchemaVersion: evals.QualityGateSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		JudgeReport:   cfg.JudgeReportPath,
		GuardReport:   cfg.GuardReportPath,
		JudgeGateMode: cfg.JudgeGateMode,
		Result:        result,
	}
	if err := writeJSON(cfg.OutPath, report); err != nil {
		return err
	}

	markdown := renderMarkdown(report)
	if cfg.SummaryMDPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.SummaryMDPath), 0o755); err != nil {
			return fmt.Errorf("create summary markdown directory: %w", err)
		}
		if err := os.WriteFile(cfg.SummaryMDPath, []byte(markdown), 0o644); err != nil {
			return fmt.Errorf("write summary markdown: %w", err)
		}
	}

	fmt.Println("Eval quality gate complete")
	fmt.Printf("  judge_report:  %s\n", cfg.JudgeReportPath)
	fmt.Printf("  guard_report:  %s\n", cfg.GuardReportPath)
	fmt.Printf("  output:        %s\n", cfg.OutPath)
	if cfg.SummaryMDPath != "" {
		fmt.Printf("  summary_md:    %s\n", cfg.SummaryMDPath)
	}
	fmt.Printf("  judge_gate:    %s\n", ternary(cfg.JudgeGateMode, "enabled", "disabled"))
	fmt.Printf("  status:        %s\n", ternary(result.Passed, "pass", "fail"))
	fmt.Printf("  pass_rate:     %.4f\n", result.Metrics.JudgePassRate)
	fmt.Printf("  guard_failed:  %d\n", result.Metrics.GuardFailedCases)
	fmt.Printf("  guard_viols:   %d\n", result.Metrics.GuardViolationsTotal)
	fmt.Printf("  hallu_hard:    %d\n", result.Metrics.JudgeHallucinationHardFailures)
	fmt.Printf("  judge_errors:  %d\n", result.Metrics.JudgeErrorCases)

	if !cfg.JudgeGateMode {
		return nil
	}
	if !result.Passed && !cfg.AllowFailures {
		return fmt.Errorf("quality gate failed (%d checks failed)", countFailedChecks(result.Checks))
	}
	return nil
}

func renderMarkdown(report qualityGateReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Eval Quality Gate\n\n")
	fmt.Fprintf(&b, "- Status: **%s**\n", strings.ToUpper(ternary(report.Result.Passed, "pass", "fail")))
	fmt.Fprintf(&b, "- Judge report: `%s`\n", report.JudgeReport)
	fmt.Fprintf(&b, "- Guard report: `%s`\n", report.GuardReport)
	fmt.Fprintf(&b, "- Judge gate mode: `%s`\n", ternary(report.JudgeGateMode, "enabled", "disabled"))
	fmt.Fprintf(&b, "- Generated at: `%s`\n", report.GeneratedAt)
	fmt.Fprintf(&b, "\n### Metrics\n\n")
	fmt.Fprintf(&b, "- Judge case count: `%d`\n", report.Result.Metrics.JudgeCaseCount)
	fmt.Fprintf(&b, "- Judge pass rate: `%.6f`\n", report.Result.Metrics.JudgePassRate)
	fmt.Fprintf(&b, "- Hallucination hard failures: `%d`\n", report.Result.Metrics.JudgeHallucinationHardFailures)
	fmt.Fprintf(&b, "- Judge error cases: `%d`\n", report.Result.Metrics.JudgeErrorCases)
	fmt.Fprintf(&b, "- Guard failed cases: `%d`\n", report.Result.Metrics.GuardFailedCases)
	fmt.Fprintf(&b, "- Guard total violations: `%d`\n", report.Result.Metrics.GuardViolationsTotal)
	fmt.Fprintf(&b, "\n### Checks\n\n")
	for _, c := range report.Result.Checks {
		state := "PASS"
		if !c.Passed {
			state = "FAIL"
		}
		fmt.Fprintf(&b, "- [%s] `%s` actual=`%s` threshold=`%s` (%s)\n", state, c.ID, c.Actual, c.Threshold, c.Message)
	}
	return b.String()
}

func countFailedChecks(checks []evals.QualityGateCheck) int {
	n := 0
	for _, c := range checks {
		if !c.Passed {
			n++
		}
	}
	return n
}

func ternary(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
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

func parseFeatureFlagEnv(key string, defaultValue bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	switch trimmed {
	case "1", "true", "yes", "on", "enabled":
		return true
	case "0", "false", "no", "off", "disabled":
		return false
	default:
		return defaultValue
	}
}
