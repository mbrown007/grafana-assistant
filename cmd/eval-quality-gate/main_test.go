package main

import (
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/evals"
)

func TestRenderMarkdown(t *testing.T) {
	report := qualityGateReport{
		SchemaVersion: "quality-gate.v1",
		GeneratedAt:   "2026-02-08T00:00:00Z",
		JudgeReport:   "judge.json",
		GuardReport:   "guard.json",
		Result: evals.QualityGateResult{
			Passed: false,
			Metrics: evals.QualityGateMetrics{
				JudgeCaseCount:                 10,
				JudgePassRate:                  0.8,
				JudgeHallucinationHardFailures: 1,
				JudgeErrorCases:                0,
				GuardFailedCases:               1,
				GuardViolationsTotal:           1,
			},
			Checks: []evals.QualityGateCheck{
				{ID: "judge_pass_rate", Passed: false, Actual: "0.8", Threshold: ">=0.85", Message: "pass-rate threshold"},
				{ID: "guard_failed_cases", Passed: true, Actual: "0", Threshold: "<=0", Message: "guard failed-case threshold"},
			},
		},
	}

	md := renderMarkdown(report)
	for _, expected := range []string{
		"## Eval Quality Gate",
		"Status: **FAIL**",
		"[FAIL] `judge_pass_rate`",
		"[PASS] `guard_failed_cases`",
	} {
		if !strings.Contains(md, expected) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", expected, md)
		}
	}
}

func TestCountFailedChecks(t *testing.T) {
	checks := []evals.QualityGateCheck{
		{ID: "a", Passed: true},
		{ID: "b", Passed: false},
		{ID: "c", Passed: false},
	}
	if got := countFailedChecks(checks); got != 2 {
		t.Fatalf("countFailedChecks() = %d, want 2", got)
	}
}

func TestParseFeatureFlagEnv(t *testing.T) {
	t.Setenv("ASSISTANT_FEATURE_JUDGE_GATE_MODE", "disabled")
	if got := parseFeatureFlagEnv("ASSISTANT_FEATURE_JUDGE_GATE_MODE", true); got {
		t.Fatal("expected disabled env value to parse as false")
	}

	t.Setenv("ASSISTANT_FEATURE_JUDGE_GATE_MODE", "enabled")
	if got := parseFeatureFlagEnv("ASSISTANT_FEATURE_JUDGE_GATE_MODE", false); !got {
		t.Fatal("expected enabled env value to parse as true")
	}

	t.Setenv("ASSISTANT_FEATURE_JUDGE_GATE_MODE", "")
	if got := parseFeatureFlagEnv("ASSISTANT_FEATURE_JUDGE_GATE_MODE", true); !got {
		t.Fatal("expected default value when env var is empty")
	}
}
