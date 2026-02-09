package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brownster/grafana-assistant/internal/metrics"
	"github.com/brownster/grafana-assistant/internal/requestid"
)

// SSEClient is an MCP client that communicates via the SSE transport protocol.
// It maintains a persistent SSE connection for receiving responses and sends
// JSON-RPC requests via POST to the message endpoint.
type SSEClient struct {
	baseURL    string
	serverType string
	httpClient *http.Client
	tools      []Tool

	// SSE session state.
	mu          sync.Mutex
	messageURL  string
	connected   atomic.Bool
	responses   map[int64]chan json.RawMessage
	responsesMu sync.Mutex
	nextID      atomic.Int64
}

// NewSSEClient creates a new MCP client for the given server URL and type.
func NewSSEClient(url string, serverType string) *SSEClient {
	c := &SSEClient{
		baseURL:    strings.TrimRight(url, "/"),
		serverType: serverType,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		responses:  make(map[int64]chan json.RawMessage),
	}
	c.nextID.Store(1)
	return c
}

// Connect establishes the SSE connection and discovers available tools.
// This must be called before any tool operations.
func (c *SSEClient) Connect(ctx context.Context) error {
	if err := c.connectSSE(ctx); err != nil {
		return fmt.Errorf("sse connect: %w", err)
	}
	if _, err := c.DiscoverTools(ctx); err != nil {
		return fmt.Errorf("discover tools: %w", err)
	}
	return nil
}

// Health checks if the MCP server is reachable.
func (c *SSEClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
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

// connectSSE establishes a persistent SSE connection to the /sse endpoint
// and reads the message endpoint URL from the initial "endpoint" event.
func (c *SSEClient) connectSSE(ctx context.Context) error {
	sseURL := c.baseURL + "/sse"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	// Use a separate client without timeout for the long-lived SSE connection.
	sseClient := &http.Client{}
	resp, err := sseClient.Do(req)
	if err != nil {
		return fmt.Errorf("sse connect to %s: %w", sseURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("sse connect: status %d", resp.StatusCode)
	}

	// Read the initial "endpoint" event to get the message URL.
	endpointCh := make(chan string, 1)
	go c.readSSEStream(resp.Body, endpointCh)

	select {
	case msgURL := <-endpointCh:
		if msgURL == "" {
			return fmt.Errorf("sse: no endpoint received")
		}
		c.mu.Lock()
		// The server sends a relative path like /message?sessionId=xxx.
		if strings.HasPrefix(msgURL, "/") {
			c.messageURL = c.baseURL + msgURL
		} else {
			c.messageURL = msgURL
		}
		c.mu.Unlock()
		c.connected.Store(true)
		slog.InfoContext(ctx, "MCP SSE connected", "type", c.serverType, "messageURL", c.messageURL)
		return nil
	case <-time.After(10 * time.Second):
		resp.Body.Close()
		return fmt.Errorf("sse: timeout waiting for endpoint event")
	case <-ctx.Done():
		resp.Body.Close()
		return ctx.Err()
	}
}

// readSSEStream reads the SSE event stream. It sends the message endpoint URL
// on endpointCh (once) and dispatches JSON-RPC responses to waiting callers.
func (c *SSEClient) readSSEStream(body io.ReadCloser, endpointCh chan<- string) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	var eventType, data string
	endpointSent := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line = end of event.
			if eventType != "" && data != "" {
				switch eventType {
				case "endpoint":
					if !endpointSent {
						endpointCh <- data
						endpointSent = true
					}
				case "message":
					c.dispatchResponse(data)
				}
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("SSE stream error", "type", c.serverType, "error", err)
	}
	c.connected.Store(false)
	slog.Info("SSE stream closed", "type", c.serverType)
}

// dispatchResponse routes a JSON-RPC response from the SSE stream to the
// caller waiting on that request ID.
func (c *SSEClient) dispatchResponse(data string) {
	var envelope struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		slog.Debug("SSE: ignoring non-JSON message", "data", data[:min(len(data), 100)])
		return
	}

	// Parse the ID (can be number or string).
	var id int64
	if err := json.Unmarshal(envelope.ID, &id); err != nil {
		// Not a response we're waiting for (e.g., a notification).
		return
	}

	c.responsesMu.Lock()
	ch, ok := c.responses[id]
	if ok {
		delete(c.responses, id)
	}
	c.responsesMu.Unlock()

	if ok {
		ch <- []byte(data)
	}
}

