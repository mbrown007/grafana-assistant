package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sabio/alertmanager-mcp-go/pkg/alertmanager"
	"golang.org/x/time/rate"
)

// Pagination defaults and limits (configurable via environment variables)
var (
	DefaultSilencePage    = getEnvInt("ALERTMANAGER_DEFAULT_SILENCE_PAGE", 10)
	MaxSilencePage        = getEnvInt("ALERTMANAGER_MAX_SILENCE_PAGE", 50)
	DefaultAlertPage      = getEnvInt("ALERTMANAGER_DEFAULT_ALERT_PAGE", 10)
	MaxAlertPage          = getEnvInt("ALERTMANAGER_MAX_ALERT_PAGE", 25)
	DefaultAlertGroupPage = getEnvInt("ALERTMANAGER_DEFAULT_ALERT_GROUP_PAGE", 3)
	MaxAlertGroupPage     = getEnvInt("ALERTMANAGER_MAX_ALERT_GROUP_PAGE", 5)
)

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

// MCPServer wraps the alertmanager client and MCP server
type MCPServer struct {
	client  *alertmanager.Client
	server  *server.MCPServer
	limiter *rate.Limiter
}

// strPtr returns a pointer to a string
func strPtr(s string) *string {
	return &s
}

// NewMCPServer creates a new MCP server with alertmanager client
func NewMCPServer(client *alertmanager.Client) *MCPServer {
	return &MCPServer{
		client: client,
		server: server.NewMCPServer(
			"alertmanager-mcp-server",
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
	s.server.AddTool(s.getStatusTool(), s.handleGetStatus)
	s.server.AddTool(s.getAlertsTool(), s.handleGetAlerts)
	s.server.AddTool(s.getAlertGroupsTool(), s.handleGetAlertGroups)
	s.server.AddTool(s.getSilencesTool(), s.handleGetSilences)
	s.server.AddTool(s.postSilenceTool(), s.handlePostSilence)
	s.server.AddTool(s.deleteSilenceTool(), s.handleDeleteSilence)
	s.server.AddTool(s.postAlertsTool(), s.handlePostAlerts)
	s.server.AddTool(s.getReceiversTool(), s.handleGetReceivers)
}

// Note: Tenant ID extraction from request headers is not supported in the current
// mcp-go library. The tenant ID will be taken from the client's static configuration.

// getStatusTool returns the tool definition for get_status
func (s *MCPServer) getStatusTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_status",
		Description: "Get current status of an Alertmanager instance and its cluster",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
		},
	}
}

// handleGetStatus handles the get_status tool call
func (s *MCPServer) handleGetStatus(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	status, err := s.client.GetStatus("")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	data, _ := json.Marshal(status)
	return mcp.NewToolResultText(string(data)), nil
}

// getAlertsTool returns the tool definition for get_alerts
func (s *MCPServer) getAlertsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_alerts",
		Description: "Get a list of alerts currently in Alertmanager",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"filter": map[string]any{
					"type":        "string",
					"description": "Filtering query (e.g. alertname=~'.*CPU.*')",
				},
				"silenced": map[string]any{
					"type":        "boolean",
					"description": "If true, include silenced alerts",
				},
				"inhibited": map[string]any{
					"type":        "boolean",
					"description": "If true, include inhibited alerts",
				},
				"active": map[string]any{
					"type":        "boolean",
					"description": "If true, include active alerts",
				},
				"count": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Number of alerts to return per page (default: %d, max: %d)", DefaultAlertPage, MaxAlertPage),
					"default":     DefaultAlertPage,
					"minimum":     1,
					"maximum":     MaxAlertPage,
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Number of alerts to skip before returning results (default: 0). To paginate through all results, make multiple calls with increasing offset values (e.g., offset=0, offset=10, offset=20, etc.)",
					"default":     0,
					"minimum":     0,
				},
			},
		},
	}
}

