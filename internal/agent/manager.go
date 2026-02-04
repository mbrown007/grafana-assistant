package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	openai "github.com/sashabaranov/go-openai"

	"github.com/marcusz/monitoring-assistant/internal/api"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/internal/dashboard"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
	"github.com/marcusz/monitoring-assistant/internal/llm"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
	"github.com/marcusz/monitoring-assistant/internal/metrics"
	"github.com/marcusz/monitoring-assistant/internal/storage"
	"github.com/marcusz/monitoring-assistant/pkg/kb"
)

const maxToolIterations = 5
const llmUserErrorMessage = "I'm having issues right now. Please try again, and if the issue persists report it to the monitoring team."

// Manager orchestrates the LLM agent loop with tool calling and memory.
type Manager struct {
	llm               *llm.Client
	mcp               []mcp.Client
	enricher          *appcontext.Enricher
	store             storage.Store
	scratchpads       *dashboard.Manager
	tools             []mcp.Tool
	kbPath            string
	kbMaxSections     int
	kbMaxSectionChars int
	kbOnce            sync.Once
	kbIndex           *kb.Index
	kbErr             error
	kbPlatformOnce    sync.Once
	kbPlatformIndex   *kb.Index
	kbPlatformErr     error
	kbDashboardMap    map[string]string

	// Hybrid KB: vector search fields.
	kbStructuredPath   string
	kbVectorDBPath     string
	kbVectorMaxResults int
	kbVectorOnce       sync.Once
	kbVectorIndex      *kb.VectorIndex
	kbEmbedder         *kb.Embedder
}

// ManagerConfig holds configuration for the agent Manager.
type ManagerConfig struct {
	KBPath             string
	KBMaxSections      int
	KBMaxSectionChars  int
	KBStructuredPath   string
	KBVectorPath       string
	KBVectorDBPath     string
	KBEmbeddingModel   string
	KBVectorMaxResults int
	KBDashboardMap     map[string]string
	OpenAIAPIKey       string
}

// NewManager creates an agent manager.
func NewManager(llmClient *llm.Client, mcpClients []mcp.Client, enricher *appcontext.Enricher, store storage.Store, scratchpads *dashboard.Manager, cfg ManagerConfig) *Manager {
	if cfg.KBPath == "" {
		cfg.KBPath = "KB"
	}
	if cfg.KBMaxSections <= 0 {
		cfg.KBMaxSections = 2
	}
	if cfg.KBMaxSectionChars <= 0 {
		cfg.KBMaxSectionChars = 2000
	}
	if cfg.KBVectorMaxResults <= 0 {
		cfg.KBVectorMaxResults = 2
	}

	m := &Manager{
		llm:                llmClient,
		mcp:                mcpClients,
		enricher:           enricher,
		store:              store,
		scratchpads:        scratchpads,
		kbPath:             cfg.KBPath,
		kbMaxSections:      cfg.KBMaxSections,
		kbMaxSectionChars:  cfg.KBMaxSectionChars,
		kbStructuredPath:   cfg.KBStructuredPath,
		kbVectorDBPath:     cfg.KBVectorDBPath,
		kbVectorMaxResults: cfg.KBVectorMaxResults,
		kbDashboardMap:     normalizeDashboardMap(cfg.KBDashboardMap),
	}

	// Create embedder if API key and vector path are configured.
	if cfg.OpenAIAPIKey != "" && cfg.KBVectorPath != "" {
		model := cfg.KBEmbeddingModel
		if model == "" {
			model = "text-embedding-3-small"
		}
		m.kbEmbedder = kb.NewEmbedder(cfg.OpenAIAPIKey, model)
		slog.Info("KB vector search enabled", "vector_db", cfg.KBVectorDBPath, "model", model)
	} else {
		slog.Info("KB vector search disabled (no API key or vector path)")
	}

	return m
}

// DiscoverTools queries all MCP clients for available tools and caches them.
func (m *Manager) DiscoverTools(ctx context.Context) error {
	m.tools = nil
	for _, c := range m.mcp {
		tools, err := c.DiscoverTools(ctx)
		if err != nil {
			slog.WarnContext(ctx, "failed to discover tools from MCP server", "error", err)
			continue
		}
		m.tools = append(m.tools, tools...)
	}
	slog.InfoContext(ctx, "discovered MCP tools", "count", len(m.tools))
	return nil
}

