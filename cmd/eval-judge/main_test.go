package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/evals"
)

func TestSelectCases(t *testing.T) {
	in := []baselineCaseItem{
		{ID: "a", Category: "one"},
		{ID: "b", Category: "two"},
		{ID: "c", Category: "one"},
	}

	got := selectCases(in, "one", 1)
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("unexpected selection result: %+v", got)
	}

	gotAll := selectCases(in, "", 0)
	if len(gotAll) != 3 {
		t.Fatalf("expected all cases, got %d", len(gotAll))
	}
}

func TestMarshalCasePayload(t *testing.T) {
	c := baselineCaseItem{
		ID:            "DSH-001",
		Category:      "dashboard_summary",
		Message:       "summarize",
		AssistantText: strings.Repeat("x", 80),
		ToolTrace: []map[string]any{
			{
				"event":     "call",
				"tool":      "grafana__query_prometheus",
				"tool_id":   "t1",
				"arguments": map[string]any{"query": "up"},
			},
			{
				"event":   "result",
				"tool":    "grafana__query_prometheus",
				"tool_id": "t1",
				"result":  map[string]any{"status": "ok"},
			},
		},
	}

	b, err := marshalCasePayload(c, 1, 24)
	if err != nil {
		t.Fatalf("marshalCasePayload() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	assistantText, _ := parsed["assistant_text"].(string)
	if !strings.Contains(assistantText, "(truncated)") {
		t.Fatalf("expected assistant_text truncation marker, got %q", assistantText)
	}

	trace, _ := parsed["tool_trace_sample"].([]any)
	if len(trace) != 1 {
		t.Fatalf("expected tool_trace_sample len=1, got %d", len(trace))
	}
}

func TestValidateModelEvaluation(t *testing.T) {
	ok := llmCaseEvaluation{
		DimensionScores: evalDims(2),
		Rationales:      evalsRationales("ok"),
		EvidenceSummary: "evidence",
	}
	if err := validateModelEvaluation(ok); err != nil {
		t.Fatalf("expected valid model output, got %v", err)
	}

	badScore := ok
	badScore.DimensionScores.Factuality = 9
	if err := validateModelEvaluation(badScore); err == nil {
		t.Fatal("expected score range validation error")
	}

	badRationale := ok
	badRationale.Rationales.Actionability = ""
	if err := validateModelEvaluation(badRationale); err == nil {
		t.Fatal("expected empty rationale validation error")
	}
}

func evalDims(v int) evals.DimensionScores {
	return evals.DimensionScores{
		Factuality:           v,
		AnswerCompleteness:   v,
		ToolUsageCorrectness: v,
		HallucinationRisk:    v,
		Actionability:        v,
	}
}

func evalsRationales(msg string) evals.Rationales {
	return evals.Rationales{
		Factuality:           msg,
		AnswerCompleteness:   msg,
		ToolUsageCorrectness: msg,
		HallucinationRisk:    msg,
		Actionability:        msg,
	}
}
