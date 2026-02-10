package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brownster/grafana-assistant/internal/metrics"
)

// StdioConfig describes how to launch an MCP server over stdio.
type StdioConfig struct {
	Command    string
	Args       []string
	Env        map[string]string
	WorkDir    string
	ServerType string
}

// StdioClient is an MCP client that communicates over stdin/stdout using
// JSON-RPC with Content-Length framing.
type StdioClient struct {
	serverType string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	tools      []Tool

	writeMu     sync.Mutex
	responses   map[int64]chan json.RawMessage
	responsesMu sync.Mutex
	nextID      atomic.Int64
	connected   atomic.Bool
}

// NewStdioClient creates a new stdio MCP client.
func NewStdioClient(cfg StdioConfig) (*StdioClient, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("stdio command is required")
	}

	c := &StdioClient{
		serverType: cfg.ServerType,
		responses:  make(map[int64]chan json.RawMessage),
	}
	c.nextID.Store(1)

	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = buildEnv(cfg.Env)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdio stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdio stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stdio stderr pipe: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout

	go c.readStderr(stderr)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("stdio start: %w", err)
	}

	go c.readStream(stdout)
	go c.waitForExit()

	c.connected.Store(true)
	slog.Info("MCP stdio started", "type", c.serverType, "command", cfg.Command)
	return c, nil
}

// Connect initializes the MCP session and discovers available tools.
// The process is started in NewStdioClient.
func (c *StdioClient) Connect(ctx context.Context) error {
	if !c.connected.Load() {
		return fmt.Errorf("stdio server not running for %s", c.serverType)
	}

	// MCP protocol requires initialize handshake before any other calls.
	if err := c.initialize(ctx); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	if _, err := c.DiscoverTools(ctx); err != nil {
		return fmt.Errorf("discover tools: %w", err)
	}
	return nil
}

// initialize performs the MCP protocol handshake.
func (c *StdioClient) initialize(ctx context.Context) error {
	// Send initialize request
	raw, err := c.jsonRPC(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "monitoring-assistant",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return fmt.Errorf("initialize request: %w", err)
	}

	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("parse initialize response: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	// Send initialized notification (no response expected)
	notification := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal initialized notification: %w", err)
	}
	if err := c.writeMessage(payload); err != nil {
		return fmt.Errorf("send initialized notification: %w", err)
	}

	return nil
}

// Health checks if the stdio server process is running.
func (c *StdioClient) Health(_ context.Context) error {
	if c.connected.Load() {
		return nil
	}
	return fmt.Errorf("stdio server not connected for %s", c.serverType)
}

// DiscoverTools fetches available tools from the MCP server.
// Results are cached after the first successful call.
func (c *StdioClient) DiscoverTools(ctx context.Context) ([]Tool, error) {
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
func (c *StdioClient) InvokeTool(ctx context.Context, name string, args map[string]any) (any, error) {
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

func (c *StdioClient) jsonRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if !c.connected.Load() {
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

	respCh := make(chan json.RawMessage, 1)
	c.responsesMu.Lock()
	c.responses[id] = respCh
	c.responsesMu.Unlock()

	defer func() {
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
	}()

	if err := c.writeMessage(payload); err != nil {
		return nil, err
	}

	select {
	case raw := <-respCh:
		return raw, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("mcp %s: timeout waiting for response", method)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *StdioClient) writeMessage(payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Write raw JSON followed by newline (NDJSON format)
	if _, err := c.stdin.Write(payload); err != nil {
		return fmt.Errorf("stdio write: %w", err)
	}
	if _, err := c.stdin.Write([]byte("\n")); err != nil {
		return fmt.Errorf("stdio write newline: %w", err)
	}
	return nil
}

func (c *StdioClient) readStream(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	// Increase buffer size to handle large JSON responses
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// Make a copy since scanner reuses buffer
		msg := make([]byte, len(line))
		copy(msg, line)
		c.dispatchResponse(msg)
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("stdio read error", "type", c.serverType, "error", err)
	}
	c.connected.Store(false)
}

func (c *StdioClient) dispatchResponse(data []byte) {
	var envelope struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		slog.Debug("stdio: ignoring non-JSON message", "type", c.serverType)
		return
	}

	var id int64
	if err := json.Unmarshal(envelope.ID, &id); err != nil {
		return
	}

	c.responsesMu.Lock()
	ch, ok := c.responses[id]
	if ok {
		delete(c.responses, id)
	}
	c.responsesMu.Unlock()

	if ok {
		ch <- data
	}
}


func (c *StdioClient) readStderr(stderr io.ReadCloser) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		slog.Info("stdio stderr", "type", c.serverType, "line", line)
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("stdio stderr error", "type", c.serverType, "error", err)
	}
}

func (c *StdioClient) waitForExit() {
	if c.cmd == nil {
		return
	}
	err := c.cmd.Wait()
	if err != nil {
		slog.Warn("stdio process exited", "type", c.serverType, "error", err)
	} else {
		slog.Info("stdio process exited", "type", c.serverType)
	}
	c.connected.Store(false)
}

func buildEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}
