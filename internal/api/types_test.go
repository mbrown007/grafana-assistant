package api

import (
	"encoding/json"
	"testing"
)

// Canonical JSON examples from docs/ARTIFACT_SCHEMA.md.
// These tests ensure Go types can round-trip through JSON without data loss.

func TestArtifactData_Chart(t *testing.T) {
	const input = `{
		"type": "chart",
		"title": "Request Rate by Day",
		"chartType": "line",
		"data": [
			{"name": "Mon", "requests": 1200, "errors": 15},
			{"name": "Tue", "requests": 1350, "errors": 22}
		]
	}`
	roundTrip[ArtifactData](t, input, func(a ArtifactData) {
		assertEqual(t, "type", a.Type, "chart")
		assertEqual(t, "title", a.Title, "Request Rate by Day")
		assertEqual(t, "chartType", a.ChartType, "line")
		if a.Data == nil {
			t.Error("data should not be nil")
		}
	})
}

func TestArtifactData_Table(t *testing.T) {
	const input = `{
		"type": "table",
		"title": "Top Alerting Rules",
		"columns": [
			{"key": "rule", "label": "Rule Name"},
			{"key": "fires", "label": "Fires (24h)", "align": "right"}
		],
		"rows": [
			{"rule": "HighCPU", "fires": 12, "severity": "warning"}
		]
	}`
	roundTrip[ArtifactData](t, input, func(a ArtifactData) {
		assertEqual(t, "type", a.Type, "table")
		if len(a.Columns) != 2 {
			t.Errorf("columns: got %d, want 2", len(a.Columns))
		}
		if a.Columns[1].Align != "right" {
			t.Errorf("columns[1].align = %q, want %q", a.Columns[1].Align, "right")
		}
		if a.Rows == nil {
			t.Error("rows should not be nil")
		}
	})
}

func TestArtifactData_MetricCards(t *testing.T) {
	const input = `{
		"type": "metric-cards",
		"title": "System Overview",
		"metrics": [
			{"label": "Active Alerts", "value": 7, "change": 40, "changeLabel": "vs yesterday", "icon": "alert", "color": "red"},
			{"label": "Uptime", "value": "99.97%", "icon": "activity", "color": "green"}
		]
	}`
	roundTrip[ArtifactData](t, input, func(a ArtifactData) {
		assertEqual(t, "type", a.Type, "metric-cards")
		if len(a.Metrics) != 2 {
			t.Errorf("metrics: got %d, want 2", len(a.Metrics))
		}
		if a.Metrics[0].Label != "Active Alerts" {
			t.Errorf("metrics[0].label = %q, want %q", a.Metrics[0].Label, "Active Alerts")
		}
		if a.Metrics[0].Change == nil || *a.Metrics[0].Change != 40 {
			t.Errorf("metrics[0].change = %v, want 40", a.Metrics[0].Change)
		}
		// Value can be number or string — second metric has string value.
		if a.Metrics[1].Value == nil {
			t.Error("metrics[1].value should not be nil")
		}
	})
}

func TestArtifactData_Report(t *testing.T) {
	const input = `{
		"type": "report",
		"title": "Daily Monitoring Summary",
		"subtitle": "2025-01-30",
		"sections": [
			{"type": "summary", "title": "Executive Summary", "content": "All systems healthy."},
			{"type": "metrics", "metrics": [
				{"label": "Alerts", "value": 3, "icon": "alert", "color": "amber"}
			]},
			{"type": "chart", "title": "Error Rate", "chartType": "area", "data": [
				{"name": "00:00", "errors": 2}
			]}
		]
	}`
	roundTrip[ArtifactData](t, input, func(a ArtifactData) {
		assertEqual(t, "type", a.Type, "report")
		assertEqual(t, "subtitle", a.Subtitle, "2025-01-30")
		if len(a.Sections) != 3 {
			t.Errorf("sections: got %d, want 3", len(a.Sections))
		}
		assertEqual(t, "sections[0].type", a.Sections[0].Type, "summary")
		assertEqual(t, "sections[1].type", a.Sections[1].Type, "metrics")
		assertEqual(t, "sections[2].chartType", a.Sections[2].ChartType, "area")
	})
}

