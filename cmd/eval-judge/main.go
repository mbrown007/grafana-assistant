package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/marcusz/monitoring-assistant/internal/evals"
)

const (
	defaultRubricPath = "docs/evals/judge-rubric.v1.json"
	defaultOutDir     = "tests/evals/results"
	defaultModel      = "gpt-4o-mini"
)

var caseJudgeResponseSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["dimension_scores", "rationales", "evidence_summary"],
  "properties": {
    "dimension_scores": {
      "type": "object",
      "additionalProperties": false,
      "required": [
        "factuality",
        "answer_completeness",
        "tool_usage_correctness",
        "hallucination_risk",
        "actionability"
      ],
      "properties": {
        "factuality": { "type": "integer", "minimum": 0, "maximum": 4 },
        "answer_completeness": { "type": "integer", "minimum": 0, "maximum": 4 },
        "tool_usage_correctness": { "type": "integer", "minimum": 0, "maximum": 4 },
        "hallucination_risk": { "type": "integer", "minimum": 0, "maximum": 4 },
        "actionability": { "type": "integer", "minimum": 0, "maximum": 4 }
      }
    },
    "rationales": {
      "type": "object",
      "additionalProperties": false,
      "required": [
        "factuality",
        "answer_completeness",
        "tool_usage_correctness",
        "hallucination_risk",
        "actionability"
      ],
      "properties": {
        "factuality": { "type": "string", "minLength": 1 },
        "answer_completeness": { "type": "string", "minLength": 1 },
        "tool_usage_correctness": { "type": "string", "minLength": 1 },
        "hallucination_risk": { "type": "string", "minLength": 1 },
        "actionability": { "type": "string", "minLength": 1 }
      }
    },
    "evidence_summary": { "type": "string", "minLength": 1 }
  }
}`)

type config struct {
	RunArtifactPath string
	RubricPath      string
	OutPath         string
	Model           string
	APIKey          string
	BaseURL         string

	Category string
	Limit    int

	TimeoutSeconds int
	Retries        int
	DelayMS        int
	FailFast       bool

	MaxToolTraceEvents  int
	MaxAssistantChars   int
	MaxCompletionTokens int
}

type baselineRunArtifact struct {
	Dataset string             `json:"dataset"`
	Cases   []baselineCaseItem `json:"cases"`
}

type baselineCaseItem struct {
	ID              string           `json:"id"`
	Category        string           `json:"category"`
	Message         string           `json:"message"`
	Expected        any              `json:"expected"`
	Success         bool             `json:"success"`
	HTTPStatus      int              `json:"http_status"`
	LatencyMS       int              `json:"latency_ms"`
	AssistantText   string           `json:"assistant_text"`
	Error           string           `json:"error"`
	ToolInvocations int              `json:"tool_invocations"`
	ToolResults     int              `json:"tool_results"`
	ToolErrors      int              `json:"tool_errors"`
	StreamErrors    []map[string]any `json:"stream_errors"`
	ToolTrace       []map[string]any `json:"tool_trace"`
}

type llmCaseEvaluation struct {
	DimensionScores evals.DimensionScores `json:"dimension_scores"`
	Rationales      evals.Rationales      `json:"rationales"`
	EvidenceSummary string                `json:"evidence_summary"`
}

type judgeCaseReport struct {
	ID              string                `json:"id"`
	Category        string                `json:"category"`
	Verdict         string                `json:"verdict"`
	DimensionScores evals.DimensionScores `json:"dimension_scores"`
	WeightedScore   float64               `json:"weighted_score"`
	HardFailures    []string              `json:"hard_failures"`
	Rationales      evals.Rationales      `json:"rationales"`
	EvidenceSummary string                `json:"evidence_summary"`
	JudgeError      string                `json:"judge_error,omitempty"`
}

type judgeReport struct {
	SchemaVersion string                 `json:"schema_version"`
	RubricID      string                 `json:"rubric_id"`
	GeneratedAt   string                 `json:"generated_at"`
	JudgeModel    string                 `json:"judge_model,omitempty"`
	Dataset       string                 `json:"dataset"`
	RunArtifact   string                 `json:"run_artifact,omitempty"`
	Cases         []judgeCaseReport      `json:"cases"`
	Aggregate     evals.AggregateSummary `json:"aggregate"`
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
	flag.StringVar(&cfg.RubricPath, "rubric", defaultRubricPath, "Path to judge rubric JSON")
	flag.StringVar(&cfg.OutPath, "out", "", "Output judge report path (default: tests/evals/results/judge-<timestamp>.json)")
	flag.StringVar(&cfg.Model, "model", "", "Judge model (default: ASSISTANT_EVAL_JUDGE_MODEL or ASSISTANT_OPENAI_MODEL or gpt-4o-mini)")
	flag.StringVar(&cfg.APIKey, "openai-api-key", "", "OpenAI API key (default: ASSISTANT_OPENAI_API_KEY or OPENAI_API_KEY)")
	flag.StringVar(&cfg.BaseURL, "openai-base-url", "", "Optional OpenAI base URL override")
	flag.StringVar(&cfg.Category, "category", "", "Only score one category from run artifact")
	flag.IntVar(&cfg.Limit, "limit", 0, "Max number of cases to score (0 = all)")
	flag.IntVar(&cfg.TimeoutSeconds, "timeout", 90, "Per-case judge timeout in seconds")
	flag.IntVar(&cfg.Retries, "retries", 2, "Retry attempts per case on judge/parsing errors")
	flag.IntVar(&cfg.DelayMS, "delay-ms", 120, "Delay between case requests in milliseconds")
	flag.BoolVar(&cfg.FailFast, "fail-fast", false, "Abort the run on the first case-level judge error")
	flag.IntVar(&cfg.MaxToolTraceEvents, "max-tool-trace-events", 12, "Max tool_trace events passed to judge context per case")
	flag.IntVar(&cfg.MaxAssistantChars, "max-assistant-chars", 6000, "Max assistant_text chars passed to judge context per case")
	flag.IntVar(&cfg.MaxCompletionTokens, "max-completion-tokens", 1200, "Max completion tokens for each judge LLM call")
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
	if cfg.TimeoutSeconds <= 0 {
		return errors.New("--timeout must be > 0")
	}
	if cfg.Retries < 0 {
		return errors.New("--retries must be >= 0")
	}
	if cfg.DelayMS < 0 {
		return errors.New("--delay-ms must be >= 0")
	}
	if cfg.MaxToolTraceEvents <= 0 {
		return errors.New("--max-tool-trace-events must be > 0")
	}
	if cfg.MaxAssistantChars < 256 {
		return errors.New("--max-assistant-chars must be >= 256")
	}
	if cfg.MaxCompletionTokens <= 0 {
		return errors.New("--max-completion-tokens must be > 0")
	}

	cfg.APIKey = firstNonEmpty(cfg.APIKey, os.Getenv("ASSISTANT_OPENAI_API_KEY"), os.Getenv("OPENAI_API_KEY"))
	if cfg.APIKey == "" {
		return errors.New("OpenAI API key is required (set --openai-api-key or ASSISTANT_OPENAI_API_KEY)")
	}
	cfg.Model = firstNonEmpty(cfg.Model, os.Getenv("ASSISTANT_EVAL_JUDGE_MODEL"), os.Getenv("ASSISTANT_OPENAI_MODEL"), defaultModel)
	cfg.BaseURL = firstNonEmpty(cfg.BaseURL, os.Getenv("ASSISTANT_OPENAI_BASE_URL"))

	rubric, err := loadRubric(cfg.RubricPath)
	if err != nil {
		return err
	}

	runArtifact, err := loadRunArtifact(cfg.RunArtifactPath)
	if err != nil {
		return err
	}
	if runArtifact.Dataset == "" {
		return fmt.Errorf("run artifact missing dataset field: %s", cfg.RunArtifactPath)
	}

	cases := selectCases(runArtifact.Cases, cfg.Category, cfg.Limit)
	if len(cases) == 0 {
		return fmt.Errorf("no cases selected (category=%q limit=%d)", cfg.Category, cfg.Limit)
	}

	if cfg.OutPath == "" {
		cfg.OutPath = filepath.Join(defaultOutDir, fmt.Sprintf("judge-%s.json", time.Now().Format("20060102-150405")))
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	clientCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientCfg.BaseURL = cfg.BaseURL
	}
	client := openai.NewClientWithConfig(clientCfg)

	fmt.Println("Running eval judge")
	fmt.Printf("  run_artifact: %s\n", cfg.RunArtifactPath)
	fmt.Printf("  rubric:       %s\n", cfg.RubricPath)
	fmt.Printf("  model:        %s\n", cfg.Model)
	fmt.Printf("  cases:        %d\n", len(cases))
	if cfg.Category != "" {
		fmt.Printf("  category:     %s\n", cfg.Category)
	}
	fmt.Printf("  out_file:     %s\n", cfg.OutPath)

	results := make([]judgeCaseReport, 0, len(cases))
	scoredCases := make([]evals.CaseScoreSummary, 0, len(cases))
	delay := time.Duration(cfg.DelayMS) * time.Millisecond
	judgeErrorCases := 0

	for i, c := range cases {
		start := time.Now()
		modelEval, err := judgeCaseWithRetry(client, cfg, rubric, c)
		if err != nil {
			if cfg.FailFast {
				return fmt.Errorf("judge failed for case %s: %w", c.ID, err)
			}
			judgeErrorCases++

			fallbackEval := llmCaseEvaluation{
				DimensionScores: evals.DimensionScores{},
				Rationales: evals.Rationales{
					Factuality:           "Judge call failed before scoring; defaulted to 0.",
					AnswerCompleteness:   "Judge call failed before scoring; defaulted to 0.",
					ToolUsageCorrectness: "Judge call failed before scoring; defaulted to 0.",
					HallucinationRisk:    "Judge call failed before scoring; defaulted to 0.",
					Actionability:        "Judge call failed before scoring; defaulted to 0.",
				},
				EvidenceSummary: "Judge error; case marked failed for traceability.",
			}
			scoreSummary, scoreErr := evals.EvaluateCase(rubric, fallbackEval.DimensionScores)
			if scoreErr != nil {
				return fmt.Errorf("fallback score evaluation failed for case %s: %w", c.ID, scoreErr)
			}
			reportCase := judgeCaseReport{
				ID:              c.ID,
				Category:        c.Category,
				Verdict:         scoreSummary.Verdict,
				DimensionScores: scoreSummary.DimensionScores,
				WeightedScore:   scoreSummary.WeightedScore,
				HardFailures:    scoreSummary.HardFailures,
				Rationales:      fallbackEval.Rationales,
				EvidenceSummary: fallbackEval.EvidenceSummary,
				JudgeError:      truncate(err.Error(), 500),
			}
			results = append(results, reportCase)
			scoredCases = append(scoredCases, scoreSummary)

			fmt.Printf("[%d/%d] %s (%s): fail (judge_error, hard_failures=%d, %s)\n",
				i+1, len(cases), c.ID, c.Category, len(reportCase.HardFailures), time.Since(start).Round(time.Millisecond))
			if i < len(cases)-1 && delay > 0 {
				time.Sleep(delay)
			}
			continue
		}

		scoreSummary, err := evals.EvaluateCase(rubric, modelEval.DimensionScores)
		if err != nil {
			return fmt.Errorf("score evaluation failed for case %s: %w", c.ID, err)
		}

		reportCase := judgeCaseReport{
			ID:              c.ID,
			Category:        c.Category,
			Verdict:         scoreSummary.Verdict,
			DimensionScores: scoreSummary.DimensionScores,
			WeightedScore:   scoreSummary.WeightedScore,
			HardFailures:    scoreSummary.HardFailures,
			Rationales:      modelEval.Rationales,
			EvidenceSummary: modelEval.EvidenceSummary,
		}
		results = append(results, reportCase)
		scoredCases = append(scoredCases, scoreSummary)

		fmt.Printf("[%d/%d] %s (%s): %s (score=%.3f, hard_failures=%d, %s)\n",
			i+1, len(cases), c.ID, c.Category, reportCase.Verdict, reportCase.WeightedScore, len(reportCase.HardFailures), time.Since(start).Round(time.Millisecond))

		if i < len(cases)-1 && delay > 0 {
			time.Sleep(delay)
		}
	}

	aggregate := evals.BuildAggregate(scoredCases)
	report := judgeReport{
		SchemaVersion: evals.JudgeScoreSchemaVersion,
		RubricID:      rubric.RubricID,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		JudgeModel:    cfg.Model,
		Dataset:       runArtifact.Dataset,
		RunArtifact:   cfg.RunArtifactPath,
		Cases:         results,
		Aggregate:     aggregate,
	}

	if err := writeJSON(cfg.OutPath, report); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Eval judge complete")
	fmt.Printf("  output:             %s\n", cfg.OutPath)
	fmt.Printf("  pass rate:          %.2f%%\n", aggregate.PassRate*100)
	fmt.Printf("  avg weighted score: %.3f\n", aggregate.AverageWeightedScore)
	fmt.Printf("  pass/fail:          %d/%d\n", aggregate.PassCount, aggregate.FailCount)
	fmt.Printf("  judge case errors:  %d\n", judgeErrorCases)
	fmt.Printf("  hard failures:      factuality=%d hallucination_risk=%d\n",
		aggregate.HardFailureCounts.Factuality, aggregate.HardFailureCounts.HallucinationRisk)
	return nil
}

func loadRubric(path string) (evals.Rubric, error) {
	var rubric evals.Rubric
	if err := readJSON(path, &rubric); err != nil {
		return rubric, fmt.Errorf("load rubric %s: %w", path, err)
	}
	if err := evals.ValidateRubric(rubric); err != nil {
		return rubric, fmt.Errorf("validate rubric %s: %w", path, err)
	}
	return rubric, nil
}

func loadRunArtifact(path string) (baselineRunArtifact, error) {
	var art baselineRunArtifact
	if err := readJSON(path, &art); err != nil {
		return art, fmt.Errorf("load run artifact %s: %w", path, err)
	}
	if len(art.Cases) == 0 {
		return art, fmt.Errorf("run artifact has no cases: %s", path)
	}
	return art, nil
}

func selectCases(in []baselineCaseItem, category string, limit int) []baselineCaseItem {
	filtered := make([]baselineCaseItem, 0, len(in))
	for _, c := range in {
		if category != "" && c.Category != category {
			continue
		}
		filtered = append(filtered, c)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	return filtered
}

func judgeCaseWithRetry(client *openai.Client, cfg config, rubric evals.Rubric, c baselineCaseItem) (llmCaseEvaluation, error) {
	var lastErr error
	for attempt := 1; attempt <= cfg.Retries+1; attempt++ {
		out, err := judgeCase(client, cfg, rubric, c)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if attempt < cfg.Retries+1 {
			backoff := time.Duration(attempt*350) * time.Millisecond
			time.Sleep(backoff)
		}
	}
	return llmCaseEvaluation{}, lastErr
}

func judgeCase(client *openai.Client, cfg config, rubric evals.Rubric, c baselineCaseItem) (llmCaseEvaluation, error) {
	rubricJSON, err := json.Marshal(rubric)
	if err != nil {
		return llmCaseEvaluation{}, fmt.Errorf("marshal rubric: %w", err)
	}

	casePayload, err := marshalCasePayload(c, cfg.MaxToolTraceEvents, cfg.MaxAssistantChars)
	if err != nil {
		return llmCaseEvaluation{}, err
	}

	systemPrompt := "You are a strict evaluator for monitoring assistant answers. " +
		"Return only JSON that matches the required schema. " +
		"Use only the provided rubric and case evidence. " +
		"Do not invent facts or hidden tool outputs."
	userPrompt := strings.Join([]string{
		"Score this case using the rubric dimensions exactly.",
		"If the assistant output is missing or failed, completeness and factuality should be low.",
		"Keep rationales concise and specific to the provided evidence.",
		"",
		"RUBRIC_JSON:",
		string(rubricJSON),
		"",
		"CASE_JSON:",
		string(casePayload),
	}, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        "judge_case_score_v1",
				Description: "Structured per-case scoring for evals.",
				Schema:      caseJudgeResponseSchema,
				Strict:      true,
			},
		},
		MaxCompletionTokens: cfg.MaxCompletionTokens,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return llmCaseEvaluation{}, fmt.Errorf("openai chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return llmCaseEvaluation{}, errors.New("judge returned no choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return llmCaseEvaluation{}, errors.New("judge returned empty content")
	}

	var out llmCaseEvaluation
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return llmCaseEvaluation{}, fmt.Errorf("parse judge JSON: %w", err)
	}
	if err := validateModelEvaluation(out); err != nil {
		return llmCaseEvaluation{}, err
	}
	return out, nil
}

func marshalCasePayload(c baselineCaseItem, maxToolEvents, maxAssistantChars int) ([]byte, error) {
	toolNames := make([]string, 0, len(c.ToolTrace))
	toolNameSeen := map[string]struct{}{}
	for _, ev := range c.ToolTrace {
		name, _ := ev["tool"].(string)
		if name == "" {
			continue
		}
		if _, seen := toolNameSeen[name]; seen {
			continue
		}
		toolNameSeen[name] = struct{}{}
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	caseInput := map[string]any{
		"id":                c.ID,
		"category":          c.Category,
		"message":           c.Message,
		"expected":          c.Expected,
		"success":           c.Success,
		"http_status":       c.HTTPStatus,
		"latency_ms":        c.LatencyMS,
		"error":             c.Error,
		"assistant_text":    truncate(c.AssistantText, maxAssistantChars),
		"tool_invocations":  c.ToolInvocations,
		"tool_results":      c.ToolResults,
		"tool_errors":       c.ToolErrors,
		"tool_names":        toolNames,
		"stream_errors":     sampleStreamErrors(c.StreamErrors, 4),
		"tool_trace_sample": sampleToolTrace(c.ToolTrace, maxToolEvents),
	}

	b, err := json.Marshal(caseInput)
	if err != nil {
		return nil, fmt.Errorf("marshal case payload: %w", err)
	}
	return b, nil
}

func sampleStreamErrors(in []map[string]any, max int) []map[string]any {
	if len(in) == 0 || max <= 0 {
		return nil
	}
	if len(in) > max {
		in = in[:max]
	}
	out := make([]map[string]any, 0, len(in))
	for _, row := range in {
		out = append(out, map[string]any{
			"message": truncate(anyToCompactJSON(row["message"]), 220),
			"tool":    truncate(anyToCompactJSON(row["tool"]), 120),
			"tool_id": truncate(anyToCompactJSON(row["tool_id"]), 120),
		})
	}
	return out
}

func sampleToolTrace(events []map[string]any, max int) []map[string]any {
	if len(events) == 0 || max <= 0 {
		return nil
	}
	if len(events) > max {
		events = events[:max]
	}
	out := make([]map[string]any, 0, len(events))
	for _, ev := range events {
		row := map[string]any{
			"event":   truncate(anyToCompactJSON(ev["event"]), 32),
			"tool":    truncate(anyToCompactJSON(ev["tool"]), 160),
			"tool_id": truncate(anyToCompactJSON(ev["tool_id"]), 120),
			"reason":  truncate(anyToCompactJSON(ev["reason"]), 320),
		}
		if v, ok := ev["arguments"]; ok {
			row["arguments_json"] = truncate(anyToCompactJSON(v), 1000)
		}
		if v, ok := ev["result"]; ok {
			row["result_json"] = truncate(anyToCompactJSON(v), 1600)
		}
		if v, ok := ev["is_error"]; ok {
			row["is_error"] = v
		}
		out = append(out, row)
	}
	return out
}

func validateModelEvaluation(in llmCaseEvaluation) error {
	scores := in.DimensionScores.AsMap()
	for k, v := range scores {
		if v < 0 || v > 4 {
			return fmt.Errorf("judge dimension score %q out of range: %d", k, v)
		}
	}
	rationales := []string{
		in.Rationales.Factuality,
		in.Rationales.AnswerCompleteness,
		in.Rationales.ToolUsageCorrectness,
		in.Rationales.HallucinationRisk,
		in.Rationales.Actionability,
	}
	for i, r := range rationales {
		if strings.TrimSpace(r) == "" {
			return fmt.Errorf("judge rationale[%d] is empty", i)
		}
	}
	if strings.TrimSpace(in.EvidenceSummary) == "" {
		return errors.New("judge evidence_summary is empty")
	}
	return nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	return nil
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

func anyToCompactJSON(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "...(truncated)"
}
