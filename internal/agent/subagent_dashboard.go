package agent

const dashboardSpecialistPrompt = `You are a dashboard search specialist. Your ONLY job is to find the right Grafana dashboard.
Use search_dashboards to find dashboards by name, tag, or content.
Use get_dashboard_summary to verify a dashboard matches the user's need.
Return the dashboard UID, title, folder, and a one-sentence description of what it shows.
If you cannot find a matching dashboard, say so clearly.
Do NOT answer the user's question - only find the dashboard.`

func (m *Manager) newDashboardSubAgent() *SubAgent {
	tools := filterToolsForIntent(m.tools, IntentDashboardLookup)
	return &SubAgent{
		Name:          "dashboard",
		SystemPrompt:  dashboardSpecialistPrompt,
		Tools:         tools,
		MaxIterations: 3,
	}
}

func isDashboardLookupOnlyRequest(message string) bool {
	text := normalizeIntentText(message)
	if text == "" {
		return false
	}

	hasLookupSignal := containsAnyNormalized(text,
		"find dashboard",
		"search dashboard",
		"open dashboard",
		"which dashboard",
		"where is dashboard",
		"dashboard uid",
		"list dashboards",
		"locate dashboard",
		"find panel",
		"where is panel",
	) || (containsAnyNormalized(text, "dashboard", "dashboards", "panel") &&
		containsAnyNormalized(text, "find", "search", "open", "locate", "where", "which", "list"))
	if !hasLookupSignal {
		return false
	}

	analysisSignals := []string{
		"why",
		"investigate",
		"analyze",
		"trend",
		"spike",
		"error rate",
		"latency",
		"cpu",
		"memory",
		"logs",
		"alerts",
		"summary",
	}
	for _, signal := range analysisSignals {
		if containsAnyNormalized(text, signal) {
			return false
		}
	}

	return true
}
