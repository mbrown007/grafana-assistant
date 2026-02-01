package agent

import (
	"encoding/json"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/mcp"
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