// jsonRPC sends a JSON-RPC request via POST to the message endpoint and waits
// for the response on the SSE stream.
func (c *SSEClient) jsonRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	messageURL := c.messageURL
	c.mu.Unlock()

	if messageURL == "" {
		return nil, fmt.Errorf("not connected to MCP server %s", c.serverType)
	}

	id := c.nextID.Add(1)

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		body["params"] = params
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Register a response channel before sending.
	respCh := make(chan json.RawMessage, 1)
	c.responsesMu.Lock()
	c.responses[id] = respCh
	c.responsesMu.Unlock()

	// Clean up on failure.
	defer func() {
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, messageURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if rid := requestid.FromContext(ctx); rid != "" {
		req.Header.Set("X-Request-ID", rid)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp post: %w", err)
	}
	resp.Body.Close()

	// The server returns 202 Accepted (or 200). The actual response comes on the SSE stream.
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp message: status %d", resp.StatusCode)
	}

	// Wait for the response on the SSE stream.
	select {
	case raw := <-respCh:
		return raw, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("mcp %s: timeout waiting for response", method)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// DiscoverTools fetches available tools from the MCP server.
// Results are cached after the first successful call.
func (c *SSEClient) DiscoverTools(ctx context.Context) ([]Tool, error) {
	if len(c.tools) > 0 {
		return c.tools, nil
	}

	raw, err := c.jsonRPC(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("discover tools: %w", err)
	}

	var resp struct {
		Result struct {
			Tools []Tool `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse tools response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	// Prefix tool names with server type for disambiguation.
	for i := range resp.Result.Tools {
		resp.Result.Tools[i].Name = c.serverType + "__" + resp.Result.Tools[i].Name
	}

	c.tools = resp.Result.Tools
	return c.tools, nil
}

// InvokeTool calls an MCP tool with the given arguments.
func (c *SSEClient) InvokeTool(ctx context.Context, name string, args map[string]any) (any, error) {
	// Strip the server prefix for the actual call.
	actualName := strings.TrimPrefix(name, c.serverType+"__")

	start := time.Now()
	raw, err := c.jsonRPC(ctx, "tools/call", map[string]any{
		"name":      actualName,
		"arguments": args,
	})
	duration := time.Since(start).Seconds()
	metrics.MCPToolCallDuration.WithLabelValues(name).Observe(duration)

	if err != nil {
		metrics.MCPToolCallsTotal.WithLabelValues(name, "error").Inc()
		metrics.ErrorsTotal.WithLabelValues("mcp").Inc()
		metrics.ErrorsTotalBySource.WithLabelValues("tool_call").Inc()
		return nil, fmt.Errorf("invoke tool %s: %w", name, err)
	}

	var resp struct {
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
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse tool response: %w", err)
	}

	if resp.Error != nil {
		metrics.MCPToolCallsTotal.WithLabelValues(name, "error").Inc()
		metrics.ErrorsTotal.WithLabelValues("mcp").Inc()
		metrics.ErrorsTotalBySource.WithLabelValues("tool_call").Inc()
		return nil, fmt.Errorf("tool error: %s", resp.Error.Message)
	}

	metrics.MCPToolCallsTotal.WithLabelValues(name, "ok").Inc()

	if len(resp.Result.Content) > 0 {
		item := resp.Result.Content[0]
		if item.Type == "text" {
			return item.Text, nil
		}
		return item.Data, nil
	}

	return nil, fmt.Errorf("tool %s returned no content", name)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
