package server

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	kb "github.com/marcusz/monitoring-assistant/pkg/kb"
)

type MCPServer struct {
	index  *kb.Index
	server *server.MCPServer
}

func NewMCPServer(index *kb.Index) *MCPServer {
	return &MCPServer{
		index: index,
		server: server.NewMCPServer(
			"kb-mcp-server",
			"1.0.0",
		),
	}
}

func (s *MCPServer) GetServer() *server.MCPServer {
	return s.server
}

func (s *MCPServer) RegisterTools() {
	s.server.AddTool(s.searchTool(), s.handleSearch)
	s.server.AddTool(s.getSectionTool(), s.handleGetSection)
}

func (s *MCPServer) searchTool() mcp.Tool {
	return mcp.Tool{
		Name:        "search_kb",
		Description: "Search KB markdown sections by keyword. Returns matching titles and snippets.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Max results (default 5, max 20)",
					"default":     5,
					"minimum":     1,
					"maximum":     20,
				},
			},
			Required: []string{"query"},
		},
	}
}

func (s *MCPServer) getSectionTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_kb_section",
		Description: "Fetch a KB section by ID from search_kb.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Section ID returned by search_kb",
				},
			},
			Required: []string{"id"},
		},
	}
}

func (s *MCPServer) handleSearch(arguments map[string]any) (*mcp.CallToolResult, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	args.Limit = 5
	if arguments != nil {
		data, err := json.Marshal(arguments)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		if err := json.Unmarshal(data, &args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
	}
	if args.Limit <= 0 || args.Limit > 20 {
		args.Limit = 5
	}
	if args.Query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	weights := kb.WeightsFromTokens(kb.Tokenize(args.Query), 1)
	results := kb.Search(s.index, weights, args.Limit)
	type result struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Path    string `json:"path"`
		Snippet string `json:"snippet"`
		Score   int    `json:"score"`
	}
	payload := make([]result, 0, len(results))
	for _, r := range results {
		if r.Score <= 0 {
			continue
		}
		payload = append(payload, result{
			ID:      r.Section.ID,
			Title:   r.Section.Title,
			Path:    r.Section.Path,
			Snippet: kb.Snippet(r.Section.Content, 240),
			Score:   r.Score,
		})
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode results: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (s *MCPServer) handleGetSection(arguments map[string]any) (*mcp.CallToolResult, error) {
	var args struct {
		ID string `json:"id"`
	}
	if arguments != nil {
		data, err := json.Marshal(arguments)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		if err := json.Unmarshal(data, &args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
	}
	if args.ID == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	section, ok := kb.FindSection(s.index, args.ID)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("section not found: %s", args.ID)), nil
	}

	payload := map[string]any{
		"id":      section.ID,
		"title":   section.Title,
		"path":    section.Path,
		"content": section.Content,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode section: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
