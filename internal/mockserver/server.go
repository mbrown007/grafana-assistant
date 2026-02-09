package mockserver

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/brownster/grafana-assistant/internal/mcp"
)

// Server is a stdio MCP server backed by replay fixtures.
type Server struct {
	fixtureDir string
	serverType string
	matcher    *Matcher
	tools      []mcp.Tool
}

// New loads fixtures and prepares a mock MCP server.
func New(fixtureDir string) (*Server, error) {
	return NewForServerType(fixtureDir, "")
}

// NewForServerType loads fixtures and prepares a mock MCP server for one
// server type (e.g., grafana, alertmanager). Empty serverType disables filtering.
func NewForServerType(fixtureDir, serverType string) (*Server, error) {
	fixtures, err := LoadFixtures(fixtureDir)
	if err != nil {
		return nil, err
	}
	serverType = strings.ToLower(strings.TrimSpace(serverType))
	fixtures = FilterFixturesByServerType(fixtures, serverType)

	matcher, err := NewMatcher(fixtures)
	if err != nil {
		return nil, err
	}

	tools, err := loadToolsManifest(filepath.Join(fixtureDir, toolsManifestFile))
	if err != nil {
		return nil, err
	}
	tools = filterToolsByServerType(tools, serverType)
	if len(tools) == 0 {
		tools = buildToolsFromMatcher(matcher)
	}

	return &Server{
		fixtureDir: fixtureDir,
		serverType: serverType,
		matcher:    matcher,
		tools:      tools,
	}, nil
}

// ServeStdio starts the NDJSON JSON-RPC loop over stdin/stdout.
func (s *Server) ServeStdio() error {
	if s == nil {
		return errors.New("mock server is nil")
	}
	return s.serve(os.Stdin, os.Stdout)
}

func (s *Server) serve(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	writer := bufio.NewWriter(out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		resp, shouldReply, err := s.handleLine([]byte(line))
		if err != nil {
			return err
		}
		if !shouldReply {
			continue
		}
		if _, err := writer.Write(resp); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			return fmt.Errorf("write response newline: %w", err)
		}
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("flush response: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	return nil
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s *Server) handleLine(line []byte) ([]byte, bool, error) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return nil, false, fmt.Errorf("parse request: %w", err)
	}
	if strings.TrimSpace(req.Method) == "" {
		return nil, false, errors.New("request missing method")
	}

	switch req.Method {
	case "notifications/initialized":
		return nil, false, nil
	case "initialize":
		return marshalRPCSuccess(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "mock-mcp",
				"version": "1.0.0",
			},
		})
	case "tools/list":
		return marshalRPCSuccess(req.ID, map[string]any{"tools": s.tools})
	case "tools/call":
		var params toolCallParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return marshalRPCError(req.ID, -32602, "invalid params")
			}
		}
		params.Name = strings.TrimSpace(params.Name)
		if params.Name == "" {
			return marshalRPCError(req.ID, -32602, "tool call missing name")
		}
		response, ok := s.matcher.Match(params.Name, params.Arguments)
		if !ok {
			response = defaultNoDataResponse(params.Name)
		}
		return marshalRPCSuccess(req.ID, map[string]any{
			"content": []map[string]any{
				{
					"type": "json",
					"data": response,
				},
			},
		})
	default:
		return marshalRPCError(req.ID, -32601, "method not found")
	}
}

func defaultNoDataResponse(toolName string) map[string]any {
	return map[string]any{
		"status": "success",
		"data": map[string]any{
			"result": []any{},
		},
		"message": fmt.Sprintf("no fixture matched tool %q", toolName),
	}
}

func marshalRPCSuccess(id json.RawMessage, result any) ([]byte, bool, error) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      decodeID(id),
		"result":  result,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, false, fmt.Errorf("marshal success response: %w", err)
	}
	return b, true, nil
}

func marshalRPCError(id json.RawMessage, code int, message string) ([]byte, bool, error) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      decodeID(id),
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, false, fmt.Errorf("marshal error response: %w", err)
	}
	return b, true, nil
}

func decodeID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	var n int64
	if err := json.Unmarshal(id, &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(id, &s); err == nil {
		return s
	}
	return nil
}

func buildToolsFromMatcher(m *Matcher) []mcp.Tool {
	names := m.ToolNames()
	tools := make([]mcp.Tool, 0, len(names))
	for _, name := range names {
		tools = append(tools, mcp.Tool{
			Name:        name,
			Description: fmt.Sprintf("Mock fixture replay for %s", name),
			InputSchema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": true,
			},
		})
	}
	return tools
}

func loadToolsManifest(path string) ([]mcp.Tool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read tools manifest: %w", err)
	}

	var wrapped struct {
		Tools []mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Tools != nil {
		return normalizeTools(wrapped.Tools), nil
	}

	var list []mcp.Tool
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse tools manifest %s: %w", path, err)
	}
	return normalizeTools(list), nil
}

func normalizeTools(in []mcp.Tool) []mcp.Tool {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]mcp.Tool, 0, len(in))
	for _, tool := range in {
		tool.Name = strings.TrimSpace(tool.Name)
		if tool.Name == "" {
			continue
		}
		if _, ok := seen[tool.Name]; ok {
			continue
		}
		seen[tool.Name] = struct{}{}

		if tool.Description == "" {
			tool.Description = fmt.Sprintf("Mock fixture replay for %s", tool.Name)
		}
		if tool.InputSchema == nil {
			tool.InputSchema = map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": true,
			}
		}
		out = append(out, tool)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func filterToolsByServerType(in []mcp.Tool, serverType string) []mcp.Tool {
	if len(in) == 0 {
		return nil
	}
	serverType = strings.ToLower(strings.TrimSpace(serverType))
	if serverType == "" {
		out := make([]mcp.Tool, len(in))
		copy(out, in)
		return out
	}

	out := make([]mcp.Tool, 0, len(in))
	for _, tool := range in {
		prefix, hasPrefix := toolPrefix(tool.Name)
		if hasPrefix && prefix != serverType {
			continue
		}
		// Avoid double-prefixing by stdio client when manifests use prefixed names.
		if hasPrefix && prefix == serverType {
			tool.Name = shortToolName(tool.Name)
		}
		out = append(out, tool)
	}
	return out
}
