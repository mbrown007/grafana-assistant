package chatops

import (
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

func TestFormatResponse_TokensOnly(t *testing.T) {
	chunks := []api.StreamChunk{
		{Type: "start", SessionID: "s1"},
		{Type: "token", Message: "Hello "},
		{Type: "token", Message: "world!"},
		{Type: "complete", Message: "Hello world!"},
	}
	got := FormatResponse(chunks)
	if got != "Hello world!" {
		t.Errorf("got %q, want %q", got, "Hello world!")
	}
}

func TestFormatResponse_WithToolCalls(t *testing.T) {
	chunks := []api.StreamChunk{
		{Type: "token", Message: "CPU is at 80%."},
		{Type: "tool", Tool: "query_prometheus", Reason: "Check CPU usage", Arguments: map[string]interface{}{"query": "cpu"}},
	}
	got := FormatResponse(chunks)
	if !strings.Contains(got, "CPU is at 80%.") {
		t.Error("expected response body in output")
	}
	if !strings.Contains(got, "**Tools used:**") {
		t.Error("expected tools section")
	}
	if !strings.Contains(got, "`query_prometheus`") {
		t.Error("expected tool name")
	}
	if !strings.Contains(got, "*Check CPU usage*") {
		t.Error("expected tool reason")
	}
}

func TestFormatResponse_ToolWithoutReason(t *testing.T) {
	chunks := []api.StreamChunk{
		{Type: "token", Message: "Done."},
		{Type: "tool", Tool: "list_alerts", Arguments: map[string]interface{}{}},
	}
	got := FormatResponse(chunks)
	if !strings.Contains(got, "- `list_alerts`\n") {
		t.Errorf("expected tool line without reason suffix, got %q", got)
	}
}

func TestFormatResponse_ErrorChunk(t *testing.T) {
	chunks := []api.StreamChunk{
		{Type: "token", Message: "Partial response."},
		{Type: "error", Message: "rate limit exceeded"},
	}
	got := FormatResponse(chunks)
	if !strings.Contains(got, ":warning:") {
		t.Error("expected warning emoji in error output")
	}
	if !strings.Contains(got, "rate limit exceeded") {
		t.Error("expected error message")
	}
}

func TestFormatResponse_Empty(t *testing.T) {
	got := FormatResponse(nil)
	if got != "_No response generated._" {
		t.Errorf("got %q, want empty fallback", got)
	}
}

func TestFormatResponse_CompleteOnly(t *testing.T) {
	chunks := []api.StreamChunk{
		{Type: "complete", Message: "Final answer."},
	}
	got := FormatResponse(chunks)
	if got != "Final answer." {
		t.Errorf("got %q, want %q", got, "Final answer.")
	}
}

func TestFormatResponse_ToolInvocationWithoutArgs_Ignored(t *testing.T) {
	// Tool result chunks (no Arguments) should not appear in tool summary.
	chunks := []api.StreamChunk{
		{Type: "token", Message: "Answer."},
		{Type: "tool", Tool: "query_prometheus", Result: "some data"},
	}
	got := FormatResponse(chunks)
	if strings.Contains(got, "**Tools used:**") {
		t.Error("tool result (no args) should not generate tools section")
	}
}