// handleGetAlerts handles the get_alerts tool call
func (s *MCPServer) handleGetAlerts(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	// Parse arguments
	var args struct {
		Filter    string `json:"filter"`
		Silenced  *bool  `json:"silenced"`
		Inhibited *bool  `json:"inhibited"`
		Active    *bool  `json:"active"`
		Count     int    `json:"count"`
		Offset    int    `json:"offset"`
	}

	// Set defaults
	args.Count = DefaultAlertPage
	args.Offset = 0

	if arguments != nil {
		data, _ := json.Marshal(arguments)
		json.Unmarshal(data, &args)
	}

	// Validate pagination
	count, offset, err := ValidatePaginationParams(args.Count, args.Offset, MaxAlertPage)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	// Get all alerts
	filter := alertmanager.AlertsFilter{
		Filter:    args.Filter,
		Silenced:  args.Silenced,
		Inhibited: args.Inhibited,
		Active:    args.Active,
	}

	alerts, err := s.client.ListAlerts(filter, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	// Apply pagination
	result := PaginateResults(alerts, count, offset)
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// getAlertGroupsTool returns the tool definition for get_alert_groups
func (s *MCPServer) getAlertGroupsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_alert_groups",
		Description: "Get a list of alert groups",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"silenced": map[string]any{
					"type":        "boolean",
					"description": "If true, include silenced alerts",
				},
				"inhibited": map[string]any{
					"type":        "boolean",
					"description": "If true, include inhibited alerts",
				},
				"active": map[string]any{
					"type":        "boolean",
					"description": "If true, include active alerts",
				},
				"count": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Number of alert groups to return per page (default: %d, max: %d). Alert groups can be large as they contain all alerts within the group.", DefaultAlertGroupPage, MaxAlertGroupPage),
					"default":     DefaultAlertGroupPage,
					"minimum":     1,
					"maximum":     MaxAlertGroupPage,
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Number of alert groups to skip before returning results (default: 0). To paginate through all results, make multiple calls with increasing offset values (e.g., offset=0, offset=3, offset=6, etc.)",
					"default":     0,
					"minimum":     0,
				},
			},
		},
	}
}

// handleGetAlertGroups handles the get_alert_groups tool call
func (s *MCPServer) handleGetAlertGroups(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	// Parse arguments
	var args struct {
		Silenced  *bool `json:"silenced"`
		Inhibited *bool `json:"inhibited"`
		Active    *bool `json:"active"`
		Count     int   `json:"count"`
		Offset    int   `json:"offset"`
	}

	// Set defaults
	args.Count = DefaultAlertGroupPage
	args.Offset = 0

	if arguments != nil {
		data, _ := json.Marshal(arguments)
		json.Unmarshal(data, &args)
	}

	// Validate pagination
	count, offset, err := ValidatePaginationParams(args.Count, args.Offset, MaxAlertGroupPage)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	// Get all alert groups
	filter := alertmanager.AlertGroupsFilter{
		Silenced:  args.Silenced,
		Inhibited: args.Inhibited,
		Active:    args.Active,
	}

	groups, err := s.client.GetAlertGroups(filter, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	// Apply pagination
	result := PaginateResults(groups, count, offset)
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// getSilencesTool returns the tool definition for get_silences
func (s *MCPServer) getSilencesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_silences",
		Description: "Get list of all silences",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"filter": map[string]any{
					"type":        "string",
					"description": "Filtering query (e.g. alertname=~'.*CPU.*')",
				},
				"count": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Number of silences to return per page (default: %d, max: %d)", DefaultSilencePage, MaxSilencePage),
					"default":     DefaultSilencePage,
					"minimum":     1,
					"maximum":     MaxSilencePage,
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Number of silences to skip before returning results (default: 0). To paginate through all results, make multiple calls with increasing offset values (e.g., offset=0, offset=10, offset=20, etc.)",
					"default":     0,
					"minimum":     0,
				},
			},
		},
	}
}

// handleGetSilences handles the get_silences tool call
func (s *MCPServer) handleGetSilences(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	// Parse arguments
	var args struct {
		Filter string `json:"filter"`
		Count  int    `json:"count"`
		Offset int    `json:"offset"`
	}

	// Set defaults
	args.Count = DefaultSilencePage
	args.Offset = 0

	if arguments != nil {
		data, _ := json.Marshal(arguments)
		json.Unmarshal(data, &args)
	}

	// Validate pagination
	count, offset, err := ValidatePaginationParams(args.Count, args.Offset, MaxSilencePage)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	// Get all silences
	silences, err := s.client.ListSilences(args.Filter, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	// Apply pagination
	result := PaginateResults(silences, count, offset)
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// postSilenceTool returns the tool definition for post_silence
func (s *MCPServer) postSilenceTool() mcp.Tool {
	return mcp.Tool{
		Name:        "post_silence",
		Description: "Post a new silence or update an existing one",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"silence": map[string]any{
					"type":        "object",
					"description": "Silence object with matchers, startsAt, endsAt, createdBy, and comment fields",
					"required":    []string{"matchers", "startsAt", "endsAt", "createdBy", "comment"},
					"properties": map[string]any{
						"matchers": map[string]any{
							"type":        "array",
							"description": "List of matchers to match alerts to silence",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{
										"type": "string",
									},
									"value": map[string]any{
										"type": "string",
									},
									"isRegex": map[string]any{
										"type": "boolean",
									},
									"isEqual": map[string]any{
										"type": "boolean",
									},
								},
							},
						},
						"startsAt": map[string]any{
							"type":        "string",
							"description": "Start time of the silence (RFC3339 format)",
						},
						"endsAt": map[string]any{
							"type":        "string",
							"description": "End time of the silence (RFC3339 format)",
						},
						"createdBy": map[string]any{
							"type":        "string",
							"description": "Name of the user creating the silence",
						},
						"comment": map[string]any{
							"type":        "string",
							"description": "Comment for the silence",
						},
					},
				},
			},
			Required: []string{"silence"},
		},
	}
}

