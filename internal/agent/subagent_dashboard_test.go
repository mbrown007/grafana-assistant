package agent

import (
	"reflect"
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/mcp"
)

func TestNewDashboardSubAgent(t *testing.T) {
	mgr := &Manager{
		tools: []mcp.Tool{
			{Name: "grafana__search_dashboards"},
			{Name: "grafana__get_dashboard_by_uid"},
			{Name: "grafana__get_dashboard_summary"},
			{Name: "grafana__get_dashboard_panel_queries"},
			{Name: "grafana__list_datasources"},
			{Name: "grafana__query_prometheus"},
			{Name: "kb__search_kb"},
		},
	}

	sa := mgr.newDashboardSubAgent()
	if sa == nil {
		t.Fatal("newDashboardSubAgent() returned nil")
	}
	if sa.Name != "dashboard" {
		t.Fatalf("Name = %q, want %q", sa.Name, "dashboard")
	}
	if sa.MaxIterations != 3 {
		t.Fatalf("MaxIterations = %d, want %d", sa.MaxIterations, 3)
	}
	if len(sa.SystemPrompt) == 0 {
		t.Fatal("SystemPrompt should not be empty")
	}
	if !strings.Contains(sa.SystemPrompt, "ONLY job is to find the right Grafana dashboard") {
		t.Fatalf("SystemPrompt missing dashboard-specialist instruction: %q", sa.SystemPrompt)
	}
	if !strings.Contains(sa.SystemPrompt, "Do NOT answer the user's question") {
		t.Fatalf("SystemPrompt missing output boundary instruction: %q", sa.SystemPrompt)
	}

	gotTools := toolNames(sa.Tools)
	wantTools := []string{
		"grafana__search_dashboards",
		"grafana__get_dashboard_by_uid",
		"grafana__get_dashboard_summary",
		"grafana__get_dashboard_panel_queries",
		"grafana__list_datasources",
	}
	if !reflect.DeepEqual(gotTools, wantTools) {
		t.Fatalf("Tools = %v, want %v", gotTools, wantTools)
	}
}

func TestIsDashboardLookupOnlyRequest(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "pure lookup",
			message: "find dashboard for checkout service",
			want:    true,
		},
		{
			name:    "lookup with analysis ask",
			message: "find dashboard for checkout latency and explain why errors spiked",
			want:    false,
		},
		{
			name:    "empty message",
			message: "",
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isDashboardLookupOnlyRequest(tc.message)
			if got != tc.want {
				t.Fatalf("isDashboardLookupOnlyRequest(%q) = %t, want %t", tc.message, got, tc.want)
			}
		})
	}
}
