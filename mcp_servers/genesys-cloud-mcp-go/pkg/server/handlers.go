package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sabio/genesys-cloud-mcp-go/pkg/genesys"
	"golang.org/x/time/rate"
)

// MCPServer wraps the Genesys client and MCP server
type MCPServer struct {
	client  *genesys.Client
	server  *server.MCPServer
	limiter *rate.Limiter
}

// NewMCPServer creates a new MCP server with Genesys client
func NewMCPServer(client *genesys.Client) *MCPServer {
	return &MCPServer{
		client: client,
		server: server.NewMCPServer(
			"genesys-cloud-mcp-server",
			"1.0.0",
		),
		limiter: newRateLimiter(),
	}
}

// GetServer returns the underlying MCP server
func (s *MCPServer) GetServer() *server.MCPServer {
	return s.server
}

// RegisterTools registers all MCP tools
func (s *MCPServer) RegisterTools() {
	// Search Queues
	s.server.AddTool(mcp.Tool{
		Name:        "search_queues",
		Description: "Searches for routing queues based on their name",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Queue name to search for (supports wildcards)",
				},
				"pageNumber": map[string]interface{}{
					"type":        "number",
					"description": "Page number (default: 1)",
					"default":     1,
				},
				"pageSize": map[string]interface{}{
					"type":        "number",
					"description": "Page size (default: 25, max: 100)",
					"default":     25,
				},
			},
		},
	}, s.handleSearchQueues)

	// Query Queue Volumes
	s.server.AddTool(mcp.Tool{
		Name:        "query_queue_volumes",
		Description: "Returns conversation volumes for specified queues",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"queueIds": map[string]interface{}{
					"type":        "array",
					"description": "List of queue IDs",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"startTime": map[string]interface{}{
					"type":        "string",
					"description": "Start time (ISO 8601)",
				},
				"endTime": map[string]interface{}{
					"type":        "string",
					"description": "End time (ISO 8601)",
				},
			},
			Required: []string{"queueIds", "startTime", "endTime"},
		},
	}, s.handleQueryQueueVolumes)

	// Sample Conversations By Queue
	s.server.AddTool(mcp.Tool{
		Name:        "sample_conversations_by_queue",
		Description: "Retrieves a sample of conversation IDs for a queue",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"queueId": map[string]interface{}{
					"type":        "string",
					"description": "Queue ID",
				},
				"startTime": map[string]interface{}{
					"type":        "string",
					"description": "Start time (ISO 8601)",
				},
				"endTime": map[string]interface{}{
					"type":        "string",
					"description": "End time (ISO 8601)",
				},
				"sampleSize": map[string]interface{}{
					"type":        "number",
					"description": "Number of conversations to sample (default: 10)",
					"default":     10,
				},
			},
			Required: []string{"queueId", "startTime", "endTime"},
		},
	}, s.handleSampleConversations)

	// Search Voice Conversations
	s.server.AddTool(mcp.Tool{
		Name:        "search_voice_conversations",
		Description: "Searches for voice conversations within a time window",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"startTime": map[string]interface{}{
					"type":        "string",
					"description": "Start time (ISO 8601)",
				},
				"endTime": map[string]interface{}{
					"type":        "string",
					"description": "End time (ISO 8601)",
				},
				"phoneNumber": map[string]interface{}{
					"type":        "string",
					"description": "Optional phone number filter",
				},
				"pageSize": map[string]interface{}{
					"type":        "number",
					"description": "Page size (default: 25)",
					"default":     25,
				},
				"pageNumber": map[string]interface{}{
					"type":        "number",
					"description": "Page number (default: 1)",
					"default":     1,
				},
			},
			Required: []string{"startTime", "endTime"},
		},
	}, s.handleSearchVoiceConversations)

	// OAuth Clients
	s.server.AddTool(mcp.Tool{
		Name:        "oauth_clients",
		Description: "Retrieves a list of all OAuth clients",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleOAuthClients)
}

func (s *MCPServer) handleSearchQueues(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	name := getStringParam(arguments, "name", "")
	pageNumber := getIntParam(arguments, "pageNumber", 1)
	pageSize := getIntParam(arguments, "pageSize", 25)

	result, err := s.client.SearchQueues(name, pageNumber, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search queues: %v", err)), nil
	}

	return mcp.NewToolResultText(formatJSON(result)), nil
}

func (s *MCPServer) handleQueryQueueVolumes(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	queueIDs, ok := arguments["queueIds"].([]interface{})
	if !ok {
		return mcp.NewToolResultError("queueIds must be an array"), nil
	}

	var ids []string
	for _, id := range queueIDs {
		ids = append(ids, fmt.Sprintf("%v", id))
	}

	startTimeStr := getStringParam(arguments, "startTime", "")
	endTimeStr := getStringParam(arguments, "endTime", "")

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid startTime: %v", err)), nil
	}

	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid endTime: %v", err)), nil
	}

	result, err := s.client.QueryQueueVolumes(ids, startTime, endTime)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to query queue volumes: %v", err)), nil
	}

	return mcp.NewToolResultText(formatJSON(result)), nil
}

func (s *MCPServer) handleSampleConversations(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	queueID := getStringParam(arguments, "queueId", "")
	startTimeStr := getStringParam(arguments, "startTime", "")
	endTimeStr := getStringParam(arguments, "endTime", "")
	sampleSize := getIntParam(arguments, "sampleSize", 10)

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid startTime: %v", err)), nil
	}

	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid endTime: %v", err)), nil
	}

	result, err := s.client.SampleConversationsByQueue(queueID, startTime, endTime, sampleSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to sample conversations: %v", err)), nil
	}

	return mcp.NewToolResultText(formatJSON(map[string]interface{}{
		"conversationIds": result,
		"count":           len(result),
	})), nil
}

func (s *MCPServer) handleSearchVoiceConversations(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	startTimeStr := getStringParam(arguments, "startTime", "")
	endTimeStr := getStringParam(arguments, "endTime", "")
	phoneNumber := getStringParam(arguments, "phoneNumber", "")
	pageSize := getIntParam(arguments, "pageSize", 25)
	pageNumber := getIntParam(arguments, "pageNumber", 1)

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid startTime: %v", err)), nil
	}

	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid endTime: %v", err)), nil
	}

	result, err := s.client.SearchVoiceConversations(startTime, endTime, phoneNumber, pageSize, pageNumber)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search conversations: %v", err)), nil
	}

	return mcp.NewToolResultText(formatJSON(result)), nil
}

func (s *MCPServer) handleOAuthClients(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	result, err := s.client.ListOAuthClients()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list OAuth clients: %v", err)), nil
	}

	return mcp.NewToolResultText(formatJSON(map[string]interface{}{
		"clients": result,
		"count":   len(result),
	})), nil
}

// Helper functions
func getStringParam(args map[string]interface{}, key, defaultVal string) string {
	if args == nil {
		return defaultVal
	}
	if val, ok := args[key]; ok && val != nil {
		return fmt.Sprintf("%v", val)
	}
	return defaultVal
}

func getIntParam(args map[string]interface{}, key string, defaultVal int) int {
	if args == nil {
		return defaultVal
	}
	if val, ok := args[key]; ok && val != nil {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return defaultVal
}

func formatJSON(data interface{}) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", data)
	}
	return string(b)
}
