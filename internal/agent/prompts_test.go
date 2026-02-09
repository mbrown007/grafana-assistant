package agent

import (
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/api"
	appcontext "github.com/brownster/grafana-assistant/internal/context"
	"github.com/brownster/grafana-assistant/internal/mcp"
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

// --- BuildSystemPrompt intent-specific tests ---

func sampleTools() []mcp.Tool {
	return []mcp.Tool{
		{Name: "grafana__query_prometheus", Description: "Query Prometheus"},
		{Name: "grafana__search_dashboards", Description: "Search dashboards"},
		{Name: "alertmanager__list_alerts", Description: "List active alerts"},
	}
}

func TestBuildSystemPrompt_HowToDocsOmitsArtifacts(t *testing.T) {
	prompt := BuildSystemPrompt(PromptContext{
		Intent:            IntentHowToDocs,
		Tools:             sampleTools(),
		CompositeToolMode: true,
		Profile:           ResolvePromptProfile(PromptProfileBalanced),
	})

	// how_to_docs should always include core role and security.
	if !strings.Contains(prompt, "monitoring assistant") {
		t.Error("expected role definition")
	}
	if !strings.Contains(prompt, "## Security Rules") {
		t.Error("expected security rules")
	}

	// how_to_docs should NOT include these sections.
	for _, absent := range []string{
		"## Artifact System",
		"## Scratchpad Tool",
		"## Explore Tool",
		"## Enabled Tool Summary",
		"## Tool Policy",
		"## Composite Investigation Tool",
	} {
		if strings.Contains(prompt, absent) {
			t.Errorf("how_to_docs prompt should NOT contain %q", absent)
		}
	}

	// Should contain docs-specific guidelines.
	if !strings.Contains(prompt, "documentation") {
		t.Error("expected docs-specific guidelines")
	}
}

func TestBuildSystemPrompt_LiveDataIncludesAll(t *testing.T) {
	prompt := BuildSystemPrompt(PromptContext{
		Intent:            IntentLiveData,
		Tools:             sampleTools(),
		CompositeToolMode: true,
		Profile:           ResolvePromptProfile(PromptProfileBalanced),
	})

	for _, present := range []string{
		"## Artifact System",
		"## Scratchpad Tool",
		"## Explore Tool",
		"## Enabled Tool Summary",
		"## Tool Policy",
		"## Security Rules",
		"## Composite Investigation Tool",
		"## Guidelines",
	} {
		if !strings.Contains(prompt, present) {
			t.Errorf("live_data prompt should contain %q", present)
		}
	}
}

func TestBuildSystemPrompt_IntentGuidelines(t *testing.T) {
	cases := []struct {
		intent  IntentClass
		keyword string
	}{
		{IntentQueryHelp, "PromQL/LogQL syntax"},
		{IntentDashboardLookup, "dashboard UIDs"},
		{IntentHowToDocs, "documentation"},
		{IntentIncidentSummary, "Investigate systematically"},
		{IntentLiveData, "call the tools FIRST"},
	}

	for _, tc := range cases {
		t.Run(string(tc.intent), func(t *testing.T) {
			prompt := BuildSystemPrompt(PromptContext{
				Intent:  tc.intent,
				Tools:   sampleTools(),
				Profile: ResolvePromptProfile(PromptProfileBalanced),
			})
			if !strings.Contains(prompt, tc.keyword) {
				t.Errorf("expected %q intent guidelines to contain %q", tc.intent, tc.keyword)
			}
		})
	}
}

func TestBuildSystemPrompt_HowToDocsSmallerThanLiveData(t *testing.T) {
	tools := sampleTools()
	profile := ResolvePromptProfile(PromptProfileBalanced)

	live := BuildSystemPrompt(PromptContext{
		Intent:            IntentLiveData,
		Tools:             tools,
		CompositeToolMode: true,
		Profile:           profile,
	})
	docs := BuildSystemPrompt(PromptContext{
		Intent:  IntentHowToDocs,
		Profile: profile,
	})

	reduction := 1.0 - float64(len(docs))/float64(len(live))
	if reduction < 0.30 {
		t.Fatalf("how_to_docs prompt should be >=30%% smaller than live_data; got %.1f%% reduction (live=%d, docs=%d)",
			reduction*100, len(live), len(docs))
	}
}

func TestBuildSystemPrompt_UnknownIntentGetsFull(t *testing.T) {
	prompt := BuildSystemPrompt(PromptContext{
		Intent:            IntentClass("unknown_intent"),
		Tools:             sampleTools(),
		CompositeToolMode: true,
		Profile:           ResolvePromptProfile(PromptProfileBalanced),
	})

	// Unknown intent should get all sections as safe fallback.
	for _, present := range []string{
		"## Artifact System",
		"## Scratchpad Tool",
		"## Explore Tool",
		"## Enabled Tool Summary",
		"## Tool Policy",
		"## Composite Investigation Tool",
	} {
		if !strings.Contains(prompt, present) {
			t.Errorf("unknown intent prompt should contain %q as safe fallback", present)
		}
	}
}

func TestBuildSystemPrompt_IncidentSummaryIncludesCompositeTool(t *testing.T) {
	prompt := BuildSystemPrompt(PromptContext{
		Intent:            IntentIncidentSummary,
		Tools:             sampleTools(),
		CompositeToolMode: true,
		Profile:           ResolvePromptProfile(PromptProfileBalanced),
	})
	if !strings.Contains(prompt, "## Composite Investigation Tool") {
		t.Fatal("incident_summary with CompositeToolMode should include composite investigation block")
	}
	if !strings.Contains(prompt, "investigation__manage") {
		t.Fatal("incident_summary should reference investigation__manage tool")
	}

	// Without CompositeToolMode, should not include it.
	promptNoComposite := BuildSystemPrompt(PromptContext{
		Intent:            IntentIncidentSummary,
		Tools:             sampleTools(),
		CompositeToolMode: false,
		Profile:           ResolvePromptProfile(PromptProfileBalanced),
	})
	if strings.Contains(promptNoComposite, "## Composite Investigation Tool") {
		t.Fatal("incident_summary without CompositeToolMode should NOT include composite investigation block")
	}
}
