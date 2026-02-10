package agent

import (
	"fmt"
	"strings"

	appcontext "github.com/brownster/grafana-assistant/internal/context"
	"github.com/brownster/grafana-assistant/internal/mcp"
)

const investigationSchemaContextLimit = 2400
const investigationPanelLimit = 12

func (m *Manager) newInvestigationSubAgent(dashCtx *appcontext.DashboardSummary, schemaCtx string) *SubAgent {
	return &SubAgent{
		Name:          "investigation",
		SystemPrompt:  buildInvestigationPrompt(dashCtx, schemaCtx),
		Tools:         append([]mcp.Tool(nil), m.tools...),
		MaxIterations: 5,
	}
}

func buildInvestigationPrompt(dashCtx *appcontext.DashboardSummary, schemaCtx string) string {
	var b strings.Builder
	b.WriteString("You are an incident investigation specialist for monitoring and observability.\n")
	b.WriteString("Objective: determine what happened, when it started, impacted systems, likely causes, and next actions.\n")
	b.WriteString("Use available tools to gather metrics, alerts, logs, and dashboard evidence before concluding.\n")
	b.WriteString("Prefer the composite investigation tool for multi-step workflows when available.\n")
	b.WriteString("Do not speculate beyond the observed evidence.\n\n")

	b.WriteString("Investigation workflow:\n")
	b.WriteString("1. Establish incident scope and timeline.\n")
	b.WriteString("2. Verify symptoms with metrics and alert state.\n")
	b.WriteString("3. Correlate logs with metric/alert transitions.\n")
	b.WriteString("4. Summarize findings with explicit evidence.\n")
	b.WriteString("5. Provide prioritized next actions.\n\n")

	if dashCtx != nil {
		b.WriteString("Dashboard context:\n")
		b.WriteString(fmt.Sprintf("- UID: %s\n", strings.TrimSpace(dashCtx.UID)))
		b.WriteString(fmt.Sprintf("- Title: %s\n", strings.TrimSpace(dashCtx.Title)))
		if strings.TrimSpace(dashCtx.Folder) != "" {
			b.WriteString(fmt.Sprintf("- Folder: %s\n", strings.TrimSpace(dashCtx.Folder)))
		}
		if len(dashCtx.Tags) > 0 {
			b.WriteString(fmt.Sprintf("- Tags: %s\n", strings.Join(dashCtx.Tags, ", ")))
		}
		if len(dashCtx.Panels) > 0 {
			b.WriteString("- Panels:\n")
			limit := len(dashCtx.Panels)
			if limit > investigationPanelLimit {
				limit = investigationPanelLimit
			}
			for i := 0; i < limit; i++ {
				panel := dashCtx.Panels[i]
				title := strings.TrimSpace(panel.Title)
				if title == "" {
					title = fmt.Sprintf("panel_%d", i+1)
				}
				panelType := strings.TrimSpace(panel.Type)
				if panelType == "" {
					panelType = "unknown"
				}
				b.WriteString(fmt.Sprintf("  - %s (type=%s)\n", title, panelType))
			}
		}
		b.WriteString("\n")
	}

	schema := strings.TrimSpace(schemaCtx)
	if schema != "" {
		b.WriteString("Schema context (metrics/labels/log fields):\n")
		b.WriteString(truncate(schema, investigationSchemaContextLimit))
		b.WriteString("\n\n")
	}

	b.WriteString("Output format:\n")
	b.WriteString("- Incident Summary: one concise paragraph.\n")
	b.WriteString("- Evidence: bullet list with metric/log/alert references.\n")
	b.WriteString("- Timeline: ordered notable events with relative times if available.\n")
	b.WriteString("- Next Actions: 2-4 concrete remediation or validation steps.\n")

	return strings.TrimSpace(b.String())
}
