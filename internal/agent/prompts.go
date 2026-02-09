package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/brownster/grafana-assistant/internal/api"
	appcontext "github.com/brownster/grafana-assistant/internal/context"
	"github.com/brownster/grafana-assistant/internal/mcp"
)

const maxToolHighlights = 12

// SystemPrompt builds the system prompt with optional dashboard context and tool descriptions.
// It delegates to BuildSystemPrompt with IntentLiveData as default for backward compatibility.
func SystemPrompt(dashCtx *appcontext.DashboardSummary, reqCtx *api.DashboardContext, tools []mcp.Tool, compositeToolMode bool, profile PromptProfile) string {
	return BuildSystemPrompt(PromptContext{
		Intent:            IntentLiveData,
		DashboardSummary:  dashCtx,
		DashboardContext:  reqCtx,
		Tools:             tools,
		CompositeToolMode: compositeToolMode,
		Profile:           profile,
	})
}

func compactToolSummary(tools []mcp.Tool) string {
	var b strings.Builder

	serverCounts := map[string]int{}
	for _, t := range tools {
		serverCounts[toolServerPrefix(t.Name)]++
	}

	servers := make([]string, 0, len(serverCounts))
	for server := range serverCounts {
		servers = append(servers, server)
	}
	sort.Strings(servers)

	b.WriteString(fmt.Sprintf("- Total MCP tools enabled: %d across %d server(s).\n", len(tools), len(servers)))
	if len(servers) > 0 {
		parts := make([]string, 0, len(servers))
		for _, server := range servers {
			parts = append(parts, fmt.Sprintf("%s(%d)", server, serverCounts[server]))
		}
		b.WriteString(fmt.Sprintf("- Server coverage: %s\n", strings.Join(parts, ", ")))
	}

	highValue := pickHighValueTools(tools, maxToolHighlights)
	if len(highValue) == 0 {
		b.WriteString("- No high-value tools detected in current profile.\n")
		b.WriteString("- Full function schemas are provided separately at runtime.\n")
		return b.String()
	}

	b.WriteString("- High-value tools (sample, not exhaustive):\n")
	for _, t := range highValue {
		desc := strings.TrimSpace(t.Description)
		if desc == "" {
			desc = "No description provided."
		}
		b.WriteString(fmt.Sprintf("  - `%s`: %s\n", t.Name, truncateForPrompt(desc, 110)))
	}
	b.WriteString("- Full function schemas are provided separately at runtime.\n")
	return b.String()
}

type rankedTool struct {
	tool  mcp.Tool
	score int
}

func pickHighValueTools(tools []mcp.Tool, limit int) []mcp.Tool {
	if len(tools) == 0 || limit <= 0 {
		return nil
	}

	ranked := make([]rankedTool, 0, len(tools))
	for _, t := range tools {
		ranked = append(ranked, rankedTool{
			tool:  t,
			score: scoreTool(t.Name),
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].tool.Name < ranked[j].tool.Name
	})

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	out := make([]mcp.Tool, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, r.tool)
	}
	return out
}

func scoreTool(name string) int {
	short := toolShortName(name)
	score := 0

	switch short {
	case "query_prometheus", "query_loki_logs", "search_dashboards", "get_dashboard_summary", "get_dashboard_panel_queries":
		score += 100
	case "list_alerts", "list_alert_rules", "get_alert_rule_by_uid", "list_prometheus_metric_names", "list_loki_label_names":
		score += 90
	}

	if strings.HasPrefix(short, "query_") {
		score += 40
	}
	if strings.HasPrefix(short, "search_") {
		score += 35
	}
	if strings.HasPrefix(short, "list_") {
		score += 20
	}
	if strings.HasPrefix(short, "get_") {
		score += 10
	}
	if strings.Contains(short, "create") || strings.Contains(short, "update") || strings.Contains(short, "delete") || strings.Contains(short, "patch") {
		score -= 25
	}

	return score
}

func toolServerPrefix(name string) string {
	parts := strings.SplitN(name, "__", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
		return parts[0]
	}
	return "unknown"
}

func toolShortName(name string) string {
	parts := strings.SplitN(name, "__", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		return parts[1]
	}
	return name
}

func truncateForPrompt(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
