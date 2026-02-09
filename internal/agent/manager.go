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

	"github.com/brownster/grafana-assistant/internal/api"
	appcontext "github.com/brownster/grafana-assistant/internal/context"
	"github.com/brownster/grafana-assistant/internal/dashboard"
	"github.com/brownster/grafana-assistant/internal/grafana"
	"github.com/brownster/grafana-assistant/internal/llm"
	"github.com/brownster/grafana-assistant/internal/mcp"
	"github.com/brownster/grafana-assistant/internal/metrics"
	"github.com/brownster/grafana-assistant/internal/storage"
	"github.com/brownster/grafana-assistant/pkg/kb"
)

const maxToolIterations = 5
const llmUserErrorMessage = "I'm having issues right now. Please try again, and if the issue persists report it to the monitoring team."

// Manager orchestrates the LLM agent loop with tool calling and memory.
type Manager struct {
	llm                      *llm.Client
	mcp                      []mcp.Client
	enricher                 *appcontext.Enricher
	store                    storage.Store
	scratchpads              *dashboard.Manager
	tools                    []mcp.Tool
	toolClientMu             sync.RWMutex
	toolClientByName         map[string]mcp.Client
	investigationToolTimeout time.Duration
	kbPath                   string
	kbMaxSections            int
	kbMaxSectionChars        int
	kbOnce                   sync.Once
	kbIndex                  *kb.Index
	kbErr                    error
	kbPlatformOnce           sync.Once
	kbPlatformIndex          *kb.Index
	kbPlatformErr            error
	kbDashboardMap           map[string]string

	// Hybrid KB: vector search fields.
	kbStructuredPath   string
	kbVectorDBPath     string
	kbVectorMaxResults int
	kbVectorOnce       sync.Once
	kbVectorIndex      *kb.VectorIndex
	kbEmbedder         *kb.Embedder

	routingMode           bool
	compositeToolMode     bool
	judgeGateMode         bool
	evidenceRedactionMode bool
	flagsInitialized      bool
	promptProfile         PromptProfile
	requestBudget         RequestBudget
}

// ManagerConfig holds configuration for the agent Manager.
type ManagerConfig struct {
	KBPath                   string
	KBMaxSections            int
	KBMaxSectionChars        int
	KBStructuredPath         string
	KBVectorPath             string
	KBVectorDBPath           string
	KBEmbeddingModel         string
	KBVectorMaxResults       int
	KBDashboardMap           map[string]string
	OpenAIAPIKey             string
	InvestigationToolTimeout time.Duration
	ModelPromptProfile       string
	RequestBudget            RequestBudget

	RoutingMode           *bool
	CompositeToolMode     *bool
	JudgeGateMode         *bool
	EvidenceRedactionMode *bool
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
		llm:                      llmClient,
		mcp:                      mcpClients,
		enricher:                 enricher,
		store:                    store,
		scratchpads:              scratchpads,
		kbPath:                   cfg.KBPath,
		kbMaxSections:            cfg.KBMaxSections,
		kbMaxSectionChars:        cfg.KBMaxSectionChars,
		kbStructuredPath:         cfg.KBStructuredPath,
		kbVectorDBPath:           cfg.KBVectorDBPath,
		kbVectorMaxResults:       cfg.KBVectorMaxResults,
		kbDashboardMap:           normalizeDashboardMap(cfg.KBDashboardMap),
		toolClientByName:         map[string]mcp.Client{},
		investigationToolTimeout: cfg.InvestigationToolTimeout,
		routingMode:              boolOrDefault(cfg.RoutingMode, true),
		compositeToolMode:        boolOrDefault(cfg.CompositeToolMode, true),
		judgeGateMode:            boolOrDefault(cfg.JudgeGateMode, true),
		evidenceRedactionMode:    boolOrDefault(cfg.EvidenceRedactionMode, true),
		flagsInitialized:         true,
		promptProfile:            ResolvePromptProfile(cfg.ModelPromptProfile),
		requestBudget:            normalizeRequestBudget(cfg.RequestBudget),
	}

	if m.investigationToolTimeout <= 0 {
		m.investigationToolTimeout = investigationToolTimeout
	}

	slog.Info("agent feature flags resolved",
		"routing_mode", m.routingModeEnabled(),
		"composite_tool_mode", m.compositeToolModeEnabled(),
		"judge_gate_mode", m.judgeGateModeEnabled(),
		"evidence_redaction_mode", m.evidenceRedactionModeEnabled(),
		"model_prompt_profile", m.promptProfile.Name,
		"budget_max_prompt_tokens", m.requestBudget.MaxPromptTokens,
		"budget_max_completion_tokens", m.requestBudget.MaxCompletionTokens,
		"budget_max_tool_iterations", m.requestBudget.MaxToolIterations,
		"budget_max_tool_calls", m.requestBudget.MaxToolCalls,
		"budget_max_estimated_cost_usd", m.requestBudget.MaxEstimatedCostUSD,
	)

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
	discoveredTools := make([]mcp.Tool, 0)
	toolClientByName := map[string]mcp.Client{}
	for _, c := range m.mcp {
		tools, err := c.DiscoverTools(ctx)
		if err != nil {
			slog.WarnContext(ctx, "failed to discover tools from MCP server", "error", err)
			continue
		}
		discoveredTools = append(discoveredTools, tools...)
		for _, t := range tools {
			toolClientByName[t.Name] = c
		}
	}
	m.tools = discoveredTools
	m.toolClientMu.Lock()
	m.toolClientByName = toolClientByName
	m.toolClientMu.Unlock()
	slog.InfoContext(ctx, "discovered MCP tools", "count", len(m.tools))
	return nil
}

