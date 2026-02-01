package agent

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

// MCPToolsToOpenAI converts MCP tool definitions to OpenAI function calling format.
func MCPToolsToOpenAI(mcpTools []mcp.Tool) []openai.Tool {
	tools := make([]openai.Tool, 0, len(mcpTools))
	for _, t := range mcpTools {
		params, err := json.Marshal(t.InputSchema)
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
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "scratchpad__upsert_panel",
				Description: "Create or update the user's per-session scratchpad Grafana panel for complex visualizations.",
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
	}
}

// RouteToolCall routes a tool call to the correct MCP client based on the name prefix.
// Tool names are formatted as "servertype__toolname" (e.g., "alertmanager__list_alerts").
func RouteToolCall(ctx context.Context, name string, args map[string]any, clients []mcp.Client) (any, error) {
	for _, c := range clients {
		tools, _ := c.DiscoverTools(ctx)
		for _, t := range tools {
			if t.Name == name {
				return c.InvokeTool(ctx, name, args)
			}
		}
	}
	return nil, fmt.Errorf("no MCP server handles tool %q", name)
}
