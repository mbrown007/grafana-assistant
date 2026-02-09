package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// NewFilteredClient wraps an MCP client with per-server tool allow/deny filters.
// If both lists are empty, it returns the original client unchanged.
func NewFilteredClient(base Client, serverType string, allowlist, denylist []string) Client {
	if len(allowlist) == 0 && len(denylist) == 0 {
		return base
	}

	f := &filteredClient{
		base:       base,
		serverType: strings.TrimSpace(serverType),
	}
	f.allowExact, f.allowShort = buildToolMatchSets(allowlist)
	f.denyExact, f.denyShort = buildToolMatchSets(denylist)
	f.hasAllow = len(f.allowExact) > 0 || len(f.allowShort) > 0
	return f
}

type filteredClient struct {
	base       Client
	serverType string

	hasAllow bool

	allowExact map[string]struct{}
	allowShort map[string]struct{}
	denyExact  map[string]struct{}
	denyShort  map[string]struct{}

	mu          sync.Mutex
	discoverMu  sync.Mutex
	cachedTools []Tool
}

func (c *filteredClient) Connect(ctx context.Context) error {
	return c.base.Connect(ctx)
}

func (c *filteredClient) Health(ctx context.Context) error {
	return c.base.Health(ctx)
}

func (c *filteredClient) DiscoverTools(ctx context.Context) ([]Tool, error) {
	c.mu.Lock()
	if c.cachedTools != nil {
		tools := make([]Tool, len(c.cachedTools))
		copy(tools, c.cachedTools)
		c.mu.Unlock()
		return tools, nil
	}
	c.mu.Unlock()

	// Avoid duplicate upstream discovery work under concurrent callers.
	c.discoverMu.Lock()
	defer c.discoverMu.Unlock()

	c.mu.Lock()
	if c.cachedTools != nil {
		tools := make([]Tool, len(c.cachedTools))
		copy(tools, c.cachedTools)
		c.mu.Unlock()
		return tools, nil
	}
	c.mu.Unlock()

	tools, err := c.base.DiscoverTools(ctx)
	if err != nil {
		return nil, err
	}

	kept := make([]Tool, 0, len(tools))
	dropped := make([]string, 0)
	for _, t := range tools {
		if c.isToolAllowed(t.Name) {
			kept = append(kept, t)
			continue
		}
		dropped = append(dropped, t.Name)
	}

	slog.InfoContext(ctx, "applied MCP tool filters",
		"server_type", c.serverType,
		"discovered_count", len(tools),
		"kept_count", len(kept),
		"dropped_count", len(dropped),
		"has_allowlist", c.hasAllow,
		"denylist_count", len(c.denyExact)+len(c.denyShort),
		"dropped_preview", previewToolNames(dropped, 12),
	)

	c.mu.Lock()
	if c.cachedTools == nil {
		c.cachedTools = kept
	}
	toolsOut := make([]Tool, len(c.cachedTools))
	copy(toolsOut, c.cachedTools)
	c.mu.Unlock()
	return toolsOut, nil
}

func (c *filteredClient) InvokeTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if !c.isToolAllowed(name) {
		slog.WarnContext(ctx, "blocked MCP tool by configuration",
			"server_type", c.serverType,
			"tool", name,
		)
		return nil, fmt.Errorf("tool %q is not enabled for server %q", name, c.serverType)
	}
	return c.base.InvokeTool(ctx, name, args)
}

func (c *filteredClient) isToolAllowed(toolName string) bool {
	if c.isToolDenied(toolName) {
		return false
	}
	if !c.hasAllow {
		return true
	}

	if _, ok := c.allowExact[toolName]; ok {
		return true
	}
	if _, ok := c.allowShort[shortToolName(toolName)]; ok {
		return true
	}
	return false
}

func (c *filteredClient) isToolDenied(toolName string) bool {
	if _, ok := c.denyExact[toolName]; ok {
		return true
	}
	if _, ok := c.denyShort[shortToolName(toolName)]; ok {
		return true
	}
	return false
}

func buildToolMatchSets(entries []string) (map[string]struct{}, map[string]struct{}) {
	exact := make(map[string]struct{}, len(entries))
	short := make(map[string]struct{}, len(entries))

	for _, raw := range entries {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if strings.Contains(name, "__") {
			exact[name] = struct{}{}
			continue
		}
		short[name] = struct{}{}
	}
	return exact, short
}

func shortToolName(name string) string {
	parts := strings.SplitN(name, "__", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		return parts[1]
	}
	return name
}

func previewToolNames(names []string, limit int) []string {
	if len(names) <= limit {
		return names
	}
	out := make([]string, 0, limit+1)
	out = append(out, names[:limit]...)
	out = append(out, fmt.Sprintf("...+%d more", len(names)-limit))
	return out
}