// HandleChat runs the agent loop: system prompt → memory → LLM → tool loop → stream response.
func (m *Manager) HandleChat(ctx context.Context, user *grafana.User, req api.ChatRequest, streamFn func(api.StreamChunk)) {
	if user == nil {
		streamFn(api.StreamChunk{Type: "error", Message: "unauthorized"})
		return
	}

	// 1. Enrich dashboard context if UID is provided.
	var dashCtx *appcontext.DashboardSummary
	if req.DashboardContext != nil && req.DashboardContext.UID != "" {
		summary, err := m.enricher.GetDashboardSummary(ctx, req.DashboardContext.UID)
		if err != nil {
			slog.WarnContext(ctx, "failed to enrich dashboard context", "uid", req.DashboardContext.UID, "error", err)
		} else {
			dashCtx = summary
		}
	}

	// 2. Build system prompt.
	systemPrompt := SystemPrompt(dashCtx, req.DashboardContext, m.tools)

	// 3. Load or create session, build memory.
	mem := NewMemory(systemPrompt, 0)

	sessionID := req.SessionID
	dashboardUID := ""
	if req.DashboardContext != nil {
		dashboardUID = req.DashboardContext.UID
	}

	var sess *storage.Session
	isNewSession := false
	prevDashboardUID := ""
	if sessionID != "" {
		existing, err := m.store.GetSession(ctx, sessionID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get session", "error", err)
		}
		if existing != nil {
			if existing.UserID != user.ID || existing.OrgID != user.OrgID {
				streamFn(api.StreamChunk{Type: "error", Message: "invalid session"})
				return
			}
			sess = existing
			prevDashboardUID = existing.DashboardUID
		}
	}

	if sess == nil {
		sessionID = newSessionID()
		now := time.Now()
		sess = &storage.Session{
			ID:           sessionID,
			UserID:       user.ID,
			OrgID:        user.OrgID,
			DashboardUID: dashboardUID,
			Title:        truncate(req.Message, 100),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		metrics.ChatsStartedTotal.Inc()
		if err := m.store.CreateSession(ctx, sess); err != nil {
			slog.ErrorContext(ctx, "failed to create session", "error", err)
		}
		isNewSession = true
	} else {
		// Refresh session metadata if needed.
		if sess.Title == "" || (dashboardUID != "" && sess.DashboardUID != dashboardUID) {
			title := sess.Title
			if title == "" {
				title = truncate(req.Message, 100)
			}
			if err := m.store.UpdateSessionMeta(ctx, sess.ID, title, dashboardUID, time.Now()); err != nil {
				slog.ErrorContext(ctx, "failed to update session", "error", err)
			} else {
				sess.Title = title
				sess.DashboardUID = dashboardUID
			}
		}

		// Load existing conversation history.
		msgs, err := m.store.GetMessages(ctx, sess.ID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to load messages", "error", err)
		} else {
			if len(msgs) == 0 {
				isNewSession = true
			}
			mem.LoadHistory(msgs)
		}
	}

	streamFn(api.StreamChunk{Type: "start", SessionID: sess.ID})

	// Log system prompt metadata for audit trail.
	if sess != nil {
		now := time.Now()
		promptMeta := map[string]any{
			"prompt_length":         len(systemPrompt),
			"has_dashboard_context": dashCtx != nil,
			"tool_count":            len(m.tools),
		}
		if dashCtx != nil {
			promptMeta["dashboard_title"] = dashCtx.Title
			promptMeta["panel_count"] = len(dashCtx.Panels)
		}
		_ = m.store.AddAuditEntry(ctx, &storage.AuditEntry{
			ID:           fmt.Sprintf("%s-%d-audit-sysprompt", sess.ID, now.UnixMilli()),
			SessionID:    sess.ID,
			UserID:       user.ID,
			OrgID:        user.OrgID,
			DashboardUID: sess.DashboardUID,
			EventType:    "system_prompt",
			Request:      marshalAuditValue(promptMeta),
			CreatedAt:    now,
		})
	}

	// 4. Add user message (sanitize control characters).
	cleanMessage := sanitizeInput(req.Message)
	dashboardChanged := req.DashboardContext != nil && req.DashboardContext.UID != "" && req.DashboardContext.UID != prevDashboardUID
	shouldInjectKB := isNewSession || dashboardChanged
	var kbContext string
	var kbEvidence *api.KBSearchEvidence
	var vectorEvidence *api.VectorSearchEvidence
	if shouldInjectKB {
		kbContext, kbEvidence, vectorEvidence = m.buildKBContext(cleanMessage, dashCtx, req.DashboardContext, true)
		if kbEvidence != nil || vectorEvidence != nil {
			if kbEvidence != nil {
				kbEvidence.Query = req.Message
			}
			if vectorEvidence != nil {
				vectorEvidence.Query = req.Message
			}
			streamFn(api.StreamChunk{
				Type: "evidence",
				Evidence: &api.EvidencePayload{
					KBSearch:     kbEvidence,
					VectorSearch: vectorEvidence,
				},
			})
		}
	}
	if kbContext != "" {
		cleanMessage = cleanMessage + "\n\n[KB Context]\n" + kbContext
	}
	mem.Add(openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: cleanMessage,
	})

	// Log injected context as audit event.
	if sess != nil && (kbContext != "" || dashCtx != nil) {
		now := time.Now()
		injection := map[string]any{}
		if kbContext != "" {
			injection["kb_context"] = kbContext
		}
		if dashCtx != nil {
			injection["dashboard_context"] = dashCtx
		}
		_ = m.store.AddAuditEntry(ctx, &storage.AuditEntry{
			ID:           fmt.Sprintf("%s-%d-audit-context", sess.ID, now.UnixMilli()),
			SessionID:    sess.ID,
			UserID:       user.ID,
			OrgID:        user.OrgID,
			DashboardUID: sess.DashboardUID,
			EventType:    "context_injection",
			Request:      marshalAuditValue(injection),
			CreatedAt:    now,
		})
		slog.InfoContext(ctx, "Context injected into prompt",
			"event", "context_injection",
			"session_id", sess.ID,
			"user_id", user.ID,
			"org_id", user.OrgID,
			"dashboard_uid", sess.DashboardUID,
			"has_kb_context", kbContext != "",
			"has_dashboard_context", dashCtx != nil,
		)
	}

	// Persist user message.
	if sess != nil {
		createdAt := time.Now()
		_ = m.store.AddMessage(ctx, &storage.Message{
			ID:        fmt.Sprintf("%s-%d-user", sess.ID, createdAt.UnixMilli()),
			SessionID: sess.ID,
			Role:      "user",
			Content:   req.Message,
			CreatedAt: createdAt,
		})
		_ = m.store.AddAuditEntry(ctx, &storage.AuditEntry{
			ID:           fmt.Sprintf("%s-%d-audit-user", sess.ID, createdAt.UnixMilli()),
			SessionID:    sess.ID,
			UserID:       user.ID,
			OrgID:        user.OrgID,
			DashboardUID: sess.DashboardUID,
			EventType:    "user_message",
			Request:      req.Message,
			CreatedAt:    createdAt,
		})
		slog.InfoContext(ctx, "User message received",
			"event", "user_message",
			"session_id", sess.ID,
			"user_id", user.ID,
			"org_id", user.OrgID,
			"dashboard_uid", sess.DashboardUID,
			"message", req.Message,
		)
	}

	// 5. Prepare OpenAI tools.
	openaiTools := append(MCPToolsToOpenAI(m.tools), InternalTools()...)

	// 6. Agent loop (tool calling iterations).
	var finalContent string
	for iteration := 0; iteration <= maxToolIterations; iteration++ {
		messages := mem.Messages()

		if iteration == maxToolIterations {
			// Last iteration: stream directly to the client.
			ch, err := m.llm.StreamChat(ctx, messages, nil)
			if err != nil {
				slog.ErrorContext(ctx, "LLM stream error",
					"event", "error",
					"source", "llm",
					"session_id", sess.ID,
					"error", err,
				)
				streamFn(api.StreamChunk{Type: "error", Message: userFacingLLMError(err)})
				return
			}
			finalContent = m.streamToClient(ch, streamFn)
			break
		}

		// Use non-streaming call for tool loop iterations to check for tool calls.
		resp, err := m.llm.Chat(ctx, messages, openaiTools)
		if err != nil {
			slog.ErrorContext(ctx, "LLM error",
				"event", "error",
				"source", "llm",
				"session_id", sess.ID,
				"error", err,
			)
			streamFn(api.StreamChunk{Type: "error", Message: userFacingLLMError(err)})
			return
		}

		if len(resp.ToolCalls) == 0 {
			// No tool calls — stream the final response.
			if resp.Content != "" {
				// We already have the full content, send it as tokens.
				finalContent = resp.Content
				streamFn(api.StreamChunk{Type: "token", Message: resp.Content})
				streamFn(api.StreamChunk{Type: "complete", Message: resp.Content})
				streamFn(api.StreamChunk{Type: "done"})
			} else {
				// Re-stream for a proper token-by-token experience.
				ch, err := m.llm.StreamChat(ctx, messages, nil)
				if err != nil {
					slog.ErrorContext(ctx, "LLM stream error",
						"event", "error",
						"source", "llm",
						"session_id", sess.ID,
						"error", err,
					)
					streamFn(api.StreamChunk{Type: "error", Message: userFacingLLMError(err)})
					return
				}
				finalContent = m.streamToClient(ch, streamFn)
			}
			break
		}

		// Process tool calls.
		mem.Add(openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{"raw": tc.Function.Arguments}
			}
			metrics.ToolCallsTotal.WithLabelValues(tc.Function.Name).Inc()

			// Stream tool invocation to client.
			streamFn(api.StreamChunk{
				Type:      "tool",
				Tool:      tc.Function.Name,
				ToolID:    tc.ID,
				Arguments: args,
			})
			slog.InfoContext(ctx, "Executing tool call",
				"event", "tool_call",
				"session_id", sess.ID,
				"user_id", user.ID,
				"org_id", user.OrgID,
				"dashboard_uid", sess.DashboardUID,
				"tool_name", tc.Function.Name,
				"params", args,
			)

			// Execute tool.
			result, err := m.handleInternalTool(ctx, tc.Function.Name, args, user, sess, req.DashboardContext)
			if err == nil && result == nil {
				result, err = RouteToolCall(ctx, tc.Function.Name, args, m.mcp)
			}
			if err != nil {
				metrics.ErrorsTotalBySource.WithLabelValues("tool_call").Inc()
				slog.ErrorContext(ctx, "Tool call failed",
					"event", "error",
					"source", "tool_call",
					"session_id", sess.ID,
					"tool_name", tc.Function.Name,
					"error", err,
				)
				slog.WarnContext(ctx, "tool call failed", "tool", tc.Function.Name, "error", err)
				result = fmt.Sprintf("Error: %v", err)
			}

			resultStr := mcp.FormatToolResult(result)

			// Stream tool result to client.
			streamFn(api.StreamChunk{
				Type:   "tool",
				Tool:   tc.Function.Name,
				ToolID: tc.ID,
				Result: result,
			})

			if sess != nil {
				now := time.Now()
				_ = m.store.AddAuditEntry(ctx, &storage.AuditEntry{
					ID:           fmt.Sprintf("%s-%d-audit-tool", sess.ID, now.UnixMilli()),
					SessionID:    sess.ID,
					UserID:       user.ID,
					OrgID:        user.OrgID,
					DashboardUID: sess.DashboardUID,
					EventType:    "tool_call",
					ToolName:     tc.Function.Name,
					Request:      marshalAuditValue(args),
					Response:     marshalAuditValue(result),
					CreatedAt:    now,
				})
			}

			// Add tool result to memory with delimiters.
			mem.Add(openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    wrapToolResult(resultStr),
				ToolCallID: tc.ID,
			})
		}
	}

	// 7. Persist assistant response.
	if sess != nil && finalContent != "" {
		createdAt := time.Now()
		_ = m.store.AddMessage(ctx, &storage.Message{
			ID:        fmt.Sprintf("%s-%d-assistant", sess.ID, createdAt.UnixMilli()),
			SessionID: sess.ID,
			Role:      "assistant",
			Content:   finalContent,
			CreatedAt: createdAt,
		})
		_ = m.store.AddAuditEntry(ctx, &storage.AuditEntry{
			ID:           fmt.Sprintf("%s-%d-audit-assistant", sess.ID, createdAt.UnixMilli()),
			SessionID:    sess.ID,
			UserID:       user.ID,
			OrgID:        user.OrgID,
			DashboardUID: sess.DashboardUID,
			EventType:    "assistant_response",
			Response:     finalContent,
			CreatedAt:    createdAt,
		})
		slog.InfoContext(ctx, "LLM response sent",
			"event", "assistant_response",
			"session_id", sess.ID,
			"user_id", user.ID,
			"org_id", user.OrgID,
			"dashboard_uid", sess.DashboardUID,
			"message", finalContent,
		)
	}
}

