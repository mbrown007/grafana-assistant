package agent

import (
	"strings"
	"testing"
)

func TestShapeToolResultOutputs_RedactsClientPayloadAndKeepsSummaryForModel(t *testing.T) {
	raw := map[string]any{
		"status":        "ok",
		"tool":          "grafana__query_prometheus",
		"api_token":     "plain-token-value",
		"authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature",
		"raw": map[string]any{
			"series": []any{
				map[string]any{"metric": "up", "value": 1},
			},
		},
	}

	streamPayload, modelContent := shapeToolResultOutputs(raw)
	streamMap, ok := streamPayload.(map[string]any)
	if !ok {
		t.Fatalf("expected map stream payload, got %T", streamPayload)
	}
	if streamMap["api_token"] != redactionMask {
		t.Fatalf("expected api_token to be redacted, got %#v", streamMap["api_token"])
	}
	if streamMap["authorization"] != redactionMask {
		t.Fatalf("expected authorization to be redacted, got %#v", streamMap["authorization"])
	}
	if raw["api_token"] != "plain-token-value" {
		t.Fatalf("expected original input unchanged, got %#v", raw["api_token"])
	}

	for _, expected := range []string{
		"[TOOL_RESULT_START]",
		"Summary:",
		"Machine details:",
		`"status": "ok"`,
		"[TOOL_RESULT_END]",
	} {
		if !strings.Contains(modelContent, expected) {
			t.Fatalf("expected model content to contain %q, got: %s", expected, modelContent)
		}
	}
}

func TestShapeToolResultOutputs_StringErrorStillSummarized(t *testing.T) {
	raw := "Error: query parse failed"
	streamPayload, modelContent := shapeToolResultOutputs(raw)

	if streamPayload != raw {
		t.Fatalf("expected raw string stream payload, got %#v", streamPayload)
	}
	if !strings.Contains(modelContent, "Summary: Tool returned text") {
		t.Fatalf("expected summary-first model content, got: %s", modelContent)
	}
}

func TestShapeToolResultOutputs_NilResult(t *testing.T) {
	streamPayload, modelContent := shapeToolResultOutputs(nil)
	if streamPayload != nil {
		t.Fatalf("expected nil stream payload, got %#v", streamPayload)
	}
	for _, expected := range []string{
		"[TOOL_RESULT_START]",
		"Summary: No result returned.",
		"[TOOL_RESULT_END]",
	} {
		if !strings.Contains(modelContent, expected) {
			t.Fatalf("expected %q in model content, got: %s", expected, modelContent)
		}
	}
}

func TestShapeToolResultOutputsWithMode_RedactionDisabled(t *testing.T) {
	raw := map[string]any{
		"api_token": "plain-token-value",
		"status":    "ok",
	}

	streamPayload, _ := shapeToolResultOutputsWithMode(raw, false)
	streamMap, ok := streamPayload.(map[string]any)
	if !ok {
		t.Fatalf("expected map stream payload, got %T", streamPayload)
	}
	if streamMap["api_token"] != "plain-token-value" {
		t.Fatalf("expected api_token to remain unredacted, got %#v", streamMap["api_token"])
	}
}
