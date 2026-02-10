package agent

import (
	"strings"
	"testing"
)

func TestCoordinatorDecide(t *testing.T) {
	mgr := &Manager{}

	cases := []struct {
		name               string
		intent             IntentResult
		message            string
		wantUseDirect      bool
		wantSubAgent       string
		wantReasonContains string
	}{
		{
			name: "docs_high_confidence_uses_direct",
			intent: IntentResult{
				Label:      IntentHowToDocs,
				Confidence: 0.95,
			},
			message:            "how do i configure alert routing?",
			wantUseDirect:      true,
			wantReasonContains: "high-confidence docs intent",
		},
		{
			name: "dashboard_high_confidence_delegates_dashboard_subagent",
			intent: IntentResult{
				Label:      IntentDashboardLookup,
				Confidence: 0.95,
			},
			message:            "find the node exporter dashboard",
			wantUseDirect:      false,
			wantSubAgent:       "dashboard",
			wantReasonContains: "dashboard lookup",
		},
		{
			name: "dashboard_high_confidence_with_analysis_signals_uses_direct",
			intent: IntentResult{
				Label:      IntentDashboardLookup,
				Confidence: 0.95,
			},
			message:            "find dashboard for API latency and explain why it spiked",
			wantUseDirect:      true,
			wantReasonContains: "analysis signals",
		},
		{
			name: "incident_summary_delegates_investigation_subagent",
			intent: IntentResult{
				Label:      IntentIncidentSummary,
				Confidence: 0.65,
			},
			message:            "summarize this incident",
			wantUseDirect:      false,
			wantSubAgent:       "investigation",
			wantReasonContains: "incident summary",
		},
		{
			name: "live_data_uses_direct_default",
			intent: IntentResult{
				Label:      IntentLiveData,
				Confidence: 0.94,
			},
			message:            "what is CPU usage now?",
			wantUseDirect:      true,
			wantReasonContains: "default direct path",
		},
		{
			name: "low_confidence_dashboard_uses_direct_default",
			intent: IntentResult{
				Label:      IntentDashboardLookup,
				Confidence: 0.60,
			},
			message:            "dashboard",
			wantUseDirect:      true,
			wantReasonContains: "default direct path",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mgr.coordinatorDecide(tc.intent, tc.message)
			if got.UseDirect != tc.wantUseDirect {
				t.Fatalf("UseDirect = %t, want %t", got.UseDirect, tc.wantUseDirect)
			}
			if tc.wantSubAgent != "" {
				if len(got.SubAgents) == 0 {
					t.Fatalf("expected sub-agent %q, got none", tc.wantSubAgent)
				}
				if got.SubAgents[0] != tc.wantSubAgent {
					t.Fatalf("SubAgents[0] = %q, want %q", got.SubAgents[0], tc.wantSubAgent)
				}
			} else if len(got.SubAgents) != 0 {
				t.Fatalf("expected no sub-agents, got %v", got.SubAgents)
			}
			if tc.wantReasonContains != "" && got.Reason != tc.wantReasonContains && !containsIgnoreCase(got.Reason, tc.wantReasonContains) {
				t.Fatalf("Reason = %q, want to contain %q", got.Reason, tc.wantReasonContains)
			}
		})
	}
}

func containsIgnoreCase(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func TestSynthesizeSubAgentResponse_Investigation(t *testing.T) {
	decision := CoordinatorDecision{
		SubAgents: []string{"investigation"},
		Reason:    "incident summary -> delegate to investigation specialist",
	}
	results := []SubAgentResult{
		{
			Summary:   "Latency spiked at 14:03 UTC after a deploy; error rate increased to 2.1%.",
			ToolsUsed: []string{"grafana__query_prometheus", "grafana__query_loki_logs"},
		},
	}

	got := synthesizeSubAgentResponse(decision, results)
	if !containsIgnoreCase(got, "Investigation findings") {
		t.Fatalf("expected investigation heading, got %q", got)
	}
	if !containsIgnoreCase(got, "Evidence sources") {
		t.Fatalf("expected evidence sources line, got %q", got)
	}
	if !containsIgnoreCase(got, "query_prometheus") {
		t.Fatalf("expected tool reference in synthesis, got %q", got)
	}
}
