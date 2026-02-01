package mcp

import "context"

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Client is the interface implemented by MCP transport clients (SSE, stdio, etc.).
type Client interface {
	Connect(ctx context.Context) error
	Health(ctx context.Context) error
	DiscoverTools(ctx context.Context) ([]Tool, error)
	InvokeTool(ctx context.Context, name string, args map[string]any) (any, error)
}
