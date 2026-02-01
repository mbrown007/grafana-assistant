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

// RouteToolCall routes a tool call to the correct MCP client based on the name prefix.
// Tool names are formatted as "servertype__toolname" (e.g., "alertmanager__list_alerts").
func RouteToolCall(ctx context.Context, name string, args map[string]any, clients []*mcp.Client) (any, error) {
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