func TestStreamChunk_Token(t *testing.T) {
	const input = `{"type": "token", "message": "Let me check "}`
	roundTrip[StreamChunk](t, input, func(c StreamChunk) {
		assertEqual(t, "type", c.Type, "token")
		assertEqual(t, "message", c.Message, "Let me check ")
	})
}

func TestStreamChunk_Tool(t *testing.T) {
	const input = `{
		"type": "tool",
		"tool": "alertmanager__list_alerts",
		"reason": "Check active alert state for the current incident.",
		"arguments": {"state": "active"}
	}`
	roundTrip[StreamChunk](t, input, func(c StreamChunk) {
		assertEqual(t, "type", c.Type, "tool")
		assertEqual(t, "tool", c.Tool, "alertmanager__list_alerts")
		assertEqual(t, "reason", c.Reason, "Check active alert state for the current incident.")
		if c.Arguments["state"] != "active" {
			t.Errorf("arguments.state = %v, want %q", c.Arguments["state"], "active")
		}
	})
}

func TestStreamChunk_ToolWithResultObject(t *testing.T) {
	const input = `{
		"type": "tool",
		"tool": "grafana__query_prometheus",
		"tool_id": "tool-call-1",
		"result": {
			"status": "ok",
			"raw": {
				"series": [{"metric": "up", "value": 1}]
			}
		}
	}`

	roundTrip[StreamChunk](t, input, func(c StreamChunk) {
		assertEqual(t, "type", c.Type, "tool")
		assertEqual(t, "tool", c.Tool, "grafana__query_prometheus")
		assertEqual(t, "tool_id", c.ToolID, "tool-call-1")

		result, ok := c.Result.(map[string]any)
		if !ok {
			t.Fatalf("result should decode to object, got %T", c.Result)
		}
		if result["status"] != "ok" {
			t.Fatalf("result.status = %v, want %q", result["status"], "ok")
		}

		raw, ok := result["raw"].(map[string]any)
		if !ok {
			t.Fatalf("result.raw should decode to object, got %T", result["raw"])
		}
		series, ok := raw["series"].([]any)
		if !ok || len(series) != 1 {
			t.Fatalf("result.raw.series = %#v, want one-entry list", raw["series"])
		}
	})
}

func TestChatRequest(t *testing.T) {
	const input = `{
		"message": "What alerts are firing?",
		"session_id": "user-42-org-1",
		"dashboard_context": {
			"uid": "abc123",
			"name": "Production Overview",
			"folder": "Operations",
			"tags": ["production", "sre"],
			"time_range": {"from": "now-1h", "to": "now"},
			"variables": {"server": "prod-01"}
		}
	}`
	roundTrip[ChatRequest](t, input, func(r ChatRequest) {
		assertEqual(t, "message", r.Message, "What alerts are firing?")
		assertEqual(t, "session_id", r.SessionID, "user-42-org-1")
		if r.DashboardContext == nil {
			t.Fatal("dashboard_context should not be nil")
		}
		assertEqual(t, "uid", r.DashboardContext.UID, "abc123")
		if len(r.DashboardContext.Tags) != 2 {
			t.Errorf("tags: got %d, want 2", len(r.DashboardContext.Tags))
		}
		if r.DashboardContext.Variables["server"] != "prod-01" {
			t.Errorf("variables.server = %q, want %q", r.DashboardContext.Variables["server"], "prod-01")
		}
	})
}

func TestChatResponse(t *testing.T) {
	const input = `{"response": "There are 3 active alerts.", "session_id": "user-42-org-1"}`
	roundTrip[ChatResponse](t, input, func(r ChatResponse) {
		assertEqual(t, "response", r.Response, "There are 3 active alerts.")
		assertEqual(t, "session_id", r.SessionID, "user-42-org-1")
	})
}

// roundTrip unmarshals JSON into T, validates with fn, re-marshals, and verifies
// that a second unmarshal produces an equivalent struct.
func roundTrip[T any](t *testing.T, input string, fn func(T)) {
	t.Helper()

	var first T
	if err := json.Unmarshal([]byte(input), &first); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fn(first)

	b, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var second T
	if err := json.Unmarshal(b, &second); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}

	b1, _ := json.Marshal(first)
	b2, _ := json.Marshal(second)
	if string(b1) != string(b2) {
		t.Errorf("round-trip mismatch:\n  first:  %s\n  second: %s", b1, b2)
	}
}

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}
