package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/brownster/grafana-assistant/internal/api"
	"github.com/brownster/grafana-assistant/internal/grafana"
	"github.com/brownster/grafana-assistant/internal/mcp"
	"github.com/brownster/grafana-assistant/internal/storage"
)

// CoordinatorDecision determines whether to use direct path or delegate.
type CoordinatorDecision struct {
	UseDirect bool     // Use existing single-stream path.
	SubAgents []string // Sub-agent names to invoke.
	Reason    string   // Audit trail.
}

func (m *Manager) coordinatorDecide(intent IntentResult, message string) CoordinatorDecision {
	if strings.TrimSpace(message) == "" {
		return CoordinatorDecision{
			UseDirect: true,
			Reason:    "empty message",
		}
	}

	if intent.Confidence >= 0.90 && intent.Label == IntentHowToDocs {
		return CoordinatorDecision{
			UseDirect: true,
			Reason:    "high-confidence docs intent",
		}
	}

	if intent.Confidence >= 0.90 && intent.Label == IntentDashboardLookup {
		if !isDashboardLookupOnlyRequest(message) {
			return CoordinatorDecision{
				UseDirect: true,
				Reason:    "dashboard lookup with analysis signals -> direct path",
			}
		}
		return CoordinatorDecision{
			SubAgents: []string{"dashboard"},
			Reason:    "high-confidence dashboard lookup -> delegate to specialist",
		}
	}

	if intent.Label == IntentIncidentSummary {
		return CoordinatorDecision{
			SubAgents: []string{"investigation"},
			Reason:    "incident summary -> delegate to investigation specialist",
		}
	}

	return CoordinatorDecision{
		UseDirect: true,
		Reason:    "default direct path",
	}
}

func (m *Manager) handleChatViaSubAgents(
	ctx context.Context,
	user *grafana.User,
	req api.ChatRequest,
	sess *storage.Session,
	intent IntentResult,
	decision CoordinatorDecision,
	userMessage string,
	streamFn func(api.StreamChunk),
) (string, bool) {
	if decision.UseDirect || len(decision.SubAgents) == 0 {
		return "", false
	}
	if m.llm == nil {
		slog.WarnContext(ctx, "sub-agent mode enabled but llm client is nil; falling back to direct path")
		return "", false
	}

	toolClients := m.snapshotToolClientMap()
	if len(toolClients) == 0 {
		slog.WarnContext(ctx, "sub-agent mode enabled but no tool clients cached; falling back to direct path")
		return "", false
	}

	results := make([]SubAgentResult, 0, len(decision.SubAgents))
	for _, name := range decision.SubAgents {
		subAgent := m.newSubAgentByName(name, intent, req.DashboardContext)
		if subAgent == nil {
			slog.WarnContext(ctx, "coordinator skipped unknown sub-agent", "sub_agent", name)
			continue
		}

		subAgentStreamFn := func(chunk api.StreamChunk) {
			if m.evidenceRedactionModeEnabled() {
				if len(chunk.Arguments) > 0 {
					redactedArgsValue := redactToolStreamValueForClient(chunk.Arguments)
					redactedArgs, _ := redactedArgsValue.(map[string]any)
					if redactedArgs == nil {
						redactedArgs = map[string]any{}
					}
					chunk.Arguments = redactedArgs
				}
				if chunk.Result != nil {
					chunk.Result = redactToolStreamValueForClient(chunk.Result)
				}
			}
			streamFn(chunk)
		}

		res := subAgent.Execute(ctx, m.llm, toolClients, userMessage, subAgentStreamFn)
		if res.Metadata == nil {
			res.Metadata = map[string]string{}
		}
		res.Metadata["coordinator_reason"] = decision.Reason
		results = append(results, res)
	}
	if len(results) == 0 {
		return "", false
	}

	summary := synthesizeSubAgentResponse(decision, results)
	if summary == "" {
		summary = "I could not complete delegated specialist analysis for this request."
	}

	streamFn(api.StreamChunk{Type: "token", Message: summary})
	streamFn(api.StreamChunk{Type: "complete", Message: summary})
	streamFn(api.StreamChunk{Type: "done"})

	if sess != nil && user != nil {
		now := time.Now()
		_ = m.store.AddAuditEntry(ctx, &storage.AuditEntry{
			ID:           fmt.Sprintf("%s-%d-audit-subagent", sess.ID, now.UnixMilli()),
			SessionID:    sess.ID,
			UserID:       user.ID,
			OrgID:        user.OrgID,
			DashboardUID: sess.DashboardUID,
			EventType:    "coordinator_subagent",
			Request: marshalAuditValue(map[string]any{
				"decision_reason":     decision.Reason,
				"intent_label":        intent.Label,
				"intent_confidence":   intent.Confidence,
				"delegated_subagents": decision.SubAgents,
			}),
			Response:  marshalAuditValue(results),
			CreatedAt: now,
		})
	}

	return summary, true
}

