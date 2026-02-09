package agent

import (
	"strings"
	"testing"
)

func TestDeriveToolCallReason(t *testing.T) {
	testCases := []struct {
		name     string
		toolName string
		args     map[string]any
		expect   string
	}{
		{
			name:     "explicit_top_level_reason",
			toolName: "grafana__query_prometheus",
			args: map[string]any{
				"query":  "up",
				"reason": "Validate service health quickly.",
			},
			expect: "Validate service health quickly.",
		},
		{
			name:     "explicit_investigation_payload_reason",
			toolName: "investigation__manage",
			args: map[string]any{
				"action": "fetch_logs",
				"fetch_logs": map[string]any{
					"query":  `{app="api"} |= "error"`,
					"reason": "Check whether errors align with latency spikes.",
				},
			},
			expect: "Check whether errors align with latency spikes.",
		},
		{
			name:     "investigation_default_reason_with_query",
			toolName: "investigation__manage",
			args: map[string]any{
				"action": "fetch_metrics",
				"fetch_metrics": map[string]any{
					"query": "sum(rate(http_requests_total[5m]))",
				},
			},
			expect: "Fetch metrics to validate impact using query",
		},
		{
			name:     "generic_query_reason",
			toolName: "grafana__query_prometheus",
			args: map[string]any{
				"query": "up",
			},
			expect: "Run grafana__query_prometheus using query",
		},
		{
			name:     "generic_target_reason",
			toolName: "grafana__get_dashboard",
			args: map[string]any{
				"uid": "abc123",
			},
			expect: "Use grafana__get_dashboard for target",
		},
		{
			name:     "fallback_reason",
			toolName: "some__tool",
			args:     map[string]any{},
			expect:   "Use some__tool to gather evidence for the current request.",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := deriveToolCallReason(tc.toolName, tc.args)
			if !strings.Contains(got, tc.expect) {
				t.Fatalf("expected reason to contain %q, got %q", tc.expect, got)
			}
		})
	}
}
