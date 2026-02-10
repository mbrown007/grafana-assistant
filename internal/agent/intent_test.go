package agent

import (
	"testing"

	"github.com/brownster/grafana-assistant/internal/api"
)

func TestClassifyIntent_ByRepresentativePrompts(t *testing.T) {
	tests := []struct {
		name    string
		message string
		ctx     *api.DashboardContext
		want    IntentClass
	}{
		{
			name:    "live data status check",
			message: "What is the current CPU and error rate right now for checkout?",
			want:    IntentLiveData,
		},
		{
			name:    "query help promql",
			message: "Help me fix this PromQL query syntax error: sum by (pod) rate(http_requests_total[5m])",
			want:    IntentQueryHelp,
		},
		{
			name:    "dashboard lookup",
			message: "Find dashboard by UID fe9gm6guyzi0wd and open the panel for request latency",
			want:    IntentDashboardLookup,
		},
		{
			name:    "how-to docs",
			message: "How do I configure Alertmanager routing and silence rules? Any docs or runbook?",
			want:    IntentHowToDocs,
		},
		{
			name:    "incident summary",
			message: "Summarize the SEV1 incident timeline and probable root cause from tonight",
			want:    IntentIncidentSummary,
		},
		{
			name:    "explore context nudges query help",
			message: "Can you improve this query for better signal?",
			ctx: &api.DashboardContext{
				Explore: &api.ExploreContext{Datasource: "prometheus", Queries: []string{"rate(up[5m])"}},
			},
			want: IntentQueryHelp,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyIntent(tc.message, tc.ctx)
			if got.Label != tc.want {
				t.Fatalf("Label = %q, want %q (rationale=%q)", got.Label, tc.want, got.Rationale)
			}
			if got.Confidence <= 0 || got.Confidence > 1 {
				t.Fatalf("Confidence out of range: %v", got.Confidence)
			}
			if got.Rationale == "" {
				t.Fatalf("Rationale should be populated")
			}
		})
	}
}

func TestClassifyIntent_DefaultsToLiveDataWhenNoStrongSignal(t *testing.T) {
	got := ClassifyIntent("hello there", nil)
	if got.Label != IntentLiveData {
		t.Fatalf("Label = %q, want %q", got.Label, IntentLiveData)
	}
	if got.Confidence != 0.60 {
		t.Fatalf("Confidence = %v, want 0.60", got.Confidence)
	}
	if got.Rationale == "" {
		t.Fatalf("expected default rationale")
	}
}

func TestClassifyIntent_DashboardLookupWinsOverGenericDocs(t *testing.T) {
	got := ClassifyIntent("How do I find dashboard uid abc123 for payments service?", nil)
	if got.Label != IntentDashboardLookup {
		t.Fatalf("Label = %q, want %q (rationale=%q)", got.Label, IntentDashboardLookup, got.Rationale)
	}
}

func TestContainsAny_WordBoundaryAndLiteralBehavior(t *testing.T) {
	if containsAny("catalog service is healthy", "log") {
		t.Fatalf("expected word-boundary matcher to ignore substring in catalog")
	}
	if !containsAny("please check the log volume", "log") {
		t.Fatalf("expected word-boundary matcher to match standalone word")
	}
	if !containsAny("rate(http_requests_total[5m])", "rate(") {
		t.Fatalf("expected literal matcher to support symbol-based needles")
	}
}
