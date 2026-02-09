package evals

import "testing"

func testRubric() Rubric {
	hardFailThreshold := 1
	return Rubric{
		RubricID: "judge-rubric.v1",
		ScoreScale: ScoreScale{
			Min: 0,
			Max: 4,
		},
		Dimensions: []RubricDimension{
			{ID: DimensionFactuality, Weight: 0.30, HardFailBelowOrEqual: &hardFailThreshold},
			{ID: DimensionAnswerCompleteness, Weight: 0.25},
			{ID: DimensionToolUsageCorrectness, Weight: 0.20},
			{ID: DimensionHallucinationRisk, Weight: 0.15, HardFailBelowOrEqual: &hardFailThreshold},
			{ID: DimensionActionability, Weight: 0.10},
		},
		PassRules: RubricPassRules{
			WeightedScoreMin: 0.75,
			RequiredDimensionMin: map[string]int{
				DimensionFactuality:         2,
				DimensionAnswerCompleteness: 2,
				DimensionHallucinationRisk:  2,
			},
			HardFailDimensions: []string{
				DimensionFactuality,
				DimensionHallucinationRisk,
			},
		},
	}
}

func TestValidateRubric(t *testing.T) {
	r := testRubric()
	if err := ValidateRubric(r); err != nil {
		t.Fatalf("expected rubric to be valid, got error: %v", err)
	}
}

func TestEvaluateCasePass(t *testing.T) {
	r := testRubric()
	scores := DimensionScores{
		Factuality:           4,
		AnswerCompleteness:   3,
		ToolUsageCorrectness: 3,
		HallucinationRisk:    4,
		Actionability:        3,
	}

	got, err := EvaluateCase(r, scores)
	if err != nil {
		t.Fatalf("EvaluateCase() error = %v", err)
	}
	if got.Verdict != "pass" {
		t.Fatalf("expected verdict=pass, got %q", got.Verdict)
	}
	if len(got.HardFailures) != 0 {
		t.Fatalf("expected no hard failures, got %v", got.HardFailures)
	}
	if got.WeightedScore < 0.75 {
		t.Fatalf("expected weighted score >= 0.75, got %.6f", got.WeightedScore)
	}
}

func TestEvaluateCaseHardFail(t *testing.T) {
	r := testRubric()
	scores := DimensionScores{
		Factuality:           1,
		AnswerCompleteness:   4,
		ToolUsageCorrectness: 4,
		HallucinationRisk:    4,
		Actionability:        4,
	}

	got, err := EvaluateCase(r, scores)
	if err != nil {
		t.Fatalf("EvaluateCase() error = %v", err)
	}
	if got.Verdict != "fail" {
		t.Fatalf("expected verdict=fail, got %q", got.Verdict)
	}
	if len(got.HardFailures) != 1 || got.HardFailures[0] != DimensionFactuality {
		t.Fatalf("expected factuality hard failure, got %v", got.HardFailures)
	}
}

func TestEvaluateCaseRequiredMinimumFail(t *testing.T) {
	r := testRubric()
	scores := DimensionScores{
		Factuality:           4,
		AnswerCompleteness:   1, // below required minimum (2)
		ToolUsageCorrectness: 4,
		HallucinationRisk:    4,
		Actionability:        4,
	}

	got, err := EvaluateCase(r, scores)
	if err != nil {
		t.Fatalf("EvaluateCase() error = %v", err)
	}
	if got.WeightedScore < 0.75 {
		t.Fatalf("test setup invalid: expected weighted score >= 0.75, got %.6f", got.WeightedScore)
	}
	if len(got.HardFailures) != 0 {
		t.Fatalf("expected no hard failures, got %v", got.HardFailures)
	}
	if got.Verdict != "fail" {
		t.Fatalf("expected verdict=fail due to required minimum rule, got %q", got.Verdict)
	}
}

func TestBuildAggregate(t *testing.T) {
	cases := []CaseScoreSummary{
		{
			Verdict: "pass",
			DimensionScores: DimensionScores{
				Factuality:           4,
				AnswerCompleteness:   3,
				ToolUsageCorrectness: 3,
				HallucinationRisk:    4,
				Actionability:        3,
			},
			WeightedScore: 0.85,
			HardFailures:  nil,
		},
		{
			Verdict: "fail",
			DimensionScores: DimensionScores{
				Factuality:           1,
				AnswerCompleteness:   2,
				ToolUsageCorrectness: 2,
				HallucinationRisk:    2,
				Actionability:        2,
			},
			WeightedScore: 0.40,
			HardFailures:  []string{DimensionFactuality},
		},
	}

	got := BuildAggregate(cases)
	if got.CaseCount != 2 || got.PassCount != 1 || got.FailCount != 1 {
		t.Fatalf("unexpected aggregate counts: %+v", got)
	}
	if got.HardFailureCounts.Factuality != 1 {
		t.Fatalf("expected factuality hard failure count = 1, got %d", got.HardFailureCounts.Factuality)
	}
	if got.PassRate != 0.5 {
		t.Fatalf("expected pass_rate=0.5, got %.6f", got.PassRate)
	}
	if got.AverageWeightedScore != 0.625 {
		t.Fatalf("expected average_weighted_score=0.625, got %.6f", got.AverageWeightedScore)
	}
}
