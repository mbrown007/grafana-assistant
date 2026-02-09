package mcp

import (
	"strings"
	"testing"
)

func TestFormatToolResult(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect string
	}{
		{"nil", nil, "Summary: No result returned"},
		{"string", "hello", "Summary: Tool returned text"},
		{"number", 42, "Summary: Tool returned value"},
		{"map", map[string]any{"key": "val"}, "Machine details"},
		{"slice", []any{"a", "b"}, "list with 2 item"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatToolResult(tt.input)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("FormatToolResult(%v) = %q, want to contain %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestFormatToolResult_CompositeStatusSummary(t *testing.T) {
	input := map[string]any{
		"status":  "ok",
		"action":  "fetch_metrics",
		"tool":    "grafana__query_prometheus",
		"route":   "mcp_tool",
		"message": "ok",
		"raw": []any{
			map[string]any{"metric": "up", "value": 1},
		},
	}

	got := FormatToolResult(input)
	for _, expected := range []string{
		"status=ok",
		"action=fetch_metrics",
		"tool=grafana__query_prometheus",
		"Machine details",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected %q in output, got: %s", expected, got)
		}
	}
}

func TestFormatToolResult_ParsesJSONText(t *testing.T) {
	input := `{"status":"ok","rows":[{"name":"api","errors":3}]}`
	got := FormatToolResult(input)

	if !strings.Contains(got, "status=ok") {
		t.Fatalf("expected parsed JSON summary, got: %s", got)
	}
	if !strings.Contains(got, `"errors": 3`) {
		t.Fatalf("expected JSON machine details, got: %s", got)
	}
}

func TestFormatToolResult_TrimDetailsLength(t *testing.T) {
	long := strings.Repeat("abcdef", 1200)
	got := FormatToolResult(long)

	if !strings.Contains(got, "Summary: Tool returned text") {
		t.Fatalf("expected summary prefix, got: %s", got)
	}
	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected truncation suffix in details, got: %s", got)
	}
}

func TestFormatToolResult_ObjectIncludesMultipleCollectionCounts(t *testing.T) {
	input := map[string]any{
		"status": "ok",
		"series": []any{map[string]any{"v": 1}},
		"alerts": []any{map[string]any{"name": "A"}, map[string]any{"name": "B"}},
	}

	got := FormatToolResult(input)
	for _, expected := range []string{"series=1", "alerts=2"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected %q in output, got: %s", expected, got)
		}
	}
}

func TestFormatToolResult_HeterogeneousArraySummary(t *testing.T) {
	input := []any{
		map[string]any{"a": 1},
		"text",
		float64(42),
	}

	got := FormatToolResult(input)
	if !strings.Contains(got, "item types:") {
		t.Fatalf("expected item types summary, got: %s", got)
	}
	for _, expected := range []string{"object=1", "string=1", "number=1"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected %q in output, got: %s", expected, got)
		}
	}
}

func TestFormatToolResult_RuneSafeTruncation(t *testing.T) {
	input := strings.Repeat("🙂", 2500)
	got := FormatToolResult(input)
	if !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected truncated marker for long unicode input, got: %s", got)
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("unexpected invalid utf-8 replacement character in output: %s", got)
	}
}

func TestFormatToolResult_DetailBudgetLimit(t *testing.T) {
	input := strings.Repeat("x", formatterDetailCharLimit+300)
	got := FormatToolResult(input)

	idx := strings.Index(got, "Machine details:\n")
	if idx < 0 {
		t.Fatalf("expected machine details section, got: %s", got)
	}
	details := got[idx+len("Machine details:\n"):]
	if gotRunes := len([]rune(details)); gotRunes > formatterDetailCharLimit {
		t.Fatalf("expected details <= %d runes, got %d", formatterDetailCharLimit, gotRunes)
	}
}
