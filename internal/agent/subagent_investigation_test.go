package agent

import (
	"reflect"
	"strings"
	"testing"

	appcontext "github.com/brownster/grafana-assistant/internal/context"
	"github.com/brownster/grafana-assistant/internal/mcp"
)

func TestNewInvestigationSubAgent(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__query_prometheus"},
		{Name: "grafana__query_loki_logs"},
		{Name: "alertmanager__list_alerts"},
	}
	mgr := &Manager{tools: tools}

	dashCtx := &appcontext.DashboardSummary{
		UID:    "dash-123",
		Title:  "API Overview",
		Folder: "Operations",
		Panels: []appcontext.PanelSummary{
			{Title: "P95 latency", Type: "timeseries"},
		},
	}
	schemaCtx := "Metrics: http_request_duration_seconds_bucket, up"

	sa := mgr.newInvestigationSubAgent(dashCtx, schemaCtx)
	if sa == nil {
		t.Fatal("newInvestigationSubAgent() returned nil")
	}
	if sa.Name != "investigation" {
		t.Fatalf("Name = %q, want %q", sa.Name, "investigation")
	}
	if sa.MaxIterations != 5 {
		t.Fatalf("MaxIterations = %d, want %d", sa.MaxIterations, 5)
	}
	if !reflect.DeepEqual(toolNames(sa.Tools), toolNames(tools)) {
		t.Fatalf("Tools = %v, want %v", toolNames(sa.Tools), toolNames(tools))
	}
	if !strings.Contains(sa.SystemPrompt, "incident investigation specialist") {
		t.Fatalf("SystemPrompt missing specialist role: %q", sa.SystemPrompt)
	}
	if !strings.Contains(sa.SystemPrompt, "Dashboard context:") {
		t.Fatalf("SystemPrompt missing dashboard context block: %q", sa.SystemPrompt)
	}
	if !strings.Contains(sa.SystemPrompt, "Schema context (metrics/labels/log fields):") {
		t.Fatalf("SystemPrompt missing schema context block: %q", sa.SystemPrompt)
	}
	if strings.Contains(sa.SystemPrompt, "scratchpad__upsert_panel") {
		t.Fatalf("SystemPrompt should not include scratchpad docs: %q", sa.SystemPrompt)
	}
	if strings.Contains(sa.SystemPrompt, "explore__open") {
		t.Fatalf("SystemPrompt should not include explore docs: %q", sa.SystemPrompt)
	}
}

func TestBuildInvestigationPrompt_WithoutContext(t *testing.T) {
	prompt := buildInvestigationPrompt(nil, "")
	if !strings.Contains(prompt, "incident investigation specialist") {
		t.Fatalf("prompt missing specialist role: %q", prompt)
	}
	if strings.Contains(prompt, "Dashboard context:") {
		t.Fatalf("prompt should not include dashboard block when none provided: %q", prompt)
	}
	if strings.Contains(prompt, "Schema context (metrics/labels/log fields):") {
		t.Fatalf("prompt should not include schema block when none provided: %q", prompt)
	}
}