// handlePostSilence handles the post_silence tool call
func (s *MCPServer) handlePostSilence(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	// Parse arguments
	var args struct {
		Silence alertmanager.Silence `json:"silence"`
	}

	if arguments != nil {
		data, _ := json.Marshal(arguments)
		if err := json.Unmarshal(data, &args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error parsing silence: %v", err)), nil
		}
	}

	result, err := s.client.CreateSilence(args.Silence, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// deleteSilenceTool returns the tool definition for delete_silence
func (s *MCPServer) deleteSilenceTool() mcp.Tool {
	return mcp.Tool{
		Name:        "delete_silence",
		Description: "Delete a silence by its ID",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"silence_id": map[string]any{
					"type":        "string",
					"description": "The ID of the silence to be deleted",
				},
			},
			Required: []string{"silence_id"},
		},
	}
}

// handleDeleteSilence handles the delete_silence tool call
func (s *MCPServer) handleDeleteSilence(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	// Parse arguments
	var args struct {
		SilenceID string `json:"silence_id"`
	}

	if arguments != nil {
		data, _ := json.Marshal(arguments)
		json.Unmarshal(data, &args)
	}

	if args.SilenceID == "" {
		return mcp.NewToolResultError("error: silence_id is required"), nil
	}

	err := s.client.DeleteSilence(args.SilenceID, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(`{"message": "Silence %s deleted successfully"}`, args.SilenceID)), nil
}

// postAlertsTool returns the tool definition for post_alerts
func (s *MCPServer) postAlertsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "post_alerts",
		Description: "Create new alerts",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"alerts": map[string]any{
					"type":        "array",
					"description": "List of alert objects with startsAt, endsAt, annotations, and labels",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"startsAt": map[string]any{
								"type":        "string",
								"description": "Start time of the alert (RFC3339 format)",
							},
							"endsAt": map[string]any{
								"type":        "string",
								"description": "End time of the alert (RFC3339 format)",
							},
							"annotations": map[string]any{
								"type":        "object",
								"description": "Alert annotations (key-value pairs)",
							},
							"labels": map[string]any{
								"type":        "object",
								"description": "Alert labels (key-value pairs)",
							},
						},
					},
				},
			},
			Required: []string{"alerts"},
		},
	}
}

// handlePostAlerts handles the post_alerts tool call
func (s *MCPServer) handlePostAlerts(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	// Parse arguments
	var args struct {
		Alerts []alertmanager.Alert `json:"alerts"`
	}

	if arguments != nil {
		data, _ := json.Marshal(arguments)
		if err := json.Unmarshal(data, &args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error parsing alerts: %v", err)), nil
		}
	}

	if len(args.Alerts) == 0 {
		return mcp.NewToolResultError("error: at least one alert is required"), nil
	}

	err := s.client.CreateAlert(args.Alerts, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	return mcp.NewToolResultText(`{"message": "Alerts created successfully"}`), nil
}

// getReceiversTool returns the tool definition for get_receivers
func (s *MCPServer) getReceiversTool() mcp.Tool {
	return mcp.Tool{
		Name:        "get_receivers",
		Description: "Get list of all receivers (name of notification integrations)",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
		},
	}
}

// handleGetReceivers handles the get_receivers tool call
func (s *MCPServer) handleGetReceivers(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if result := s.enforceRateLimit(); result != nil {
		return result, nil
	}
	receivers, err := s.client.GetReceivers("")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
	}

	data, _ := json.Marshal(receivers)
	return mcp.NewToolResultText(string(data)), nil
}