func (m *Manager) newSubAgentByName(name string, intent IntentResult, reqCtx *api.DashboardContext) *SubAgent {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "dashboard":
		return m.newDashboardSubAgent()
	case "investigation":
		return &SubAgent{
			Name:          "investigation",
			SystemPrompt:  investigationCoordinatorPrompt(reqCtx, intent),
			Tools:         append([]mcp.Tool(nil), m.tools...),
			MaxIterations: 5,
		}
	default:
		return nil
	}
}

func (m *Manager) snapshotToolClientMap() map[string]mcp.Client {
	m.toolClientMu.RLock()
	defer m.toolClientMu.RUnlock()
	snapshot := make(map[string]mcp.Client, len(m.toolClientByName))
	for name, client := range m.toolClientByName {
		snapshot[name] = client
	}
	return snapshot
}

func investigationCoordinatorPrompt(reqCtx *api.DashboardContext, intent IntentResult) string {
	var b strings.Builder
	b.WriteString("You are an incident investigation specialist.\n")
	b.WriteString("Gather evidence from metrics, logs, and alerts, then return a concise findings summary.\n")
	b.WriteString("Prioritize factual evidence and avoid speculation.\n")
	if reqCtx != nil && strings.TrimSpace(reqCtx.UID) != "" {
		b.WriteString("Dashboard UID context: ")
		b.WriteString(strings.TrimSpace(reqCtx.UID))
		b.WriteString(".\n")
	}
	if strings.TrimSpace(intent.Rationale) != "" {
		b.WriteString("Intent rationale: ")
		b.WriteString(strings.TrimSpace(intent.Rationale))
	}
	return strings.TrimSpace(b.String())
}

func synthesizeSubAgentResponse(decision CoordinatorDecision, results []SubAgentResult) string {
	if len(results) == 1 {
		res := results[0]
		if strings.TrimSpace(res.Summary) != "" {
			return strings.TrimSpace(res.Summary)
		}
		if strings.TrimSpace(res.Error) != "" {
			return fmt.Sprintf("Delegated specialist failed: %s", strings.TrimSpace(res.Error))
		}
	}

	lines := make([]string, 0, 1+len(results))
	if strings.TrimSpace(decision.Reason) != "" {
		lines = append(lines, "Coordinator decision: "+strings.TrimSpace(decision.Reason))
	}
	for _, res := range results {
		name := strings.TrimSpace(res.Metadata["agent_name"])
		if name == "" {
			name = "specialist"
		}
		line := fmt.Sprintf("%s: %s", name, strings.TrimSpace(res.Summary))
		if strings.TrimSpace(res.Summary) == "" {
			line = fmt.Sprintf("%s: no summary produced", name)
		}
		if strings.TrimSpace(res.Error) != "" {
			line += fmt.Sprintf(" (error: %s)", strings.TrimSpace(res.Error))
		}
		if len(res.ToolsUsed) > 0 {
			line += fmt.Sprintf(" [tools: %s]", strings.Join(res.ToolsUsed, ", "))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
