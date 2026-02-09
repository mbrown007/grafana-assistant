package evals

import "testing"

func TestGuardNoFabricatedMetricValuesWithoutData(t *testing.T) {
	in := GuardCaseInput{
		ID:            "ANO-001",
		Category:      "anomaly_detection",
		AssistantText: "P95 latency is 320ms and error rate is 2.4%.",
		Success:       true,
		ToolResults:   0,
		ExpectedTools: []string{"grafana__query_prometheus"},
	}

	got := EvaluateDeterministicGuards(in)
	if got.Passed {
		t.Fatalf("expected guard failure, got pass")
	}
	if len(got.Violations) != 1 || got.Violations[0].Rule != GuardRuleNoFabricatedMetricValues {
		t.Fatalf("unexpected violations: %+v", got.Violations)
	}
}

func TestGuardNoFabricatedMetricValues_WithDataPass(t *testing.T) {
	in := GuardCaseInput{
		ID:            "ANO-002",
		Category:      "anomaly_detection",
		AssistantText: "P95 latency is 280ms.",
		Success:       true,
		ToolResults:   2,
		ExpectedTools: []string{"grafana__query_prometheus"},
	}

	got := EvaluateDeterministicGuards(in)
	if !got.Passed {
		t.Fatalf("expected guard pass, got violations: %+v", got.Violations)
	}
}

func TestGuardToolErrorsMustBeAcknowledged(t *testing.T) {
	in := GuardCaseInput{
		ID:            "INC-001",
		Category:      "incident_response",
		AssistantText: "Service appears healthy overall.",
		Success:       true,
		ToolErrors:    1,
		ExpectedTools: []string{"grafana__query_prometheus"},
	}

	got := EvaluateDeterministicGuards(in)
	if got.Passed {
		t.Fatalf("expected guard failure, got pass")
	}
	if len(got.Violations) != 1 || got.Violations[0].Rule != GuardRuleToolErrorHandling {
		t.Fatalf("unexpected violations: %+v", got.Violations)
	}
}

func TestGuardDocsEvidenceSignalRequired(t *testing.T) {
	in := GuardCaseInput{
		ID:            "DOC-001",
		Category:      "docs_runbook",
		AssistantText: "Start by checking queues, then inspect related alerts.",
		Success:       true,
		ExpectedTools: []string{"kb__search_kb"},
		UsedTools:     []string{},
	}

	got := EvaluateDeterministicGuards(in)
	if got.Passed {
		t.Fatalf("expected guard failure, got pass")
	}
	if len(got.Violations) != 1 || got.Violations[0].Rule != GuardRuleDocsEvidenceSignals {
		t.Fatalf("unexpected violations: %+v", got.Violations)
	}
}

func TestGuardDocsEvidenceSignalPassWithKBTool(t *testing.T) {
	in := GuardCaseInput{
		ID:            "DOC-002",
		Category:      "docs_runbook",
		AssistantText: "Investigate queue growth per the runbook.",
		Success:       true,
		ExpectedTools: []string{"kb__search_kb"},
		UsedTools:     []string{"kb__search_kb_semantic"},
	}

	got := EvaluateDeterministicGuards(in)
	if !got.Passed {
		t.Fatalf("expected guard pass, got violations: %+v", got.Violations)
	}
}

func TestGuardDocsEvidenceSignalPassWithCitationText(t *testing.T) {
	in := GuardCaseInput{
		ID:            "DOC-003",
		Category:      "docs_runbook",
		AssistantText: "According to the runbook, restart the queue workers first.",
		Success:       true,
		ExpectedTools: []string{"kb__search_kb"},
		UsedTools:     []string{},
	}

	got := EvaluateDeterministicGuards(in)
	if !got.Passed {
		t.Fatalf("expected guard pass, got violations: %+v", got.Violations)
	}
}

func TestBuildGuardTotals(t *testing.T) {
	results := []GuardCaseResult{
		{
			ID:       "a",
			Category: "x",
			Passed:   true,
		},
		{
			ID:       "b",
			Category: "y",
			Passed:   false,
			Violations: []GuardViolation{
				{Rule: GuardRuleToolErrorHandling, Severity: "hard", Message: "tool error not acknowledged"},
				{Rule: GuardRuleDocsEvidenceSignals, Severity: "hard", Message: "missing docs evidence"},
			},
		},
	}

	totals := BuildGuardTotals(results)
	if totals.Cases != 2 || totals.PassedCases != 1 || totals.FailedCases != 1 {
		t.Fatalf("unexpected totals: %+v", totals)
	}
	if totals.ViolationsTotal != 2 {
		t.Fatalf("expected violations_total=2, got %d", totals.ViolationsTotal)
	}
	if totals.ViolationsByRule[GuardRuleToolErrorHandling] != 1 {
		t.Fatalf("expected tool error rule count=1, got %d", totals.ViolationsByRule[GuardRuleToolErrorHandling])
	}
}

func TestUniqueToolNamesFromTrace(t *testing.T) {
	trace := []map[string]any{
		{"tool": "grafana__query_prometheus"},
		{"tool": "kb__search_kb"},
		{"tool": "grafana__query_prometheus"},
		{"tool": ""},
		{"tool": "   "},
		{"no_tool": "ignored"},
	}

	got := UniqueToolNamesFromTrace(trace)
	if len(got) != 2 {
		t.Fatalf("expected 2 tool names, got %d (%v)", len(got), got)
	}
	if got[0] != "grafana__query_prometheus" || got[1] != "kb__search_kb" {
		t.Fatalf("unexpected tool names: %v", got)
	}
}
