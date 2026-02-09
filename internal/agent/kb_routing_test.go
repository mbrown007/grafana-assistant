package agent

import (
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

func TestDecideKBRouting_DocsIntentInjects(t *testing.T) {
	intent := IntentResult{Label: IntentHowToDocs, Confidence: 0.90}
	decision := decideKBRouting(intent, "How do I configure Alertmanager routing?", nil, false, false)

	if !decision.Inject {
		t.Fatalf("expected KB injection for docs intent, got %+v", decision)
	}
	if decision.Reason != "docs_intent_relevance_met" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
	if decision.RelevanceScore < 2 {
		t.Fatalf("expected relevance score >= 2, got %d", decision.RelevanceScore)
	}
}

func TestDecideKBRouting_NonDocsIntentDoesNotInjectEvenWithLegacyTriggers(t *testing.T) {
	intent := IntentResult{Label: IntentLiveData, Confidence: 0.90}
	decision := decideKBRouting(intent, "What is CPU right now?", nil, true, true)

	if decision.Inject {
		t.Fatalf("expected no KB injection for non-doc intent, got %+v", decision)
	}
	if decision.Reason != "intent_not_docs" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
	if len(decision.Signals) == 0 {
		t.Fatalf("expected legacy-ignored signals to aid traceability")
	}
}

func TestDecideKBRouting_DocsIntentUsesContextSignal(t *testing.T) {
	intent := IntentResult{Label: IntentHowToDocs, Confidence: 0.78}
	decision := decideKBRouting(intent, "why does this fail", &api.DashboardContext{
		UID: "abc123",
	}, false, false)

	if !decision.Inject {
		t.Fatalf("expected KB injection with docs intent + context, got %+v", decision)
	}
}

func TestDecideKBRouting_CrossIntentDocsOverride(t *testing.T) {
	intent := IntentResult{Label: IntentIncidentSummary, Confidence: 0.88}
	decision := decideKBRouting(intent, "Summarize the incident and include the alertmanager runbook docs", nil, false, false)

	if !decision.Inject {
		t.Fatalf("expected KB injection for explicit docs request in non-doc intent, got %+v", decision)
	}
	if decision.Reason != "cross_intent_docs_override" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
}

func TestOrderMCPToolsForIntent_DocsIntentPrioritizesKBTools(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__search_dashboards"},
		{Name: "kb__search_kb_semantic"},
		{Name: "kb__search_kb"},
		{Name: "grafana__query_prometheus"},
	}

	ordered := orderMCPToolsForIntent(tools, IntentHowToDocs)
	if len(ordered) != len(tools) {
		t.Fatalf("unexpected ordered length: %d", len(ordered))
	}
	if ordered[0].Name != "kb__search_kb_semantic" && ordered[0].Name != "kb__search_kb" {
		t.Fatalf("expected KB tool first, got %q", ordered[0].Name)
	}
}

func TestIsDocsRetrievalTool(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"kb__search_kb", true},
		{"kb__search_kb_semantic", true},
		{"kb__get_kb_section", true},
		{"grafana__search_dashboards", false},
	}

	for _, tc := range tests {
		if got := isDocsRetrievalTool(tc.name); got != tc.want {
			t.Fatalf("tool %q = %v, want %v", tc.name, got, tc.want)
		}
	}
}