// HandleChat runs the agent loop: system prompt → memory → LLM → tool loop → stream response.
func (m *Manager) HandleChat(ctx context.Context, user *grafana.User, req api.ChatRequest, streamFn func(api.StreamChunk)) {
	if user == nil {
		streamFn(api.StreamChunk{Type: "error", Message: "unauthorized"})
		return
	}

	intent := ClassifyIntent(req.Message, req.DashboardContext)
	slog.InfoContext(ctx, "classified request intent",
		"event", "intent_classification",
		"intent_label", intent.Label,
		"intent_confidence", intent.Confidence,
		"intent_rationale", intent.Rationale,
	)

	intentScopedMCPTools := m.tools
	if m.routingModeEnabled() {
		intentScopedMCPTools = filterToolsForIntent(intentScopedMCPTools, intent.Label)
		intentScopedMCPTools = orderMCPToolsForIntent(intentScopedMCPTools, intent.Label)
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

	// 2. Build system prompt (intent-aware).
	systemPrompt := BuildSystemPrompt(PromptContext{
		Intent:            intent.Label,
		DashboardSummary:  dashCtx,
		DashboardContext:  req.DashboardContext,
		Tools:             intentScopedMCPTools,
		CompositeToolMode: m.compositeToolModeEnabled(),
		Profile:           m.promptProfile,
	})

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
			"tool_count":            len(intentScopedMCPTools),
			"tool_count_total":      len(m.tools),
			"intent_label":          intent.Label,
			"intent_confidence":     intent.Confidence,
			"intent_rationale":      intent.Rationale,
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
	var (
		dashboardLookupContext    string
		dashboardSemanticFallback bool
		schemaContext             string
		shouldInjectKB            bool
	)
	dashboardChanged := req.DashboardContext != nil && req.DashboardContext.UID != "" && req.DashboardContext.UID != prevDashboardUID
	if m.routingModeEnabled() {
		dashboardLookupContext, dashboardSemanticFallback = m.buildDashboardLookupContext(ctx, intent, cleanMessage, req.DashboardContext)
		schemaContext = m.buildSchemaContext(ctx, intent, cleanMessage, req.DashboardContext)
		kbDecision := decideKBRouting(intent, cleanMessage, req.DashboardContext, isNewSession, dashboardChanged)
		shouldInjectKB = kbDecision.Inject
		slog.InfoContext(ctx, "evaluated KB retrieval routing",
			"event", "kb_routing",
			"intent_label", intent.Label,
			"intent_confidence", intent.Confidence,
			"inject_kb_context", kbDecision.Inject,
			"kb_route_reason", kbDecision.Reason,
			"kb_relevance_score", kbDecision.RelevanceScore,
			"kb_signals", kbDecision.Signals,
			"legacy_is_new_session", isNewSession,
			"legacy_dashboard_changed", dashboardChanged,
		)
	} else {
		slog.InfoContext(ctx, "routing mode disabled; skipping schema/dashboard/KB pre-routing",
			"event", "routing_mode_disabled",
			"intent_label", intent.Label,
			"intent_confidence", intent.Confidence,
		)
	}

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
			evidenceForClient := &api.EvidencePayload{
				KBSearch:     kbEvidence,
				VectorSearch: vectorEvidence,
			}
			if m.evidenceRedactionModeEnabled() {
				evidenceForClient = redactEvidencePayloadForClient(evidenceForClient)
			}
			streamFn(api.StreamChunk{
				Type:     "evidence",
				Evidence: evidenceForClient,
			})
		}
	}
	if schemaContext != "" {
		cleanMessage = cleanMessage + "\n\n[Schema Context]\n" + schemaContext
	}
	if dashboardLookupContext != "" {
		cleanMessage = cleanMessage + "\n\n[Dashboard Lookup Context]\n" + dashboardLookupContext
	}
	if kbContext != "" {
		cleanMessage = cleanMessage + "\n\n[KB Context]\n" + kbContext
	}
	selectedContextBlock := buildSelectedContextBlock(req.SelectedContext)
	if selectedContextBlock != "" {
		cleanMessage = cleanMessage + "\n\n[Selected Context]\n" + selectedContextBlock
	}
	mem.Add(openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: cleanMessage,
	})

	// Log injected context as audit event.
	hasSelectedContext := len(req.SelectedContext) > 0
	if sess != nil && (schemaContext != "" || dashboardLookupContext != "" || kbContext != "" || dashCtx != nil || hasSelectedContext) {
		now := time.Now()
		injection := map[string]any{}
		if schemaContext != "" {
			injection["schema_context"] = schemaContext
		}
		if dashboardLookupContext != "" {
			injection["dashboard_lookup_context"] = dashboardLookupContext
			injection["dashboard_lookup_used_semantic_fallback"] = dashboardSemanticFallback
		}
		if kbContext != "" {
			injection["kb_context"] = kbContext
		}
		if dashCtx != nil {
			injection["dashboard_context"] = dashCtx
		}
		if hasSelectedContext {
			injection["selected_context"] = req.SelectedContext
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
			"has_schema_context", schemaContext != "",
			"has_dashboard_lookup_context", dashboardLookupContext != "",
			"dashboard_lookup_used_semantic_fallback", dashboardSemanticFallback,
			"has_kb_context", kbContext != "",
			"has_dashboard_context", dashCtx != nil,
			"has_selected_context", hasSelectedContext,
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
	openaiTools := append(MCPToolsToOpenAI(intentScopedMCPTools), selectInternalTools(m.compositeToolModeEnabled(), m.promptProfile)...)
	intentLabel := string(intent.Label)
	metrics.PromptChars.Observe(float64(len(systemPrompt)))
	metrics.PromptCharsByIntent.WithLabelValues(intentLabel).Observe(float64(len(systemPrompt)))
	metrics.PromptToolCount.Observe(float64(len(openaiTools)))
	metrics.PromptToolCountByIntent.WithLabelValues(intentLabel).Observe(float64(len(openaiTools)))
	metrics.PromptToolCountFiltered.Observe(float64(len(intentScopedMCPTools)))

	// 6. Agent loop (tool calling iterations).
	var (
		finalContent           string
		promptTokensUsed       int
		completionTokensUsed   int
		toolCallsExecuted      int
		budgetTriggerReason    string
		budgetEstimatedCostUSD float64
	)

	maxToolLoopIterations := m.requestBudget.MaxToolIterations
	if maxToolLoopIterations <= 0 {
		maxToolLoopIterations = maxToolIterations
	}

	for iteration := 0; iteration <= maxToolLoopIterations; iteration++ {
		messages := mem.Messages()
		fittedMessages, dropped, estimatedPromptTokens := fitMessagesToPromptBudget(messages, m.requestBudget.MaxPromptTokens)
		messages = fittedMessages
		if dropped > 0 {
			slog.InfoContext(ctx, "trimmed chat history to fit prompt token budget",
				"event", "request_budget_trim",
				"dropped_messages", dropped,
				"estimated_prompt_tokens", estimatedPromptTokens,
				"max_prompt_tokens", m.requestBudget.MaxPromptTokens,
			)
		}
		if estimatedPromptTokens > m.requestBudget.MaxPromptTokens {
			budgetTriggerReason = budgetReasonPromptTokens
		}

		if budgetTriggerReason != "" || iteration == maxToolLoopIterations {
			if budgetTriggerReason != "" {
				metrics.RequestBudgetTripsTotal.WithLabelValues(budgetTriggerReason).Inc()
				slog.WarnContext(ctx, "request budget guardrail triggered",
					"event", "request_budget_triggered",
					"reason", budgetTriggerReason,
					"prompt_tokens_used", promptTokensUsed,
					"completion_tokens_used", completionTokensUsed,
					"tool_calls_executed", toolCallsExecuted,
					"estimated_cost_usd", budgetEstimatedCostUSD,
				)
				guidance := budgetGuidanceMessage(budgetTriggerReason)
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: guidance,
				})
				streamFn(api.StreamChunk{Type: "token", Message: budgetUserMessage(budgetTriggerReason) + " "})
			}

			remainingCompletion := remainingCompletionTokens(m.requestBudget, completionTokensUsed)
			if remainingCompletion == 0 {
				finalContent = strings.TrimSpace(budgetUserMessage(budgetReasonCompletionTokens))
				streamFn(api.StreamChunk{Type: "complete", Message: finalContent})
				streamFn(api.StreamChunk{Type: "done"})
				break
			}

			streamOpts := llm.ChatOptions{MaxCompletionTokens: remainingCompletion}
			ch, err := m.llm.StreamChatWithOptions(ctx, messages, nil, streamOpts)
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

		remainingCompletion := remainingCompletionTokens(m.requestBudget, completionTokensUsed)
		if remainingCompletion == 0 {
			budgetTriggerReason = budgetReasonCompletionTokens
			continue
		}

		// Use non-streaming call for tool loop iterations to check for tool calls.
		resp, usage, err := m.llm.ChatWithOptions(ctx, messages, openaiTools, llm.ChatOptions{MaxCompletionTokens: remainingCompletion})
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

		promptTokens := usage.PromptTokens
		if promptTokens <= 0 {
			promptTokens = estimatedPromptTokens
		}
		completionTokens := usage.CompletionTokens
		if completionTokens <= 0 {
			completionTokens = estimateTextTokens(resp.Content)
		}
		promptTokensUsed += promptTokens
		completionTokensUsed += completionTokens
		budgetEstimatedCostUSD = estimateRequestCostUSD(
			promptTokensUsed,
			completionTokensUsed,
			m.requestBudget.PromptCostPer1MUSD,
			m.requestBudget.CompletionCostPer1MUSD,
		)
		metrics.RequestBudgetEstimatedCostUSD.Observe(budgetEstimatedCostUSD)
		if m.requestBudget.MaxEstimatedCostUSD > 0 && budgetEstimatedCostUSD > m.requestBudget.MaxEstimatedCostUSD {
			budgetTriggerReason = budgetReasonEstimatedCost
		}
		if completionTokensUsed >= m.requestBudget.MaxCompletionTokens {
			budgetTriggerReason = budgetReasonCompletionTokens
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
				ch, err := m.llm.StreamChatWithOptions(ctx, messages, nil, llm.ChatOptions{MaxCompletionTokens: remainingCompletion})
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
			if toolCallsExecuted >= m.requestBudget.MaxToolCalls {
				budgetTriggerReason = budgetReasonToolCalls
				break
			}
			toolCallsExecuted++

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{"raw": tc.Function.Arguments}
			}
			metrics.ToolCallsTotal.WithLabelValues(tc.Function.Name).Inc()
			toolReason := deriveToolCallReason(tc.Function.Name, args)
			argsForClientValue := any(args)
			if m.evidenceRedactionModeEnabled() {
				argsForClientValue = redactToolStreamValueForClient(args)
			}
			argsForClient, _ := argsForClientValue.(map[string]any)
			if argsForClient == nil {
				argsForClient = map[string]any{}
			}

			// Stream tool invocation to client.
			streamFn(api.StreamChunk{
				Type:      "tool",
				Tool:      tc.Function.Name,
				Reason:    toolReason,
				ToolID:    tc.ID,
				Arguments: argsForClient,
			})
			slog.InfoContext(ctx, "Executing tool call",
				"event", "tool_call",
				"session_id", sess.ID,
				"user_id", user.ID,
				"org_id", user.OrgID,
				"dashboard_uid", sess.DashboardUID,
				"tool_name", tc.Function.Name,
				"params", argsForClient,
			)

			// Execute tool.
			result, err := m.handleInternalTool(ctx, tc.Function.Name, args, user, sess, req.DashboardContext)
			if err == nil && result == nil {
				result, err = m.invokeMCPTool(ctx, tc.Function.Name, args)
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

			resultForClient, resultForModel := shapeToolResultOutputsWithMode(result, m.evidenceRedactionModeEnabled())

			// Stream tool result to client.
			streamFn(api.StreamChunk{
				Type:   "tool",
				Tool:   tc.Function.Name,
				Reason: toolReason,
				ToolID: tc.ID,
				Result: resultForClient,
			})

			if sess != nil {
				now := time.Now()
				// Keep raw tool response in server-side audit logs for forensics.
				// Redaction applies to client stream payloads only.
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
				Content:    resultForModel,
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

// shapeToolResultOutputs keeps raw data for client evidence while feeding
// summary-first formatted content to model memory.
func shapeToolResultOutputs(result any) (any, string) {
	return shapeToolResultOutputsWithMode(result, true)
}

func shapeToolResultOutputsWithMode(result any, evidenceRedactionMode bool) (any, string) {
	streamPayload := result
	if evidenceRedactionMode {
		streamPayload = redactToolStreamValueForClient(result)
	}
	return streamPayload, wrapToolResult(mcp.FormatToolResult(result))
}

func deriveToolCallReason(toolName string, args map[string]any) string {
	if explicit := extractExplicitToolReason(args); explicit != "" {
		return explicit
	}

	if strings.EqualFold(strings.TrimSpace(toolName), "investigation__manage") {
		if investigationReason := deriveInvestigationManageReason(args); investigationReason != "" {
			return investigationReason
		}
	}

	if query := firstNonEmptyArgString(args, "query", "expr", "promql", "logql"); query != "" {
		return fmt.Sprintf("Run %s using query %q.", toolName, truncate(query, 110))
	}

	if target := firstNonEmptyArgString(args, "uid", "dashboardUid", "datasourceUid", "ruleUid", "alertUid"); target != "" {
		return fmt.Sprintf("Use %s for target %q.", toolName, truncate(target, 90))
	}

	return fmt.Sprintf("Use %s to gather evidence for the current request.", toolName)
}

func extractExplicitToolReason(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	if reason := strings.TrimSpace(toString(args["reason"])); reason != "" {
		return truncate(reason, 220)
	}

	action := strings.ToLower(strings.TrimSpace(toString(args["action"])))
	payloadKey, ok := investigationPayloadByAction[action]
	if !ok || payloadKey == "" {
		return ""
	}

	payload, ok := args[payloadKey].(map[string]any)
	if !ok {
		return ""
	}
	if reason := strings.TrimSpace(toString(payload["reason"])); reason != "" {
		return truncate(reason, 220)
	}
	return ""
}

func deriveInvestigationManageReason(args map[string]any) string {
	action := strings.ToLower(strings.TrimSpace(toString(args["action"])))
	switch action {
	case investigationActionPlan:
		return "Plan investigation scope and sequence before running fetch actions."
	case investigationActionFetchMetrics:
		if query := investigationPayloadQuery(args, investigationActionFetchMetrics); query != "" {
			return fmt.Sprintf("Fetch metrics to validate impact using query %q.", truncate(query, 110))
		}
		return "Fetch metrics to validate impact and timeframe."
	case investigationActionFetchLogs:
		if query := investigationPayloadQuery(args, investigationActionFetchLogs); query != "" {
			return fmt.Sprintf("Fetch logs to correlate events using query %q.", truncate(query, 110))
		}
		return "Fetch logs to correlate events around the anomaly window."
	case investigationActionSummarize:
		return "Summarize investigation findings into a concise incident update."
	case investigationActionNextStep:
		return "Recommend the next investigation step based on current evidence."
	default:
		return ""
	}
}

func investigationPayloadQuery(args map[string]any, action string) string {
	payloadKey, ok := investigationPayloadByAction[action]
	if !ok {
		return ""
	}
	payload, ok := args[payloadKey].(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(toString(payload["query"]))
}

func firstNonEmptyArgString(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(toString(args[key])); value != "" {
			return value
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
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

func boolOrDefault(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func (m *Manager) routingModeEnabled() bool {
	if !m.flagsInitialized {
		return true
	}
	return m.routingMode
}

func (m *Manager) compositeToolModeEnabled() bool {
	if !m.flagsInitialized {
		return true
	}
	return m.compositeToolMode
}

func (m *Manager) judgeGateModeEnabled() bool {
	if !m.flagsInitialized {
		return true
	}
	return m.judgeGateMode
}

func (m *Manager) evidenceRedactionModeEnabled() bool {
	if !m.flagsInitialized {
		return true
	}
	return m.evidenceRedactionMode
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

func (m *Manager) invokeMCPTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if client := m.getCachedToolClient(name); client != nil {
		return client.InvokeTool(ctx, name, args)
	}

	client, err := FindToolClient(ctx, name, m.mcp)
	if err != nil {
		return nil, err
	}
	m.setCachedToolClient(name, client)
	return client.InvokeTool(ctx, name, args)
}

func (m *Manager) getCachedToolClient(name string) mcp.Client {
	m.toolClientMu.RLock()
	client := m.toolClientByName[name]
	m.toolClientMu.RUnlock()
	return client
}

func (m *Manager) setCachedToolClient(name string, client mcp.Client) {
	if client == nil || strings.TrimSpace(name) == "" {
		return
	}
	m.toolClientMu.Lock()
	if m.toolClientByName == nil {
		m.toolClientByName = map[string]mcp.Client{}
	}
	m.toolClientByName[name] = client
	m.toolClientMu.Unlock()
}

func (m *Manager) handleInternalTool(ctx context.Context, name string, args map[string]any, user *grafana.User, sess *storage.Session, reqCtx *api.DashboardContext) (any, error) {
	switch name {
	case "investigation__manage":
		if !m.compositeToolModeEnabled() {
			requestedAction := strings.ToLower(strings.TrimSpace(toString(args["action"])))
			return map[string]any{
				"status":    "tool_unavailable",
				"action":    requestedAction,
				"retryable": false,
				"message":   "investigation__manage is disabled by feature flag",
			}, nil
		}
		action, payloadKey, payload, err := parseInvestigationManageArgs(args)
		if err != nil {
			requestedAction := strings.ToLower(strings.TrimSpace(toString(args["action"])))
			return map[string]any{
				"status":           "input_error",
				"action":           requestedAction,
				"retryable":        true,
				"message":          err.Error(),
				"supportedActions": investigationManageActions,
			}, nil
		}
		investigationID := strings.TrimSpace(toString(args["investigationId"]))
		result := m.routeInvestigationAction(ctx, action, investigationID, payload, reqCtx)
		if result == nil {
			return nil, fmt.Errorf("investigation action %q produced no result", action)
		}
		result["payloadKey"] = payloadKey
		return result, nil
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
