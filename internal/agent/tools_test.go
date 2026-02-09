package agent

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/mcp"
)

func TestMCPToolsToOpenAI(t *testing.T) {
	mcpTools := []mcp.Tool{
		{
			Name:        "alertmanager__list_alerts",
			Description: "List active alerts from Alertmanager",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional filter expression",
					},
				},
			},
		},
		{
			Name:        "grafana__search_dashboards",
			Description: "Search Grafana dashboards",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type": "string",
					},
				},
			},
		},
	}

	result := MCPToolsToOpenAI(mcpTools)

	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}

	if result[0].Function.Name != "alertmanager__list_alerts" {
		t.Errorf("expected tool name alertmanager__list_alerts, got %s", result[0].Function.Name)
	}

	if result[1].Function.Description != "Search Grafana dashboards" {
		t.Errorf("unexpected description: %s", result[1].Function.Description)
	}

	// Verify parameters are valid JSON.
	var params map[string]any
	if err := json.Unmarshal(result[0].Function.Parameters.(json.RawMessage), &params); err != nil {
		t.Fatalf("failed to parse parameters: %v", err)
	}
	if params["type"] != "object" {
		t.Error("expected parameters type to be 'object'")
	}
}

func TestMCPToolsToOpenAI_Empty(t *testing.T) {
	result := MCPToolsToOpenAI(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 tools, got %d", len(result))
	}
}

func TestFilterToolsForIntent_HowToDocsOnlyKB(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__query_prometheus"},
		{Name: "kb__search_kb"},
		{Name: "kb__search_kb_semantic"},
		{Name: "alertmanager__list_alerts"},
		{Name: "kb__get_kb_section"},
	}

	filtered := filterToolsForIntent(tools, IntentHowToDocs)
	got := toolNames(filtered)
	want := []string{
		"kb__search_kb",
		"kb__search_kb_semantic",
		"kb__get_kb_section",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered=%v want=%v", got, want)
	}
}

func TestFilterToolsForIntent_DashboardLookupOnlyDashboardTools(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__query_prometheus"},
		{Name: "grafana__search_dashboards"},
		{Name: "grafana__get_dashboard_by_uid"},
		{Name: "grafana__get_dashboard_summary"},
		{Name: "grafana__get_dashboard_panel_queries"},
		{Name: "grafana__list_datasources"},
		{Name: "kb__search_kb_semantic"},
	}

	filtered := filterToolsForIntent(tools, IntentDashboardLookup)
	got := toolNames(filtered)
	want := []string{
		"grafana__search_dashboards",
		"grafana__get_dashboard_by_uid",
		"grafana__get_dashboard_summary",
		"grafana__get_dashboard_panel_queries",
		"grafana__list_datasources",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered=%v want=%v", got, want)
	}
}

func TestFilterToolsForIntent_LiveDataKeepsAllTools(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "alertmanager__list_alerts"},
		{Name: "grafana__search_dashboards"},
		{Name: "kb__search_kb"},
		{Name: "grafana__query_prometheus"},
	}

	filtered := filterToolsForIntent(tools, IntentLiveData)
	got := toolNames(filtered)
	want := toolNames(tools)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered=%v want=%v", got, want)
	}
}

func TestInternalTools_InvestigationManageContract(t *testing.T) {
	tools := InternalTools()

	var rawParams json.RawMessage
	found := false
	for _, tool := range tools {
		if tool.Function != nil && tool.Function.Name == "investigation__manage" {
			found = true
			params, ok := tool.Function.Parameters.(json.RawMessage)
			if !ok {
				t.Fatalf("expected investigation__manage parameters to be json.RawMessage, got %T", tool.Function.Parameters)
			}
			rawParams = params
			break
		}
	}
	if !found {
		t.Fatal("expected investigation__manage in InternalTools")
	}

	var schema map[string]any
	if err := json.Unmarshal(rawParams, &schema); err != nil {
		t.Fatalf("failed to parse investigation__manage schema: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("expected top-level schema type=object, got %v", schema["type"])
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false, got %v", schema["additionalProperties"])
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties object in schema, got %T", schema["properties"])
	}

	actionDef, ok := properties["action"].(map[string]any)
	if !ok {
		t.Fatalf("expected action property in schema, got %T", properties["action"])
	}
	enumValues, ok := actionDef["enum"].([]any)
	if !ok {
		t.Fatalf("expected action enum array, got %T", actionDef["enum"])
	}

	expectedActions := map[string]struct{}{
		investigationActionPlan:         {},
		investigationActionFetchMetrics: {},
		investigationActionFetchLogs:    {},
		investigationActionSummarize:    {},
		investigationActionNextStep:     {},
	}
	if len(enumValues) != len(expectedActions) {
		t.Fatalf("expected %d action enum values, got %d", len(expectedActions), len(enumValues))
	}
	for _, value := range enumValues {
		name, ok := value.(string)
		if !ok {
			t.Fatalf("expected enum value to be string, got %T", value)
		}
		delete(expectedActions, name)
	}
	if len(expectedActions) != 0 {
		t.Fatalf("missing action enum values: %v", expectedActions)
	}

	for _, payloadKey := range investigationPayloadByAction {
		payloadDef, ok := properties[payloadKey].(map[string]any)
		if !ok {
			t.Fatalf("expected payload definition for %q, got %T", payloadKey, properties[payloadKey])
		}
		if payloadDef["type"] != "object" {
			t.Fatalf("expected payload %q type=object, got %v", payloadKey, payloadDef["type"])
		}
	}

	requiredFields, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("expected required array, got %T", schema["required"])
	}
	hasAction := false
	for _, value := range requiredFields {
		if s, ok := value.(string); ok && s == "action" {
			hasAction = true
			break
		}
	}
	if !hasAction {
		t.Fatal("expected action in required fields")
	}
}

