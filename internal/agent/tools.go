package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

var (
	dashboardLookupToolShortNames = map[string]struct{}{
		"search_dashboards":           {},
		"get_dashboard_by_uid":        {},
		"get_dashboard_summary":       {},
		"get_dashboard_panel_queries": {},
		"list_datasources":            {},
	}
	queryHelpToolShortNames = map[string]struct{}{
		"query_prometheus":             {},
		"query_loki_logs":              {},
		"list_prometheus_metric_names": {},
		"list_prometheus_label_names":  {},
		"list_prometheus_label_values": {},
		"list_loki_label_names":        {},
		"list_datasources":             {},
		"search_dashboards":            {},
		"get_dashboard_panel_queries":  {},
	}
)

// filterToolsForIntent returns only the tools relevant to the given intent.
// This reduces function schema tokens sent to the LLM.
func filterToolsForIntent(tools []mcp.Tool, intent IntentClass) []mcp.Tool {
	switch intent {
	case IntentHowToDocs:
		return filterByPrefix(tools, "kb__")
	case IntentDashboardLookup:
		return filterByShortNames(tools, dashboardLookupToolShortNames)
	case IntentQueryHelp:
		return filterByShortNames(tools, queryHelpToolShortNames)
	default:
		return append([]mcp.Tool(nil), tools...)
	}
}

func filterByPrefix(tools []mcp.Tool, prefix string) []mcp.Tool {
	if len(tools) == 0 {
		return nil
	}
	filtered := make([]mcp.Tool, 0, len(tools))
	for _, t := range tools {
		if strings.HasPrefix(t.Name, prefix) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func filterByShortNames(tools []mcp.Tool, allowed map[string]struct{}) []mcp.Tool {
	if len(tools) == 0 {
		return nil
	}
	filtered := make([]mcp.Tool, 0, len(tools))
	for _, t := range tools {
		if _, ok := allowed[toolShortName(t.Name)]; ok {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// MCPToolsToOpenAI converts MCP tool definitions to OpenAI function calling format.
func MCPToolsToOpenAI(mcpTools []mcp.Tool) []openai.Tool {
	tools := make([]openai.Tool, 0, len(mcpTools))
	for _, t := range mcpTools {
		schema := t.InputSchema
		// OpenAI requires object schemas to have a non-nil properties field.
		// MCP tools with no parameters may omit it or send an empty map.
		if schema != nil {
			if typ, _ := schema["type"].(string); typ == "object" {
				props, _ := schema["properties"].(map[string]any)
				if len(props) == 0 {
					schema = map[string]any{
						"type":                 "object",
						"properties":           map[string]any{},
						"additionalProperties": false,
					}
				}
			}
		}
		params, err := json.Marshal(schema)
		if err != nil {
			continue
		}
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(params),
			},
		})
	}
	return tools
}

// InternalTools returns OpenAI tool definitions handled by the agent itself.
func InternalTools() []openai.Tool {
	return selectInternalTools(true, ResolvePromptProfile(PromptProfileBalanced))
}

func selectInternalTools(compositeToolMode bool, profile PromptProfile) []openai.Tool {
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "scratchpad__upsert_panel",
				Description: toolDescription(profile, "scratchpad__upsert_panel", "Create or update the user's per-session scratchpad Grafana panel for complex visualizations."),
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {"type": "string", "description": "PromQL/SQL query for the panel"},
						"title": {"type": "string", "description": "Panel title"},
						"description": {"type": "string", "description": "Panel description"},
						"panelType": {"type": "string", "description": "Optional Grafana panel type hint", "enum": ["timeseries", "stat", "table"]},
						"datasource": {"type": "object", "description": "Optional Grafana datasource object (uid/type)"},
						"timeRange": {"type": "object", "description": "Optional time range override", "additionalProperties": {"type": "string"}}
					},
					"required": ["query", "title"]
				}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "explore__open",
				Description: toolDescription(profile, "explore__open", "Open Grafana Explore with one or more ad-hoc queries for visualization."),
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {"type": "string", "description": "Convenience PromQL/LogQL query (single query)."},
						"queries": {
							"type": "array",
							"description": "Explicit query objects for Explore.",
							"items": {
								"type": "object",
								"properties": {
									"refId": {"type": "string"},
									"expr": {"type": "string"},
									"range": {"type": "boolean"},
									"instant": {"type": "boolean"},
									"legendFormat": {"type": "string"},
									"editorMode": {"type": "string"},
									"datasource": {"type": "object"}
								},
								"required": ["expr"]
							}
						},
						"datasource": {"type": "object", "description": "Grafana datasource object (uid/type)."},
						"timeRange": {"type": "object", "description": "Optional time range override", "additionalProperties": {"type": "string"}},
						"orgId": {"type": "integer"}
					}
				}`),
			},
		},
	}

	if compositeToolMode {
		tools = append([]openai.Tool{
			{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        "investigation__manage",
					Description: toolDescription(profile, "investigation__manage", "Composite investigation contract with typed actions for plan, metrics, logs, summarize, and next-step flow."),
					Parameters:  investigationManageToolParameters(),
				},
			},
		}, tools...)
	}

	return tools
}

func toolDescription(profile PromptProfile, name, fallback string) string {
	if profile.CompactToolDescriptions {
		switch name {
		case "investigation__manage":
			return "Run typed investigation actions: plan, fetch_metrics, fetch_logs, summarize, next_step."
		case "scratchpad__upsert_panel":
			return "Create or update a scratchpad dashboard panel."
		case "explore__open":
			return "Open Grafana Explore with one or more queries."
		}
	}
	return fallback
}

// RouteToolCall routes a tool call to the correct MCP client based on the name prefix.
// Tool names are formatted as "servertype__toolname" (e.g., "alertmanager__list_alerts").
func RouteToolCall(ctx context.Context, name string, args map[string]any, clients []mcp.Client) (any, error) {
	client, err := FindToolClient(ctx, name, clients)
	if err != nil {
		return nil, err
	}
	return client.InvokeTool(ctx, name, args)
}

// FindToolClient resolves the MCP client that serves a given tool name.
func FindToolClient(ctx context.Context, name string, clients []mcp.Client) (mcp.Client, error) {
	for _, c := range clients {
		tools, _ := c.DiscoverTools(ctx)
		for _, t := range tools {
			if t.Name == name {
				return c, nil
			}
		}
	}
	return nil, fmt.Errorf("no MCP server handles tool %q", name)
}
