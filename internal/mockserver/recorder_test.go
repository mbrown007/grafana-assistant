package mockserver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/mcp"
)

type recorderMockClient struct {
	result any
	err    error
}

func (m *recorderMockClient) Connect(context.Context) error { return nil }
func (m *recorderMockClient) Health(context.Context) error  { return nil }
func (m *recorderMockClient) DiscoverTools(context.Context) ([]mcp.Tool, error) {
	return nil, nil
}
func (m *recorderMockClient) InvokeTool(context.Context, string, map[string]any) (any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestRecordingClientWritesFixtureFile(t *testing.T) {
	dir := t.TempDir()
	inner := &recorderMockClient{
		result: map[string]any{
			"status": "success",
			"data":   map[string]any{"resultType": "matrix"},
		},
	}
	client := NewRecordingClient(inner, dir)

	args := map[string]any{
		"datasourceUid": "prometheus",
		"expr":          `rate(node_cpu_seconds_total{mode!="idle"}[5m])`,
		"limit":         100,
	}
	ctx := ContextWithEvalCaseID(context.Background(), "case-01")
	if _, err := client.InvokeTool(ctx, "grafana__query_prometheus", args); err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}

	expectedHash, err := fixtureArgHash(args)
	if err != nil {
		t.Fatalf("fixtureArgHash: %v", err)
	}
	outPath := filepath.Join(dir, "grafana__query_prometheus", "case-01_"+expectedHash+".json")
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", outPath, err)
	}

	var fixture Fixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if got, want := fixture.ToolName, "grafana__query_prometheus"; got != want {
		t.Fatalf("fixture tool_name = %q, want %q", got, want)
	}
	if got, want := fixture.Match["datasourceUid"], "^prometheus$"; got != want {
		t.Fatalf("datasource regex = %v, want %v", got, want)
	}
	if got, want := fixture.Match["limit"], "^100$"; got != want {
		t.Fatalf("limit regex = %v, want %v", got, want)
	}
	resp, ok := fixture.Response.(map[string]any)
	if !ok {
		t.Fatalf("fixture response type = %T, want map[string]any", fixture.Response)
	}
	if got, want := resp["status"], "success"; got != want {
		t.Fatalf("fixture response status = %v, want %v", got, want)
	}
}

func TestRecordingClientDoesNotWriteOnError(t *testing.T) {
	dir := t.TempDir()
	client := NewRecordingClient(&recorderMockClient{
		err: errors.New("boom"),
	}, dir)
	_, err := client.InvokeTool(context.Background(), "grafana__query_prometheus", map[string]any{"expr": "up"})
	if err == nil {
		t.Fatal("expected invoke error")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no fixture files, found %d entries", len(entries))
	}
}

func TestContextWithEvalCaseIDSanitizesValue(t *testing.T) {
	ctx := ContextWithEvalCaseID(context.Background(), " p8/case 1 ")
	got := EvalCaseIDFromContext(ctx)
	if got != "p8_case_1" {
		t.Fatalf("EvalCaseIDFromContext() = %q, want %q", got, "p8_case_1")
	}
}

func TestRecordingClientFallsBackToCounterCaseID(t *testing.T) {
	dir := t.TempDir()
	client := NewRecordingClient(&recorderMockClient{
		result: map[string]any{"status": "success"},
	}, dir)

	if _, err := client.InvokeTool(context.Background(), "grafana__search_dashboards", map[string]any{"query": "cpu"}); err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	toolDir := filepath.Join(dir, "grafana__search_dashboards")
	entries, err := os.ReadDir(toolDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", toolDir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one fixture file, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "case_0001_") {
		t.Fatalf("unexpected fixture filename: %s", entries[0].Name())
	}
}
