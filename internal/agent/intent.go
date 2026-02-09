package agent

import (
	"strings"
	"unicode"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

// IntentClass labels the user request class used for retrieval routing.
type IntentClass string

const (
	IntentLiveData        IntentClass = "live_data"
	IntentQueryHelp       IntentClass = "query_help"
	IntentDashboardLookup IntentClass = "dashboard_lookup"
	IntentHowToDocs       IntentClass = "how_to_docs"
	IntentIncidentSummary IntentClass = "incident_summary"
)

// IntentResult is a deterministic classification result with traceable reasoning.
type IntentResult struct {
	Label      IntentClass `json:"label"`
	Confidence float64     `json:"confidence"`
	Rationale  string      `json:"rationale"`
}

type intentScore struct {
	score   int
	reasons []string
}

// ClassifyIntent applies rule-based heuristics to classify user intent.
func ClassifyIntent(message string, reqCtx *api.DashboardContext) IntentResult {
	text := normalizeIntentText(message)
	scores := map[IntentClass]*intentScore{
		IntentLiveData:        {},
		IntentQueryHelp:       {},
		IntentDashboardLookup: {},
		IntentHowToDocs:       {},
		IntentIncidentSummary: {},
	}

	incidentTerms := containsAnyNormalized(text, "incident", "outage", "sev1", "sev2", "postmortem", "root cause", "rca", "blast radius", "impact")
	summaryTerms := containsAnyNormalized(text, "summarize", "summary", "timeline", "what happened", "recap")
	if incidentTerms {
		addIntentScore(scores, IntentIncidentSummary, 5, "incident keywords")
	}
	if summaryTerms {
		addIntentScore(scores, IntentIncidentSummary, 4, "summary/timeline request")
	}
	if incidentTerms && summaryTerms {
		addIntentScore(scores, IntentIncidentSummary, 2, "incident+summary combination")
	}

	if containsAnyNormalized(text, "find dashboard", "search dashboard", "open dashboard", "which dashboard", "dashboard uid", "list dashboards", "where is dashboard", "locate dashboard") {
		addIntentScore(scores, IntentDashboardLookup, 6, "dashboard lookup phrase")
	}
	if containsAnyNormalized(text, "find panel", "which panel", "where is panel", "panel with", "locate panel") {
		addIntentScore(scores, IntentDashboardLookup, 6, "panel lookup phrase")
	}
	if containsAnyNormalized(text, "dashboard", "panel", "uid", "folder", "tag") && containsAnyNormalized(text, "find", "search", "open", "locate", "where", "which", "list") {
		addIntentScore(scores, IntentDashboardLookup, 3, "dashboard metadata lookup pattern")
	}

	if containsAnyNormalized(text, "promql", "logql", "query syntax", "query help", "write query", "build query", "fix query", "debug query", "invalid query", "parse error", "syntax error") {
		addIntentScore(scores, IntentQueryHelp, 6, "query-language help phrase")
	}
	if containsAnyNormalized(text, "query", "selector", "label matcher", "histogram_quantile", "rate(", "sum by", "group by", "log query") {
		addIntentScore(scores, IntentQueryHelp, 3, "query construction terms")
	}

	if containsAnyNormalized(text, "how to", "how do i", "docs", "documentation", "explain", "guide", "runbook", "what does", "configure", "setup", "install", "why does") {
		addIntentScore(scores, IntentHowToDocs, 4, "how-to/docs phrase")
	}
	if containsAnyNormalized(text, "best practice", "example", "tutorial", "walkthrough") {
		addIntentScore(scores, IntentHowToDocs, 2, "documentation guidance terms")
	}

	if containsAnyNormalized(text, "right now", "current", "now", "today", "last", "latest", "how many", "show me", "are there", "what is") {
		addIntentScore(scores, IntentLiveData, 2, "live-state wording")
	}
	if containsAnyNormalized(text, "cpu", "memory", "latency", "error rate", "throughput", "alerts", "firing", "logs", "p95", "p99", "availability", "uptime") {
		addIntentScore(scores, IntentLiveData, 3, "live-metric terms")
	}
	if containsAnyNormalized(text, "status", "health", "spike", "trend", "anomaly", "degraded", "down") {
		addIntentScore(scores, IntentLiveData, 2, "operational state terms")
	}

	if reqCtx != nil {
		if reqCtx.Explore != nil {
			addIntentScore(scores, IntentQueryHelp, 1, "explore context present")
		}
		if reqCtx.UID != "" || reqCtx.Name != "" {
			addIntentScore(scores, IntentLiveData, 1, "dashboard context present")
		}
	}

	best := IntentLiveData
	bestScore := -1
	tieBreakOrder := []IntentClass{
		IntentDashboardLookup,
		IntentIncidentSummary,
		IntentQueryHelp,
		IntentHowToDocs,
		IntentLiveData,
	}
	for _, intent := range tieBreakOrder {
		s := scores[intent].score
		if s > bestScore {
			best = intent
			bestScore = s
		}
	}

	if bestScore <= 0 {
		return IntentResult{
			Label:      IntentLiveData,
			Confidence: 0.60,
			Rationale:  "defaulted to live_data: no strong deterministic keyword match",
		}
	}

	conf := confidenceForScore(bestScore)
	reasons := scores[best].reasons
	rationale := "matched: " + strings.Join(reasons, "; ")
	return IntentResult{
		Label:      best,
		Confidence: conf,
		Rationale:  rationale,
	}
}

func addIntentScore(scores map[IntentClass]*intentScore, intent IntentClass, delta int, reason string) {
	scores[intent].score += delta
	scores[intent].reasons = append(scores[intent].reasons, reason)
}

func confidenceForScore(score int) float64 {
	switch {
	case score >= 10:
		return 0.95
	case score >= 8:
		return 0.90
	case score >= 6:
		return 0.85
	case score >= 4:
		return 0.78
	default:
		return 0.70
	}
}

func containsAny(text string, needles ...string) bool {
	return containsAnyNormalized(normalizeIntentText(text), needles...)
}

func containsAnyNormalized(text string, needles ...string) bool {
	if text == "" {
		return false
	}
	wordNormalizedText := ""

	for _, n := range needles {
		needle := strings.ToLower(strings.TrimSpace(n))
		if needle == "" {
			continue
		}

		if isLiteralNeedle(needle) {
			if strings.Contains(text, needle) {
				return true
			}
			continue
		}

		if wordNormalizedText == "" {
			wordNormalizedText = normalizeWordMatch(text)
		}
		needleWord := normalizeWordMatch(needle)
		if needleWord == "" {
			continue
		}
		if strings.Contains(" "+wordNormalizedText+" ", " "+needleWord+" ") {
			return true
		}
	}
	return false
}

func isLiteralNeedle(needle string) bool {
	for _, r := range needle {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '_' {
			continue
		}
		return true
	}
	return false
}

func normalizeWordMatch(in string) string {
	var b strings.Builder
	b.Grow(len(in))

	lastSpace := true
	for _, r := range strings.ToLower(in) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizeIntentText(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.Join(strings.Fields(lower), " ")
}