// streamToClient consumes an LLM stream channel and forwards chunks to the SSE callback.
// Returns the full accumulated content.
func (m *Manager) streamToClient(ch <-chan llm.StreamChunk, streamFn func(api.StreamChunk)) string {
	var content string
	for chunk := range ch {
		switch chunk.Type {
		case "token":
			content += chunk.Message
			streamFn(api.StreamChunk{Type: "token", Message: chunk.Message})
		case "error":
			streamFn(api.StreamChunk{Type: "error", Message: userFacingLLMError(errors.New(chunk.Message))})
		case "complete":
			if content == "" {
				content = chunk.Message
			}
			streamFn(api.StreamChunk{Type: "complete", Message: chunk.Message})
		case "done":
			streamFn(api.StreamChunk{Type: "done"})
		}
	}
	return content
}

// sanitizeInput strips control characters (except newline, tab, carriage return)
// from user input before it enters the LLM conversation.
func sanitizeInput(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func userFacingLLMError(err error) string {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatusCode == 429 {
		return llmUserErrorMessage
	}

	if strings.Contains(err.Error(), "status code: 429") {
		return llmUserErrorMessage
	}

	return llmUserErrorMessage
}

// wrapToolResult wraps tool output with delimiters to help the LLM distinguish
// tool data from instructions.
func wrapToolResult(result string) string {
	return "[TOOL_RESULT_START]\n" + result + "\n[TOOL_RESULT_END]"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func normalizeDashboardMap(mapping map[string]string) map[string]string {
	if len(mapping) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(mapping))
	for k, v := range mapping {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" || v == "" {
			continue
		}
		normalized[key] = v
	}
	return normalized
}

func newSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

func marshalAuditValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

func (m *Manager) handleInternalTool(ctx context.Context, name string, args map[string]any, user *grafana.User, sess *storage.Session, reqCtx *api.DashboardContext) (any, error) {
	switch name {
	case "scratchpad__upsert_panel":
		if m.scratchpads == nil {
			return nil, fmt.Errorf("scratchpad manager not configured")
		}
		if user == nil || sess == nil {
			return nil, fmt.Errorf("missing user/session context for scratchpad")
		}

		query, _ := args["query"].(string)
		title, _ := args["title"].(string)
		description, _ := args["description"].(string)
		panelType, _ := args["panelType"].(string)
		if query == "" || title == "" {
			return nil, fmt.Errorf("query and title are required")
		}

		var datasource map[string]any
		if raw, ok := args["datasource"].(map[string]any); ok {
			datasource = raw
		}

		var timeRange map[string]string
		if raw, ok := args["timeRange"].(map[string]any); ok {
			timeRange = make(map[string]string)
			for k, v := range raw {
				if s, ok := v.(string); ok {
					timeRange[k] = s
				}
			}
		}

		uid := sess.ScratchpadUID
		panelID := 1
		url := ""
		if uid == "" {
			var err error
			uid, panelID, url, err = m.scratchpads.GetOrCreateScratchpad(ctx, user, sess.ID)
			if err != nil {
				metrics.ErrorsTotalBySource.WithLabelValues("grafana_api").Inc()
				slog.ErrorContext(ctx, "Failed to get or create scratchpad",
					"event", "error",
					"source", "grafana_api",
					"session_id", sess.ID,
					"user_id", user.ID,
					"org_id", user.OrgID,
					"error", err,
				)
				return nil, err
			}
			if err := m.store.UpdateSessionScratchpad(ctx, sess.ID, uid, time.Now()); err != nil {
				slog.WarnContext(ctx, "failed to persist scratchpad uid", "error", err)
			} else {
				sess.ScratchpadUID = uid
			}
		}

		if err := m.scratchpads.UpdatePanel(ctx, uid, panelID, query, title, description, panelType, datasource, timeRange); err != nil {
			metrics.ErrorsTotalBySource.WithLabelValues("grafana_api").Inc()
			slog.ErrorContext(ctx, "Failed to update scratchpad panel",
				"event", "error",
				"source", "grafana_api",
				"session_id", sess.ID,
				"user_id", user.ID,
				"org_id", user.OrgID,
				"error", err,
			)
			return nil, err
		}
		if url == "" {
			url = "/grafana/d/" + uid
		}

		return map[string]any{
			"dashboardUid": uid,
			"panelId":      panelID,
			"url":          url,
		}, nil
	case "explore__open":
		url, err := buildExploreURL(args, reqCtx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"url": url}, nil
	default:
		return nil, nil
	}
}
