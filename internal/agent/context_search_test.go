package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

func TestBuildSelectedContextBlock_Empty(t *testing.T) {
	if got := buildSelectedContextBlock(nil); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	if got := buildSelectedContextBlock([]api.ContextEntity{}); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestBuildSelectedContextBlock_Datasource(t *testing.T) {
	entities := []api.ContextEntity{
		{Type: api.ContextEntityDatasource, ID: "prom-1", DisplayName: "Prometheus", Metadata: map[string]string{"ds_type": "prometheus"}},
	}
	got := buildSelectedContextBlock(entities)
	if !strings.Contains(got, "Datasource: Prometheus") {
		t.Fatalf("expected datasource line, got:\n%s", got)
	}
	if !strings.Contains(got, "type=prometheus") {
		t.Fatalf("expected ds_type metadata, got:\n%s", got)
	}
	if !strings.Contains(got, `datasourceUid="prom-1"`) {
		t.Fatalf("expected datasourceUid, got:\n%s", got)
	}
	if !strings.Contains(got, "use with query tools") {
		t.Fatalf("expected usage hint, got:\n%s", got)
	}
}

func TestBuildSelectedContextBlock_Dashboard(t *testing.T) {
	entities := []api.ContextEntity{
		{Type: api.ContextEntityDashboard, ID: "abc-123", DisplayName: "Node Exporter", Metadata: map[string]string{"folder": "Infra"}},
	}
	got := buildSelectedContextBlock(entities)
	if !strings.Contains(got, "Dashboard: Node Exporter") {
		t.Fatalf("expected dashboard line, got:\n%s", got)
	}
	if !strings.Contains(got, `dashboardUid="abc-123"`) {
		t.Fatalf("expected dashboardUid, got:\n%s", got)
	}
	if !strings.Contains(got, "folder=Infra") {
		t.Fatalf("expected folder metadata, got:\n%s", got)
	}
	if !strings.Contains(got, "use with dashboard tools") {
		t.Fatalf("expected usage hint, got:\n%s", got)
	}
}

func TestBuildSelectedContextBlock_DashboardNoFolder(t *testing.T) {
	entities := []api.ContextEntity{
		{Type: api.ContextEntityDashboard, ID: "abc-123", DisplayName: "Node Exporter"},
	}
	got := buildSelectedContextBlock(entities)
	if strings.Contains(got, "folder=") {
		t.Fatalf("expected no folder field, got:\n%s", got)
	}
}

func TestBuildSelectedContextBlock_Metric(t *testing.T) {
	entities := []api.ContextEntity{
		{Type: api.ContextEntityMetric, ID: "node_cpu_seconds_total", DisplayName: "node_cpu_seconds_total"},
	}
	got := buildSelectedContextBlock(entities)
	if !strings.Contains(got, "Prometheus metric: node_cpu_seconds_total") {
		t.Fatalf("expected metric line, got:\n%s", got)
	}
}

func TestBuildSelectedContextBlock_Label(t *testing.T) {
	entities := []api.ContextEntity{
		{Type: api.ContextEntityLabel, ID: "instance", DisplayName: "instance"},
	}
	got := buildSelectedContextBlock(entities)
	if !strings.Contains(got, "Prometheus label: instance") {
		t.Fatalf("expected label line, got:\n%s", got)
	}
}

func TestBuildSelectedContextBlock_AllTypes(t *testing.T) {
	entities := []api.ContextEntity{
		{Type: api.ContextEntityDatasource, ID: "prom-1", DisplayName: "Prometheus", Metadata: map[string]string{"ds_type": "prometheus"}},
		{Type: api.ContextEntityDashboard, ID: "dash-1", DisplayName: "Overview"},
		{Type: api.ContextEntityMetric, ID: "up", DisplayName: "up"},
		{Type: api.ContextEntityLabel, ID: "job", DisplayName: "job"},
	}
	got := buildSelectedContextBlock(entities)

	if !strings.HasPrefix(got, "The user explicitly selected the following entities") {
		t.Fatalf("expected preamble, got:\n%s", got)
	}
	if !strings.Contains(got, "Datasource UIDs are NOT dashboard UIDs") {
		t.Fatalf("expected disambiguation warning, got:\n%s", got)
	}
	if !strings.Contains(got, "Datasource: Prometheus") {
		t.Fatalf("missing datasource, got:\n%s", got)
	}
	if !strings.Contains(got, "Dashboard: Overview") {
		t.Fatalf("missing dashboard, got:\n%s", got)
	}
	if !strings.Contains(got, "Prometheus metric: up") {
		t.Fatalf("missing metric, got:\n%s", got)
	}
	if !strings.Contains(got, "Prometheus label: job") {
		t.Fatalf("missing label, got:\n%s", got)
	}
}

func TestParseDashboardSearchResults_Array(t *testing.T) {
	input := `[
		{"uid":"dash-1","title":"Overview","folderTitle":"General"},
		{"uid":"dash-2","title":"Node Exporter"}
	]`
	results := parseDashboardSearchResults(input)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "dash-1" || results[0].DisplayName != "Overview" {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
	if results[0].Metadata["folder"] != "General" {
		t.Fatalf("expected folder=General, got %q", results[0].Metadata["folder"])
	}
	if results[1].ID != "dash-2" || results[1].DisplayName != "Node Exporter" {
		t.Fatalf("unexpected second result: %+v", results[1])
	}
	if _, hasFolder := results[1].Metadata["folder"]; hasFolder {
		t.Fatalf("expected no folder for second result")
	}
}

func TestParseDashboardSearchResults_WrappedObject(t *testing.T) {
	input := `{"dashboards":[{"uid":"d1","title":"Dash One"}]}`
	results := parseDashboardSearchResults(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "d1" {
		t.Fatalf("expected uid=d1, got %q", results[0].ID)
	}
}

func TestParseDashboardSearchResults_MissingUID(t *testing.T) {
	input := `[{"title":"No UID Dashboard"}]`
	results := parseDashboardSearchResults(input)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for missing UID, got %d", len(results))
	}
}

func TestParseDashboardSearchResults_Empty(t *testing.T) {
	if results := parseDashboardSearchResults(nil); results != nil {
		t.Fatalf("expected nil for nil input, got %+v", results)
	}
	if results := parseDashboardSearchResults("[]"); len(results) != 0 {
		t.Fatalf("expected 0 for empty array, got %d", len(results))
	}
}

func TestParseDashboardSearchResults_EntityType(t *testing.T) {
	input := `[{"uid":"d1","title":"Test"}]`
	results := parseDashboardSearchResults(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type != api.ContextEntityDashboard {
		t.Fatalf("expected type=dashboard, got %q", results[0].Type)
	}
}

func TestParseDashboardSearchResults_JSONMap(t *testing.T) {
	// Test the json.RawMessage / map[string]any wrapper case via coerceJSONValue.
	raw := map[string]any{
		"items": []any{
			map[string]any{"uid": "x1", "title": "X One"},
		},
	}
	b, _ := json.Marshal(raw)
	results := parseDashboardSearchResults(string(b))
	if len(results) != 1 {
		t.Fatalf("expected 1 result from items wrapper, got %d", len(results))
	}
	if results[0].ID != "x1" {
		t.Fatalf("expected uid=x1, got %q", results[0].ID)
	}
}
