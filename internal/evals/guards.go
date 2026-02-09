package evals

import (
	"regexp"
	"sort"
	"strings"
)

const (
	GuardRuleNoFabricatedMetricValues = "no_fabricated_metric_values_without_data"
	GuardRuleToolErrorHandling        = "tool_errors_must_be_acknowledged"
	GuardRuleDocsEvidenceSignals      = "required_evidence_signal_for_docs_cases"
)

var (
	metricValuePattern = regexp.MustCompile(`(?i)\b\d+(?:\.\d+)?\s*(?:%|ms|rps|qps|req/s|requests/s|errors/s|kb|mb|gb|tb|bytes)\b`)
	// Heuristic context matcher for metric-like phrasing; intentionally broad for
	// zero-tool-results guarding and may match explanatory text in edge cases.
	metricContextPattern = regexp.MustCompile(`(?i)\b(?:p\d{2}|latency|error(?:\s+rate)?|throughput|cpu|memory|utilization)\b[^\n.]{0,40}\b\d+(?:\.\d+)?\b`)
	errorAckPattern      = regexp.MustCompile(`(?i)\b(error|failed|unable|could not|couldn't|cannot|can't|unavailable|partial|incomplete|retry|timed out|timeout)\b`)
	citationPattern      = regexp.MustCompile(`(?i)\b(source|runbook|kb|reference|according to|based on)\b`)
)

type GuardCaseInput struct {
	ID            string
	Category      string
	Message       string
	AssistantText string
	Success       bool
	Error         string
	ToolResults   int
	ToolErrors    int
	ExpectedTools []string
	UsedTools     []string
}

type GuardViolation struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type GuardCaseResult struct {
	ID         string           `json:"id"`
	Category   string           `json:"category"`
	Passed     bool             `json:"passed"`
	Violations []GuardViolation `json:"violations,omitempty"`
}

type GuardTotals struct {
	Cases            int            `json:"cases"`
	PassedCases      int            `json:"passed_cases"`
	FailedCases      int            `json:"failed_cases"`
	ViolationsTotal  int            `json:"violations_total"`
	ViolationsByRule map[string]int `json:"violations_by_rule"`
}

func EvaluateDeterministicGuards(in GuardCaseInput) GuardCaseResult {
	result := GuardCaseResult{
		ID:       in.ID,
		Category: in.Category,
		Passed:   true,
	}

	// Rule 1: if the response appears to state concrete metric values but no live data
	// was retrieved for a case that expected live data tools, fail hard.
	// Note: this intentionally does not try to validate value-level grounding when
	// ToolResults > 0; that deeper comparison requires parsing tool payload semantics.
	if in.Success && expectsLiveData(in.ExpectedTools) && in.ToolResults == 0 && hasMetricLikeClaim(in.AssistantText) {
		result.Violations = append(result.Violations, GuardViolation{
			Rule:     GuardRuleNoFabricatedMetricValues,
			Severity: "hard",
			Message:  "Assistant appears to report metric values without live data tool results.",
		})
	}

	// Rule 2: tool errors must be acknowledged clearly in the assistant response.
	if in.ToolErrors > 0 && !hasErrorAcknowledgment(in.AssistantText) {
		result.Violations = append(result.Violations, GuardViolation{
			Rule:     GuardRuleToolErrorHandling,
			Severity: "hard",
			Message:  "Tool errors occurred but assistant response does not acknowledge degraded evidence/error state.",
		})
	}

	// Rule 3: docs/runbook guidance must include an evidence signal: either docs tool use
	// or explicit source/reference language in the answer.
	if in.Success && requiresDocsEvidence(in.Category, in.ExpectedTools) && !hasDocsEvidenceSignal(in.UsedTools, in.AssistantText) {
		result.Violations = append(result.Violations, GuardViolation{
			Rule:     GuardRuleDocsEvidenceSignals,
			Severity: "hard",
			Message:  "Docs/runbook case lacks evidence signal (no KB retrieval signal and no citation/source language).",
		})
	}

	if len(result.Violations) > 0 {
		result.Passed = false
	}
	return result
}

func BuildGuardTotals(results []GuardCaseResult) GuardTotals {
	totals := GuardTotals{
		Cases:            len(results),
		PassedCases:      0,
		FailedCases:      0,
		ViolationsTotal:  0,
		ViolationsByRule: map[string]int{},
	}
	for _, r := range results {
		if r.Passed {
			totals.PassedCases++
		} else {
			totals.FailedCases++
		}
		for _, v := range r.Violations {
			totals.ViolationsTotal++
			totals.ViolationsByRule[v.Rule]++
		}
	}

	// Keep stable ordering when serialized by pre-seeding known rules.
	for _, rule := range []string{
		GuardRuleNoFabricatedMetricValues,
		GuardRuleToolErrorHandling,
		GuardRuleDocsEvidenceSignals,
	} {
		if _, ok := totals.ViolationsByRule[rule]; !ok {
			totals.ViolationsByRule[rule] = 0
		}
	}
	return totals
}

func expectsLiveData(expectedTools []string) bool {
	for _, t := range expectedTools {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" {
			continue
		}
		if strings.Contains(t, "__query_") || t == "alertmanager__list_alerts" {
			return true
		}
	}
	return false
}

func requiresDocsEvidence(category string, expectedTools []string) bool {
	if strings.EqualFold(strings.TrimSpace(category), "docs_runbook") {
		return true
	}
	for _, t := range expectedTools {
		if isDocsToolName(t) {
			return true
		}
	}
	return false
}

func hasMetricLikeClaim(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return metricValuePattern.MatchString(text) || metricContextPattern.MatchString(text)
}

func hasErrorAcknowledgment(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return errorAckPattern.MatchString(text)
}

func hasDocsEvidenceSignal(usedTools []string, assistantText string) bool {
	for _, t := range usedTools {
		if isDocsToolName(t) {
			return true
		}
	}
	return citationPattern.MatchString(assistantText)
}

func isDocsToolName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return n == "kb__search_kb" || n == "kb__search_kb_semantic"
}

// UniqueToolNamesFromTrace returns sorted unique tool names from tool trace events.
func UniqueToolNamesFromTrace(trace []map[string]any) []string {
	set := map[string]struct{}{}
	out := make([]string, 0, len(trace))
	for _, ev := range trace {
		name, _ := ev["tool"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := set[name]; ok {
			continue
		}
		set[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
