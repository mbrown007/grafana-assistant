package agent

import (
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

func TestSystemPrompt_NoDashboardNoTools(t *testing.T) {
	prompt := SystemPrompt(nil, nil, nil, true, ResolvePromptProfile(PromptProfileBalanced))
	if !strings.Contains(prompt, "monitoring assistant") {
		t.Error("expected role definition in system prompt")
	}
	if strings.Contains(prompt, "## Current Dashboard Context") {
		t.Error("should not contain dashboard context when nil")
	}
	if strings.Contains(prompt, "## Enabled Tool Summary") {
		t.Error("should not contain tool summary section when tools are empty")
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

	prompt := SystemPrompt(dashCtx, nil, nil, true, ResolvePromptProfile(PromptProfileBalanced))
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

	prompt := SystemPrompt(nil, reqCtx, nil, true, ResolvePromptProfile(PromptProfileBalanced))
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

	prompt := SystemPrompt(nil, nil, tools, true, ResolvePromptProfile(PromptProfileBalanced))
	if !strings.Contains(prompt, "## Enabled Tool Summary") {
		t.Error("expected compact tool summary section")
	}
	if !strings.Contains(prompt, "alertmanager__list_alerts") {
		t.Error("expected tool name")
	}
	if strings.Contains(prompt, "## Available Tools") {
		t.Error("should not include full tool catalog heading")
	}
}

func TestSystemPrompt_ToolSummaryDoesNotDumpFullCatalog(t *testing.T) {
	tools := make([]mcp.Tool, 0, 20)
	for i := 0; i < 20; i++ {
		name := "grafana__list_metric_" + string(rune('a'+i))
		tools = append(tools, mcp.Tool{Name: name, Description: "desc"})
	}

	prompt := SystemPrompt(nil, nil, tools, true, ResolvePromptProfile(PromptProfileBalanced))
	if !strings.Contains(prompt, "High-value tools (sample, not exhaustive)") {
		t.Fatal("expected compact sampled tool list")
	}

	// Ensure the prompt does not enumerate every tool from a large list.
	if strings.Contains(prompt, "grafana__list_metric_t") {
		t.Fatal("expected lower-priority tail tools to be omitted from compact summary")
	}
}

func TestSystemPrompt_ArtifactExamples(t *testing.T) {
	prompt := SystemPrompt(nil, nil, nil, true, ResolvePromptProfile(PromptProfileBalanced))
	for _, keyword := range []string{"artifact", "chart", "metric-cards", "table", "report"} {
		if !strings.Contains(prompt, keyword) {
			t.Errorf("expected artifact example containing %q", keyword)
		}
	}
}

func TestSystemPrompt_CompositeInvestigationGuidance(t *testing.T) {
	prompt := SystemPrompt(nil, nil, nil, true, ResolvePromptProfile(PromptProfileBalanced))
	for _, keyword := range []string{
		"investigation__manage",
		"plan, fetch_metrics, fetch_logs, summarize, next_step",
		"fetch_metrics and fetch_logs call underlying MCP query tools",
		"plan, summarize, and next_step provide structured planning/decision scaffolding",
	} {
		if !strings.Contains(prompt, keyword) {
			t.Errorf("expected prompt guidance containing %q", keyword)
		}
	}
}

func TestSystemPrompt_CompositeInvestigationGuidanceDisabled(t *testing.T) {
	prompt := SystemPrompt(nil, nil, nil, false, ResolvePromptProfile(PromptProfileBalanced))
	if strings.Contains(prompt, "investigation__manage") {
		t.Fatal("expected prompt to omit composite investigation guidance when feature is disabled")
	}
}

func TestSystemPrompt_CompactProfileGuidance(t *testing.T) {
	prompt := SystemPrompt(nil, nil, nil, true, ResolvePromptProfile(PromptProfileCompact))
	if !strings.Contains(prompt, "Active profile: compact.") {
		t.Fatal("expected compact profile section in prompt")
	}
	if !strings.Contains(prompt, "Keep responses short and execution-oriented.") {
		t.Fatal("expected compact response guidance in prompt")
	}
}