func TestSelectInternalTools_CompositeDisabled(t *testing.T) {
	tools := selectInternalTools(false, ResolvePromptProfile(PromptProfileBalanced))
	for _, tool := range tools {
		if tool.Function != nil && tool.Function.Name == "investigation__manage" {
			t.Fatal("did not expect investigation__manage when composite mode is disabled")
		}
	}
}

func TestSelectInternalTools_CompactProfileDescriptions(t *testing.T) {
	tools := selectInternalTools(true, ResolvePromptProfile(PromptProfileCompact))
	var foundInvestigation bool
	for _, tool := range tools {
		if tool.Function == nil || tool.Function.Name != "investigation__manage" {
			continue
		}
		foundInvestigation = true
		if !strings.Contains(tool.Function.Description, "typed investigation actions") {
			t.Fatalf("expected compact tool description, got %q", tool.Function.Description)
		}
	}
	if !foundInvestigation {
		t.Fatal("expected investigation__manage tool")
	}
}

func TestParseInvestigationManageArgs(t *testing.T) {
	t.Run("valid action and payload", func(t *testing.T) {
		args := map[string]any{
			"action": "FETCH_METRICS",
			"fetch_metrics": map[string]any{
				"query": "up",
			},
		}

		action, payloadKey, payload, err := parseInvestigationManageArgs(args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action != investigationActionFetchMetrics {
			t.Fatalf("expected normalized action=%q, got %q", investigationActionFetchMetrics, action)
		}
		if payloadKey != "fetch_metrics" {
			t.Fatalf("expected payload key fetch_metrics, got %q", payloadKey)
		}
		if payload["query"] != "up" {
			t.Fatalf("expected payload query to be preserved, got %v", payload["query"])
		}
	})

	t.Run("missing action", func(t *testing.T) {
		_, _, _, err := parseInvestigationManageArgs(map[string]any{
			"plan": map[string]any{"goal": "debug"},
		})
		if err == nil || !strings.Contains(err.Error(), "action is required") {
			t.Fatalf("expected missing action error, got %v", err)
		}
	})

	t.Run("unsupported action", func(t *testing.T) {
		_, _, _, err := parseInvestigationManageArgs(map[string]any{
			"action": "delete_everything",
		})
		if err == nil || !strings.Contains(err.Error(), "unsupported action") {
			t.Fatalf("expected unsupported action error, got %v", err)
		}
	})

	t.Run("missing payload object", func(t *testing.T) {
		_, _, _, err := parseInvestigationManageArgs(map[string]any{
			"action": "summarize",
		})
		if err == nil || !strings.Contains(err.Error(), "summarize payload is required") {
			t.Fatalf("expected missing payload error, got %v", err)
		}
	})

	t.Run("payload must be object", func(t *testing.T) {
		_, _, _, err := parseInvestigationManageArgs(map[string]any{
			"action":    "next_step",
			"next_step": "not-an-object",
		})
		if err == nil || !strings.Contains(err.Error(), "next_step payload must be an object") {
			t.Fatalf("expected payload type error, got %v", err)
		}
	})
}

type findToolClientMock struct {
	tools []mcp.Tool
}

func (m *findToolClientMock) Connect(context.Context) error { return nil }
func (m *findToolClientMock) Health(context.Context) error  { return nil }
func (m *findToolClientMock) DiscoverTools(context.Context) ([]mcp.Tool, error) {
	return m.tools, nil
}
func (m *findToolClientMock) InvokeTool(context.Context, string, map[string]any) (any, error) {
	return nil, errors.New("not implemented")
}

func TestFindToolClient(t *testing.T) {
	alert := &findToolClientMock{
		tools: []mcp.Tool{
			{Name: "alertmanager__list_alerts"},
		},
	}
	grafana := &findToolClientMock{
		tools: []mcp.Tool{
			{Name: "grafana__query_prometheus"},
		},
	}

	client, err := FindToolClient(context.Background(), "grafana__query_prometheus", []mcp.Client{alert, grafana})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != grafana {
		t.Fatalf("expected grafana client, got %#v", client)
	}

	_, err = FindToolClient(context.Background(), "grafana__missing_tool", []mcp.Client{alert, grafana})
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func toolNames(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	return names
}
