package mockserver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFixturesAndMatch(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "grafana__query_prometheus")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir fixtures: %v", err)
	}

	fixturePath := filepath.Join(toolDir, "case_01.json")
	fixtureJSON := `{
  "tool_name": "grafana__query_prometheus",
  "match": {
    "datasourceUid": "^prometheus$",
    "expr": "node_cpu"
  },
  "response": {
    "status": "success",
    "data": {
      "resultType": "matrix",
      "result": []
    }
  }
}`
	if err := os.WriteFile(fixturePath, []byte(fixtureJSON), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	fixtures, err := LoadFixtures(dir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	if len(fixtures) != 1 {
		t.Fatalf("expected 1 fixture, got %d", len(fixtures))
	}

	matcher, err := NewMatcher(fixtures)
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	response, ok := matcher.Match("query_prometheus", map[string]any{
		"datasourceUid": "prometheus",
		"expr":          `rate(node_cpu_seconds_total{mode!="idle"}[5m])`,
	})
	if !ok {
		t.Fatal("expected fixture match")
	}
	m, ok := response.(map[string]any)
	if !ok {
		t.Fatalf("expected map response, got %T", response)
	}
	if got, _ := m["status"].(string); got != "success" {
		t.Fatalf("unexpected status: %q", got)
	}

	_, ok = matcher.Match("query_prometheus", map[string]any{
		"datasourceUid": "prometheus",
		"expr":          `sum(rate(http_requests_total[5m]))`,
	})
	if ok {
		t.Fatal("expected no fixture match for expr without node_cpu")
	}
}

func TestLoadFixturesDerivesToolNameFromPath(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "alertmanager__list_alerts")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir fixtures: %v", err)
	}
	fixturePath := filepath.Join(toolDir, "case_01.json")
	fixtureJSON := `{
  "match": { "severity": "^critical$" },
  "response": { "status": "success", "alerts": [] }
}`
	if err := os.WriteFile(fixturePath, []byte(fixtureJSON), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	fixtures, err := LoadFixtures(dir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	if len(fixtures) != 1 {
		t.Fatalf("expected 1 fixture, got %d", len(fixtures))
	}
	if got, want := fixtures[0].ToolName, "alertmanager__list_alerts"; got != want {
		t.Fatalf("unexpected derived tool name: got %q want %q", got, want)
	}
}

func TestNewMatcherRejectsInvalidRegex(t *testing.T) {
	_, err := NewMatcher([]Fixture{
		{
			ToolName: "grafana__search_dashboards",
			Match: map[string]any{
				"query": "(unclosed",
			},
			Response:   map[string]any{"status": "success"},
			SourcePath: "bad_fixture.json",
		},
	})
	if err == nil {
		t.Fatal("expected regex compile error")
	}
}

func TestToolNamesAreNormalizedAndUnique(t *testing.T) {
	matcher, err := NewMatcher([]Fixture{
		{ToolName: "grafana__query_prometheus", Response: map[string]any{}},
		{ToolName: "query_prometheus", Response: map[string]any{}},
		{ToolName: "kb__search_docs", Response: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}
	names := matcher.ToolNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 normalized names, got %d (%v)", len(names), names)
	}
	if names[0] != "query_prometheus" || names[1] != "search_docs" {
		t.Fatalf("unexpected tool names: %v", names)
	}
}

func TestFilterFixturesByServerType(t *testing.T) {
	fixtures := []Fixture{
		{ToolName: "grafana__query_prometheus"},
		{ToolName: "alertmanager__list_alerts"},
		{ToolName: "search_kb"},
	}

	filtered := FilterFixturesByServerType(fixtures, "grafana")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 fixtures after filter, got %d", len(filtered))
	}
	if filtered[0].ToolName != "grafana__query_prometheus" {
		t.Fatalf("unexpected first fixture: %q", filtered[0].ToolName)
	}
	if filtered[1].ToolName != "search_kb" {
		t.Fatalf("unexpected second fixture: %q", filtered[1].ToolName)
	}
}
