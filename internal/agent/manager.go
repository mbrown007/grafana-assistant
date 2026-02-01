package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	openai "github.com/sashabaranov/go-openai"

	"github.com/marcusz/monitoring-assistant/internal/api"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
	"github.com/marcusz/monitoring-assistant/internal/llm"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
	"github.com/marcusz/monitoring-assistant/internal/storage"
)

const maxToolIterations = 5

// Manager orchestrates the LLM agent loop with tool calling and memory.
type Manager struct {
	llm      *llm.Client
	mcp      []*mcp.Client
	enricher *appcontext.Enricher
	store    storage.Store
	tools    []mcp.Tool
}

// NewManager creates an agent manager.
func NewManager(llmClient *llm.Client, mcpClients []*mcp.Client, enricher *appcontext.Enricher, store storage.Store) *Manager {
	return &Manager{
		llm:      llmClient,
		mcp:      mcpClients,
		enricher: enricher,
		store:    store,
	}
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
		if err := m.store.CreateSession(ctx, sess); err != nil {
			slog.ErrorContext(ctx, "failed to create session", "error", err)
		}
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
			mem.LoadHistory(msgs)
		}
	}

	streamFn(api.StreamChunk{Type: "start", SessionID: sess.ID})

	// 4. Add user message (sanitize control characters).
	cleanMessage := sanitizeInput(req.Message)
	mem.Add(openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: cleanMessage,
	})

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
	}

	// 5. Prepare OpenAI tools.
	openaiTools := MCPToolsToOpenAI(m.tools)

	// 6. Agent loop (tool calling iterations).
	var finalContent string
	for iteration := 0; iteration <= maxToolIterations; iteration++ {
		messages := mem.Messages()

		if iteration == maxToolIterations {
			// Last iteration: stream directly to the client.
			ch, err := m.llm.StreamChat(ctx, messages, nil)
			if err != nil {
				streamFn(api.StreamChunk{Type: "error", Message: fmt.Sprintf("LLM error: %v", err)})
				return
			}
			finalContent = m.streamToClient(ch, streamFn)
			break
		}

		// Use non-streaming call for tool loop iterations to check for tool calls.
		resp, err := m.llm.Chat(ctx, messages, openaiTools)
		if err != nil {
			streamFn(api.StreamChunk{Type: "error", Message: fmt.Sprintf("LLM error: %v", err)})
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
					streamFn(api.StreamChunk{Type: "error", Message: fmt.Sprintf("LLM error: %v", err)})
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

			// Stream tool invocation to client.
			streamFn(api.StreamChunk{
				Type:      "tool",
				Tool:      tc.Function.Name,
				ToolID:    tc.ID,
				Arguments: args,
			})

			// Execute tool.
			result, err := RouteToolCall(ctx, tc.Function.Name, args, m.mcp)
			if err != nil {
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
			streamFn(api.StreamChunk{Type: "error", Message: chunk.Message})
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
