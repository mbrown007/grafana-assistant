package agent

import (
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

func TestSystemPrompt_NoDashboardNoTools(t *testing.T) {
	prompt := SystemPrompt(nil, nil, nil)
	if !strings.Contains(prompt, "monitoring assistant") {
		t.Error("expected role definition in system prompt")
	}
	if strings.Contains(prompt, "## Current Dashboard Context") {
		t.Error("should not contain dashboard context when nil")
	}
	if strings.Contains(prompt, "## Available Tools") {
		t.Error("should not contain tools section when empty")
	}
}

func TestSystemPrompt_WithDashboard(t *testing.T) {
	dashCtx := &appcontext.DashboardSummary{
		UID:    "abc123",
		Title:  "Node Exporter",
		Folder: "Infrastructure",
		Tags:   []string{"linux", "prometheus"},
		Panels: []appcontext.PanelSummary{
			{
				Title:   "CPU Usage",
				Type:    "timeseries",
				Queries: []string{`rate(node_cpu_seconds_total[5m])`},
			},
		},
	}

	prompt := SystemPrompt(dashCtx, nil, nil)
	if !strings.Contains(prompt, "Node Exporter") {
		t.Error("expected dashboard title")
	}
	if !strings.Contains(prompt, "Infrastructure") {
		t.Error("expected folder")
	}
	if !strings.Contains(prompt, "linux, prometheus") {
		t.Error("expected tags")
	}
	if !strings.Contains(prompt, "CPU Usage") {
		t.Error("expected panel title")
	}
	if !strings.Contains(prompt, "node_cpu_seconds_total") {
		t.Error("expected query")
	}
}

func TestSystemPrompt_WithTimeRange(t *testing.T) {
	reqCtx := &api.DashboardContext{
		UID:       "abc123",
		TimeRange: map[string]string{"from": "now-1h", "to": "now"},
		Variables: map[string]string{"instance": "prod-01"},
	}

	prompt := SystemPrompt(nil, reqCtx, nil)
	if !strings.Contains(prompt, "now-1h") {
		t.Error("expected time range from")
	}
	if !strings.Contains(prompt, "instance=prod-01") {
		t.Error("expected variables")
	}
}

func TestSystemPrompt_WithTools(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "alertmanager__list_alerts", Description: "List active alerts"},
	}

	prompt := SystemPrompt(nil, nil, tools)
	if !strings.Contains(prompt, "## Available Tools") {
		t.Error("expected tools section")
	}
	if !strings.Contains(prompt, "alertmanager__list_alerts") {
		t.Error("expected tool name")
	}
}

func TestSystemPrompt_ArtifactExamples(t *testing.T) {
	prompt := SystemPrompt(nil, nil, nil)
	for _, keyword := range []string{"artifact", "chart", "metric-cards", "table", "report"} {
		if !strings.Contains(prompt, keyword) {
			t.Errorf("expected artifact example containing %q", keyword)
		}
	}
}
