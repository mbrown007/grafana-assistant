package evals

import (
	"fmt"
	"strings"
)

const (
	QualityCheckJudgeSchemaVersion = "judge_schema_version"
	QualityCheckGuardSchemaVersion = "guard_schema_version"
	QualityCheckMinCaseCount       = "min_case_count"
	QualityCheckJudgePassRate      = "judge_pass_rate"
	QualityCheckHallucinationHard  = "hallucination_hard_failures"
	QualityCheckJudgeErrorCases    = "judge_error_cases"
	QualityCheckGuardFailedCases   = "guard_failed_cases"
	QualityCheckGuardViolations    = "guard_violations_total"
	QualityCheckDatasetMatch       = "dataset_match"
	QualityCheckRunArtifactMatch   = "run_artifact_match"
	QualityCheckCaseCountMatch     = "case_count_match"
)

type JudgeScoreReport struct {
	SchemaVersion string           `json:"schema_version"`
	Dataset       string           `json:"dataset"`
	RunArtifact   string           `json:"run_artifact"`
	Cases         []JudgeScoreCase `json:"cases"`
	Aggregate     JudgeAggregate   `json:"aggregate"`
}

type JudgeScoreCase struct {
	JudgeError string `json:"judge_error,omitempty"`
}

type JudgeAggregate struct {
	CaseCount         int               `json:"case_count"`
	PassRate          float64           `json:"pass_rate"`
	HardFailureCounts HardFailureCounts `json:"hard_failure_counts"`
}

type GuardCheckReport struct {
	SchemaVersion string      `json:"schema_version"`
	Dataset       string      `json:"dataset"`
	RunArtifact   string      `json:"run_artifact"`
	Totals        GuardTotals `json:"totals"`
}

type QualityGateThresholds struct {
	MinPassRate                  float64 `json:"min_pass_rate"`
	MinCaseCount                 int     `json:"min_case_count"`
	MaxHallucinationHardFailures int     `json:"max_hallucination_hard_failures"`
	MaxJudgeErrorCases           int     `json:"max_judge_error_cases"`
	MaxGuardFailedCases          int     `json:"max_guard_failed_cases"`
	MaxGuardViolationsTotal      int     `json:"max_guard_violations_total"`
	RequireDatasetMatch          bool    `json:"require_dataset_match"`
	RequireRunArtifactMatch      bool    `json:"require_run_artifact_match"`
	RequireCaseCountMatch        bool    `json:"require_case_count_match"`
}

type QualityGateMetrics struct {
	JudgeCaseCount                 int     `json:"judge_case_count"`
	JudgePassRate                  float64 `json:"judge_pass_rate"`
	JudgeHallucinationHardFailures int     `json:"judge_hallucination_hard_failures"`
	JudgeErrorCases                int     `json:"judge_error_cases"`
	GuardCaseCount                 int     `json:"guard_case_count"`
	GuardFailedCases               int     `json:"guard_failed_cases"`
	GuardViolationsTotal           int     `json:"guard_violations_total"`
	DatasetMatch                   bool    `json:"dataset_match"`
	RunArtifactMatch               bool    `json:"run_artifact_match"`
	CaseCountMatch                 bool    `json:"case_count_match"`
}

type QualityGateCheck struct {
	ID        string `json:"id"`
	Passed    bool   `json:"passed"`
	Actual    string `json:"actual"`
	Threshold string `json:"threshold"`
	Message   string `json:"message"`
}

type QualityGateResult struct {
	Passed     bool                  `json:"passed"`
	Thresholds QualityGateThresholds `json:"thresholds"`
	Metrics    QualityGateMetrics    `json:"metrics"`
	Checks     []QualityGateCheck    `json:"checks"`
}

func DefaultQualityGateThresholds() QualityGateThresholds {
	return QualityGateThresholds{
		MinPassRate:                  0.85,
		MinCaseCount:                 1,
		MaxHallucinationHardFailures: 0,
		MaxJudgeErrorCases:           0,
		MaxGuardFailedCases:          0,
		MaxGuardViolationsTotal:      0,
		RequireDatasetMatch:          true,
		RequireRunArtifactMatch:      true,
		RequireCaseCountMatch:        true,
	}
}

