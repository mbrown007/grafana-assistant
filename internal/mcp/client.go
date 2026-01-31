package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Client is an MCP HTTP client with retry logic.
type Client struct {
	url        string
	serverType string
	httpClient *http.Client
	tools      []Tool
}

// NewClient creates a new MCP client for the given server URL and type.
func NewClient(url string, serverType string) *Client {
	return &Client{
		url:        strings.TrimRight(url, "/"),
		serverType: serverType,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Health checks if the MCP server is reachable.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mcp health check: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mcp health check: status %d", resp.StatusCode)
	}
	return nil
}

// DiscoverTools fetches available tools from the MCP server.
// Results are cached after the first successful call.
func (c *Client) DiscoverTools(ctx context.Context) ([]Tool, error) {
	if len(c.tools) > 0 {
		return c.tools, nil
	}

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	var result struct {
		Result struct {
			Tools []Tool `json:"tools"`
		} `json:"result"`
	}

	if err := c.jsonRPC(ctx, body, &result); err != nil {
		return nil, fmt.Errorf("discover tools: %w", err)
	}

	// Prefix tool names with server type for disambiguation.
	for i := range result.Result.Tools {
		result.Result.Tools[i].Name = c.serverType + "__" + result.Result.Tools[i].Name
	}

	c.tools = result.Result.Tools
	return c.tools, nil
}

// InvokeTool calls an MCP tool with the given arguments.
func (c *Client) InvokeTool(ctx context.Context, name string, args map[string]any) (any, error) {
	// Strip the server prefix for the actual call.
	actualName := strings.TrimPrefix(name, c.serverType+"__")

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixMilli(),
		"method":  "tools/call",
		"params": map[string]any{
			"name":      actualName,
			"arguments": args,
		},
	}

	var result struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
				Data any    `json:"data,omitempty"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := c.jsonRPC(ctx, body, &result); err != nil {
		return nil, fmt.Errorf("invoke tool %s: %w", name, err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("tool error: %s", result.Error.Message)
	}

	if len(result.Result.Content) > 0 {
		item := result.Result.Content[0]
		if item.Type == "text" {
			return item.Text, nil
		}
		return item.Data, nil
	}

	return nil, fmt.Errorf("tool %s returned no content", name)
}

// jsonRPC sends a JSON-RPC request with up to 3 retries.
func (c *Client) jsonRPC(ctx context.Context, body any, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("mcp server error: %d", resp.StatusCode)
			continue
		}

		return json.Unmarshal(data, dst)
	}

	return fmt.Errorf("mcp request failed after retries: %w", lastErr)
}
