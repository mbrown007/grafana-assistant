package evals

import "testing"

func TestEvaluateQualityGatePass(t *testing.T) {
	judge := JudgeScoreReport{
		SchemaVersion: "judge-score.v1",
		Dataset:       "tests/evals/dataset.yaml",
		RunArtifact:   "tests/evals/results/baseline-123.json",
		Cases: []JudgeScoreCase{
			{},
			{},
		},
		Aggregate: JudgeAggregate{
			CaseCount: 10,
			PassRate:  0.9,
			HardFailureCounts: HardFailureCounts{
				HallucinationRisk: 0,
			},
		},
	}
	guard := GuardCheckReport{
		SchemaVersion: "guard-report.v1",
		Dataset:       "tests/evals/dataset.yaml",
		RunArtifact:   "tests/evals/results/baseline-123.json",
		Totals: GuardTotals{
			Cases:           10,
			FailedCases:     0,
			ViolationsTotal: 0,
		},
	}

	got, err := EvaluateQualityGate(judge, guard, DefaultQualityGateThresholds())
	if err != nil {
		t.Fatalf("EvaluateQualityGate() error = %v", err)
	}
	if !got.Passed {
		t.Fatalf("expected gate pass, got fail: %+v", got.Checks)
	}
}

func TestEvaluateQualityGateFail_PassRate(t *testing.T) {
	judge, guard := baselineQualityReports()
	judge.Aggregate.PassRate = 0.7

	got, err := EvaluateQualityGate(judge, guard, DefaultQualityGateThresholds())
	if err != nil {
		t.Fatalf("EvaluateQualityGate() error = %v", err)
	}
	if got.Passed {
		t.Fatal("expected gate fail due pass-rate threshold")
	}
	assertCheckFailed(t, got, QualityCheckJudgePassRate)
}

func TestEvaluateQualityGateFail_HallucinationAndGuardViolations(t *testing.T) {
	judge, guard := baselineQualityReports()
	judge.Aggregate.HardFailureCounts.HallucinationRisk = 2
	guard.Totals.FailedCases = 1
	guard.Totals.ViolationsTotal = 2

	got, err := EvaluateQualityGate(judge, guard, DefaultQualityGateThresholds())
	if err != nil {
		t.Fatalf("EvaluateQualityGate() error = %v", err)
	}
	if got.Passed {
		t.Fatal("expected gate fail for hallucination and guard checks")
	}
	assertCheckFailed(t, got, QualityCheckHallucinationHard)
	assertCheckFailed(t, got, QualityCheckGuardFailedCases)
	assertCheckFailed(t, got, QualityCheckGuardViolations)
}

func TestEvaluateQualityGateFail_JudgeErrorCases(t *testing.T) {
	judge, guard := baselineQualityReports()
	judge.Cases = []JudgeScoreCase{
		{},
		{JudgeError: "model timeout"},
	}

	got, err := EvaluateQualityGate(judge, guard, DefaultQualityGateThresholds())
	if err != nil {
		t.Fatalf("EvaluateQualityGate() error = %v", err)
	}
	if got.Passed {
		t.Fatal("expected gate fail for judge error cases")
	}
	assertCheckFailed(t, got, QualityCheckJudgeErrorCases)
}

func TestEvaluateQualityGateFail_DatasetMismatchOptional(t *testing.T) {
	judge, guard := baselineQualityReports()
	guard.Dataset = "tests/evals/other.yaml"

	thresholds := DefaultQualityGateThresholds()
	thresholds.RequireDatasetMatch = true
	got, err := EvaluateQualityGate(judge, guard, thresholds)
	if err != nil {
		t.Fatalf("EvaluateQualityGate() error = %v", err)
	}
	if got.Passed {
		t.Fatal("expected gate fail for dataset mismatch")
	}
	assertCheckFailed(t, got, QualityCheckDatasetMatch)

	thresholds.RequireDatasetMatch = false
	got, err = EvaluateQualityGate(judge, guard, thresholds)
	if err != nil {
		t.Fatalf("EvaluateQualityGate() error = %v", err)
	}
	if !got.Passed {
		t.Fatalf("expected gate pass when dataset-match check disabled, got fail: %+v", got.Checks)
	}
}

func TestEvaluateQualityGateThresholdValidation(t *testing.T) {
	judge, guard := baselineQualityReports()
	thresholds := DefaultQualityGateThresholds()
	thresholds.MinPassRate = 1.1
	if _, err := EvaluateQualityGate(judge, guard, thresholds); err == nil {
		t.Fatal("expected error for invalid min_pass_rate")
	}
}

func baselineQualityReports() (JudgeScoreReport, GuardCheckReport) {
	judge := JudgeScoreReport{
		SchemaVersion: "judge-score.v1",
		Dataset:       "tests/evals/dataset.yaml",
		RunArtifact:   "tests/evals/results/baseline-123.json",
		Cases: []JudgeScoreCase{
			{},
		},
		Aggregate: JudgeAggregate{
			CaseCount: 10,
			PassRate:  0.9,
			HardFailureCounts: HardFailureCounts{
				HallucinationRisk: 0,
			},
		},
	}
	guard := GuardCheckReport{
		SchemaVersion: "guard-report.v1",
		Dataset:       "tests/evals/dataset.yaml",
		RunArtifact:   "tests/evals/results/baseline-123.json",
		Totals: GuardTotals{
			Cases:           10,
			FailedCases:     0,
			ViolationsTotal: 0,
		},
	}
	return judge, guard
}

func assertCheckFailed(t *testing.T, result QualityGateResult, checkID string) {
	t.Helper()
	for _, c := range result.Checks {
		if c.ID != checkID {
			continue
		}
		if c.Passed {
			t.Fatalf("expected check %q to fail, got pass", checkID)
		}
		return
	}
	t.Fatalf("check %q not found", checkID)
}