func EvaluateQualityGate(judge JudgeScoreReport, guard GuardCheckReport, thresholds QualityGateThresholds) (QualityGateResult, error) {
	if thresholds.MinPassRate < 0 || thresholds.MinPassRate > 1 {
		return QualityGateResult{}, fmt.Errorf("min_pass_rate must be between 0 and 1, got %.6f", thresholds.MinPassRate)
	}
	if thresholds.MinCaseCount < 1 {
		return QualityGateResult{}, fmt.Errorf("min_case_count must be >= 1, got %d", thresholds.MinCaseCount)
	}
	if thresholds.MaxHallucinationHardFailures < 0 {
		return QualityGateResult{}, fmt.Errorf("max_hallucination_hard_failures must be >= 0, got %d", thresholds.MaxHallucinationHardFailures)
	}
	if thresholds.MaxJudgeErrorCases < 0 {
		return QualityGateResult{}, fmt.Errorf("max_judge_error_cases must be >= 0, got %d", thresholds.MaxJudgeErrorCases)
	}
	if thresholds.MaxGuardFailedCases < 0 {
		return QualityGateResult{}, fmt.Errorf("max_guard_failed_cases must be >= 0, got %d", thresholds.MaxGuardFailedCases)
	}
	if thresholds.MaxGuardViolationsTotal < 0 {
		return QualityGateResult{}, fmt.Errorf("max_guard_violations_total must be >= 0, got %d", thresholds.MaxGuardViolationsTotal)
	}

	metrics := QualityGateMetrics{
		JudgeCaseCount:                 judge.Aggregate.CaseCount,
		JudgePassRate:                  judge.Aggregate.PassRate,
		JudgeHallucinationHardFailures: judge.Aggregate.HardFailureCounts.HallucinationRisk,
		JudgeErrorCases:                countJudgeErrorCases(judge.Cases),
		GuardCaseCount:                 guard.Totals.Cases,
		GuardFailedCases:               guard.Totals.FailedCases,
		GuardViolationsTotal:           guard.Totals.ViolationsTotal,
		DatasetMatch:                   strings.TrimSpace(judge.Dataset) == strings.TrimSpace(guard.Dataset),
		RunArtifactMatch:               strings.TrimSpace(judge.RunArtifact) == strings.TrimSpace(guard.RunArtifact),
		CaseCountMatch:                 judge.Aggregate.CaseCount == guard.Totals.Cases,
	}

	checks := make([]QualityGateCheck, 0, 11)
	passed := true

	addCheck := func(id string, ok bool, actual, threshold, message string) {
		checks = append(checks, QualityGateCheck{
			ID:        id,
			Passed:    ok,
			Actual:    actual,
			Threshold: threshold,
			Message:   message,
		})
		if !ok {
			passed = false
		}
	}

	addCheck(
		QualityCheckJudgeSchemaVersion,
		strings.TrimSpace(judge.SchemaVersion) == JudgeScoreSchemaVersion,
		strings.TrimSpace(judge.SchemaVersion),
		JudgeScoreSchemaVersion,
		"Judge report must use the expected judge schema version.",
	)
	addCheck(
		QualityCheckGuardSchemaVersion,
		strings.TrimSpace(guard.SchemaVersion) == GuardReportSchemaVersion,
		strings.TrimSpace(guard.SchemaVersion),
		GuardReportSchemaVersion,
		"Guard report must use the expected guard schema version.",
	)

	addCheck(
		QualityCheckMinCaseCount,
		metrics.JudgeCaseCount >= thresholds.MinCaseCount,
		fmt.Sprintf("%d", metrics.JudgeCaseCount),
		fmt.Sprintf(">=%d", thresholds.MinCaseCount),
		"Judge report must contain at least the minimum evaluated case count.",
	)
	addCheck(
		QualityCheckJudgePassRate,
		metrics.JudgePassRate >= thresholds.MinPassRate,
		fmt.Sprintf("%.6f", metrics.JudgePassRate),
		fmt.Sprintf(">=%.6f", thresholds.MinPassRate),
		"Judge pass rate must meet or exceed threshold.",
	)
	addCheck(
		QualityCheckHallucinationHard,
		metrics.JudgeHallucinationHardFailures <= thresholds.MaxHallucinationHardFailures,
		fmt.Sprintf("%d", metrics.JudgeHallucinationHardFailures),
		fmt.Sprintf("<=%d", thresholds.MaxHallucinationHardFailures),
		"Hallucination hard-failure count must stay at or below threshold.",
	)
	addCheck(
		QualityCheckJudgeErrorCases,
		metrics.JudgeErrorCases <= thresholds.MaxJudgeErrorCases,
		fmt.Sprintf("%d", metrics.JudgeErrorCases),
		fmt.Sprintf("<=%d", thresholds.MaxJudgeErrorCases),
		"Judge case-level errors must stay at or below threshold.",
	)
	addCheck(
		QualityCheckGuardFailedCases,
		metrics.GuardFailedCases <= thresholds.MaxGuardFailedCases,
		fmt.Sprintf("%d", metrics.GuardFailedCases),
		fmt.Sprintf("<=%d", thresholds.MaxGuardFailedCases),
		"Guard failed-case count must stay at or below threshold.",
	)
	addCheck(
		QualityCheckGuardViolations,
		metrics.GuardViolationsTotal <= thresholds.MaxGuardViolationsTotal,
		fmt.Sprintf("%d", metrics.GuardViolationsTotal),
		fmt.Sprintf("<=%d", thresholds.MaxGuardViolationsTotal),
		"Total deterministic guard violations must stay at or below threshold.",
	)

	if thresholds.RequireDatasetMatch {
		addCheck(
			QualityCheckDatasetMatch,
			metrics.DatasetMatch,
			fmt.Sprintf("judge=%q guard=%q", strings.TrimSpace(judge.Dataset), strings.TrimSpace(guard.Dataset)),
			"equal",
			"Judge and guard reports must reference the same dataset.",
		)
	}
	if thresholds.RequireRunArtifactMatch {
		addCheck(
			QualityCheckRunArtifactMatch,
			metrics.RunArtifactMatch,
			fmt.Sprintf("judge=%q guard=%q", strings.TrimSpace(judge.RunArtifact), strings.TrimSpace(guard.RunArtifact)),
			"equal",
			"Judge and guard reports must reference the same baseline run artifact.",
		)
	}
	if thresholds.RequireCaseCountMatch {
		addCheck(
			QualityCheckCaseCountMatch,
			metrics.CaseCountMatch,
			fmt.Sprintf("judge=%d guard=%d", metrics.JudgeCaseCount, metrics.GuardCaseCount),
			"equal",
			"Judge and guard reports must cover the same number of cases.",
		)
	}

	return QualityGateResult{
		Passed:     passed,
		Thresholds: thresholds,
		Metrics:    metrics,
		Checks:     checks,
	}, nil
}

func countJudgeErrorCases(cases []JudgeScoreCase) int {
	out := 0
	for _, c := range cases {
		if strings.TrimSpace(c.JudgeError) != "" {
			out++
		}
	}
	return out
}
