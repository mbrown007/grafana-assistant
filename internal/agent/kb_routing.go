package agent

import (
	"strings"

	"github.com/brownster/grafana-assistant/internal/api"
)

type kbRoutingDecision struct {
	Inject         bool
	Reason         string
	RelevanceScore int
	Signals        []string
}

func shouldRouteDocsKB(intent IntentClass) bool {
	return intent == IntentHowToDocs
}

func isDocsRetrievalTool(name string) bool {
	switch toolShortName(name) {
	case "search_kb", "search_kb_semantic", "get_kb_section":
		return true
	default:
		return false
	}
}

func decideKBRouting(intent IntentResult, message string, reqCtx *api.DashboardContext, isNewSession, dashboardChanged bool) kbRoutingDecision {
	decision := kbRoutingDecision{
		Inject: false,
		Reason: "intent_not_docs",
	}

	if !shouldRouteDocsKB(intent.Label) {
		text := normalizeIntentText(message)
		if hasExplicitDocsRequest(text) {
			score := 1
			signals := []string{"explicit_docs_request"}
			if hasDashboardOrExploreContext(reqCtx) {
				score++
				signals = append(signals, "dashboard_or_explore_context")
			}
			if containsAnyNormalized(text, "error", "failed", "unable", "debug", "troubleshoot", "incident", "sev1", "sev2") {
				score++
				signals = append(signals, "operational_problem_context")
			}

			decision.Inject = true
			decision.Reason = "cross_intent_docs_override"
			decision.RelevanceScore = score
			decision.Signals = signals
			return decision
		}

		if isNewSession {
			decision.Signals = append(decision.Signals, "legacy_new_session_ignored")
		}
		if dashboardChanged {
			decision.Signals = append(decision.Signals, "legacy_dashboard_change_ignored")
		}
		return decision
	}

	text := normalizeIntentText(message)
	score := 0
	signals := make([]string, 0, 4)

	if containsAnyNormalized(text,
		"how to", "how do i", "docs", "documentation", "guide", "runbook", "tutorial", "walkthrough",
		"configure", "setup", "install", "why does", "explain", "what does",
	) {
		score += 2
		signals = append(signals, "docs_keywords")
	}

	if containsAnyNormalized(text,
		"error", "failed", "unable", "not working", "broken", "invalid", "troubleshoot", "debug", "fix",
	) {
		score++
		signals = append(signals, "troubleshooting_terms")
	}

	if hasDashboardOrExploreContext(reqCtx) {
		score++
		signals = append(signals, "dashboard_or_explore_context")
	}

	if intent.Confidence >= 0.85 {
		score++
		signals = append(signals, "high_intent_confidence")
	}

	decision.RelevanceScore = score
	decision.Signals = signals

	if score >= 2 {
		decision.Inject = true
		decision.Reason = "docs_intent_relevance_met"
		return decision
	}

	decision.Inject = false
	decision.Reason = "docs_intent_low_relevance"
	return decision
}

func hasDashboardOrExploreContext(reqCtx *api.DashboardContext) bool {
	if reqCtx == nil {
		return false
	}
	if strings.TrimSpace(reqCtx.UID) != "" || strings.TrimSpace(reqCtx.Name) != "" {
		return true
	}
	if len(reqCtx.Tags) > 0 || len(reqCtx.Variables) > 0 {
		return true
	}
	if reqCtx.Explore != nil {
		if strings.TrimSpace(reqCtx.Explore.Datasource) != "" || len(reqCtx.Explore.Queries) > 0 {
			return true
		}
	}
	return false
}

func hasExplicitDocsRequest(text string) bool {
	return containsAnyNormalized(text,
		"runbook",
		"docs",
		"documentation",
		"guide",
		"playbook",
		"knowledge base",
		"kb article",
	)
}
