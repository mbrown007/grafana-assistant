package evals

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

const (
	DimensionFactuality           = "factuality"
	DimensionAnswerCompleteness   = "answer_completeness"
	DimensionToolUsageCorrectness = "tool_usage_correctness"
	DimensionHallucinationRisk    = "hallucination_risk"
	DimensionActionability        = "actionability"
)

var requiredDimensionIDs = []string{
	DimensionFactuality,
	DimensionAnswerCompleteness,
	DimensionToolUsageCorrectness,
	DimensionHallucinationRisk,
	DimensionActionability,
}

type Rubric struct {
	RubricID   string            `json:"rubric_id"`
	ScoreScale ScoreScale        `json:"score_scale"`
	Dimensions []RubricDimension `json:"dimensions"`
	PassRules  RubricPassRules   `json:"pass_rules"`
}

type ScoreScale struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type RubricDimension struct {
	ID                   string  `json:"id"`
	Weight               float64 `json:"weight"`
	HardFailBelowOrEqual *int    `json:"hard_fail_below_or_equal,omitempty"`
}

type RubricPassRules struct {
	WeightedScoreMin     float64        `json:"weighted_score_min"`
	RequiredDimensionMin map[string]int `json:"required_dimension_min"`
	HardFailDimensions   []string       `json:"hard_fail_dimensions"`
}

type DimensionScores struct {
	Factuality           int `json:"factuality"`
	AnswerCompleteness   int `json:"answer_completeness"`
	ToolUsageCorrectness int `json:"tool_usage_correctness"`
	HallucinationRisk    int `json:"hallucination_risk"`
	Actionability        int `json:"actionability"`
}

type Rationales struct {
	Factuality           string `json:"factuality"`
	AnswerCompleteness   string `json:"answer_completeness"`
	ToolUsageCorrectness string `json:"tool_usage_correctness"`
	HallucinationRisk    string `json:"hallucination_risk"`
	Actionability        string `json:"actionability"`
}

type DimensionAverages struct {
	Factuality           float64 `json:"factuality"`
	AnswerCompleteness   float64 `json:"answer_completeness"`
	ToolUsageCorrectness float64 `json:"tool_usage_correctness"`
	HallucinationRisk    float64 `json:"hallucination_risk"`
	Actionability        float64 `json:"actionability"`
}

type HardFailureCounts struct {
	Factuality        int `json:"factuality"`
	HallucinationRisk int `json:"hallucination_risk"`
}

type CaseScoreSummary struct {
	Verdict         string          `json:"verdict"`
	DimensionScores DimensionScores `json:"dimension_scores"`
	WeightedScore   float64         `json:"weighted_score"`
	HardFailures    []string        `json:"hard_failures"`
}

type AggregateSummary struct {
	CaseCount              int               `json:"case_count"`
	PassCount              int               `json:"pass_count"`
	FailCount              int               `json:"fail_count"`
	PassRate               float64           `json:"pass_rate"`
	AverageWeightedScore   float64           `json:"average_weighted_score"`
	AverageDimensionScores DimensionAverages `json:"average_dimension_scores"`
	HardFailureCounts      HardFailureCounts `json:"hard_failure_counts"`
}

func ValidateRubric(r Rubric) error {
	if r.RubricID == "" {
		return errors.New("rubric_id is required")
	}
	if r.ScoreScale.Max <= r.ScoreScale.Min {
		return fmt.Errorf("invalid score_scale range: min=%d max=%d", r.ScoreScale.Min, r.ScoreScale.Max)
	}
	if len(r.Dimensions) == 0 {
		return errors.New("rubric.dimensions is required")
	}

	byID := map[string]RubricDimension{}
	weightSum := 0.0
	for _, d := range r.Dimensions {
		if d.ID == "" {
			return errors.New("rubric dimension id is required")
		}
		if _, exists := byID[d.ID]; exists {
			return fmt.Errorf("duplicate rubric dimension id %q", d.ID)
		}
		if d.Weight <= 0 {
			return fmt.Errorf("dimension %q weight must be > 0", d.ID)
		}
		byID[d.ID] = d
		weightSum += d.Weight
	}

	for _, id := range requiredDimensionIDs {
		if _, ok := byID[id]; !ok {
			return fmt.Errorf("rubric missing required dimension %q", id)
		}
	}

	if math.Abs(weightSum-1.0) > 1e-6 {
		return fmt.Errorf("dimension weights must sum to 1.0, got %.6f", weightSum)
	}

	for id, minScore := range r.PassRules.RequiredDimensionMin {
		if _, ok := byID[id]; !ok {
			return fmt.Errorf("required_dimension_min references unknown dimension %q", id)
		}
		if minScore < r.ScoreScale.Min || minScore > r.ScoreScale.Max {
			return fmt.Errorf("required_dimension_min[%q] out of range", id)
		}
	}

	for _, id := range r.PassRules.HardFailDimensions {
		if _, ok := byID[id]; !ok {
			return fmt.Errorf("hard_fail_dimensions references unknown dimension %q", id)
		}
	}

	if r.PassRules.WeightedScoreMin < 0 || r.PassRules.WeightedScoreMin > 1 {
		return fmt.Errorf("weighted_score_min must be between 0 and 1, got %.4f", r.PassRules.WeightedScoreMin)
	}

	return nil
}

func (d DimensionScores) AsMap() map[string]int {
	return map[string]int{
		DimensionFactuality:           d.Factuality,
		DimensionAnswerCompleteness:   d.AnswerCompleteness,
		DimensionToolUsageCorrectness: d.ToolUsageCorrectness,
		DimensionHallucinationRisk:    d.HallucinationRisk,
		DimensionActionability:        d.Actionability,
	}
}

func EvaluateCase(r Rubric, scores DimensionScores) (CaseScoreSummary, error) {
	if err := ValidateRubric(r); err != nil {
		return CaseScoreSummary{}, err
	}

	dimensionScores := scores.AsMap()
	dimensionByID := map[string]RubricDimension{}
	for _, d := range r.Dimensions {
		dimensionByID[d.ID] = d
	}

	rangeWidth := float64(r.ScoreScale.Max - r.ScoreScale.Min)
	if rangeWidth <= 0 {
		return CaseScoreSummary{}, errors.New("invalid score range")
	}

	weighted := 0.0
	for id, raw := range dimensionScores {
		if raw < r.ScoreScale.Min || raw > r.ScoreScale.Max {
			return CaseScoreSummary{}, fmt.Errorf("dimension %q score out of range: %d", id, raw)
		}
		d, ok := dimensionByID[id]
		if !ok {
			return CaseScoreSummary{}, fmt.Errorf("dimension %q is not defined in rubric", id)
		}
		normalized := float64(raw-r.ScoreScale.Min) / rangeWidth
		weighted += d.Weight * normalized
	}

	hardFailures := make([]string, 0, len(r.PassRules.HardFailDimensions))
	for _, id := range r.PassRules.HardFailDimensions {
		score := dimensionScores[id]
		threshold := 1
		if d, ok := dimensionByID[id]; ok && d.HardFailBelowOrEqual != nil {
			threshold = *d.HardFailBelowOrEqual
		}
		if score <= threshold {
			hardFailures = append(hardFailures, id)
		}
	}

	meetsRequiredMins := true
	for id, minScore := range r.PassRules.RequiredDimensionMin {
		if dimensionScores[id] < minScore {
			meetsRequiredMins = false
			break
		}
	}

	verdict := "fail"
	if len(hardFailures) == 0 && meetsRequiredMins && weighted >= r.PassRules.WeightedScoreMin {
		verdict = "pass"
	}

	sort.Strings(hardFailures)
	return CaseScoreSummary{
		Verdict:         verdict,
		DimensionScores: scores,
		WeightedScore:   roundFloat(weighted, 6),
		HardFailures:    hardFailures,
	}, nil
}

func BuildAggregate(cases []CaseScoreSummary) AggregateSummary {
	out := AggregateSummary{
		CaseCount: len(cases),
		FailCount: len(cases),
	}
	if len(cases) == 0 {
		return out
	}

	var (
		weightedSum             float64
		factualitySum           int
		answerCompletenessSum   int
		toolUsageCorrectnessSum int
		hallucinationRiskSum    int
		actionabilitySum        int
	)

	for _, c := range cases {
		if c.Verdict == "pass" {
			out.PassCount++
			out.FailCount--
		}
		weightedSum += c.WeightedScore

		factualitySum += c.DimensionScores.Factuality
		answerCompletenessSum += c.DimensionScores.AnswerCompleteness
		toolUsageCorrectnessSum += c.DimensionScores.ToolUsageCorrectness
		hallucinationRiskSum += c.DimensionScores.HallucinationRisk
		actionabilitySum += c.DimensionScores.Actionability

		for _, hf := range c.HardFailures {
			switch hf {
			case DimensionFactuality:
				out.HardFailureCounts.Factuality++
			case DimensionHallucinationRisk:
				out.HardFailureCounts.HallucinationRisk++
			}
		}
	}

	n := float64(len(cases))
	out.PassRate = roundFloat(float64(out.PassCount)/n, 6)
	out.AverageWeightedScore = roundFloat(weightedSum/n, 6)
	out.AverageDimensionScores = DimensionAverages{
		Factuality:           roundFloat(float64(factualitySum)/n, 6),
		AnswerCompleteness:   roundFloat(float64(answerCompletenessSum)/n, 6),
		ToolUsageCorrectness: roundFloat(float64(toolUsageCorrectnessSum)/n, 6),
		HallucinationRisk:    roundFloat(float64(hallucinationRiskSum)/n, 6),
		Actionability:        roundFloat(float64(actionabilitySum)/n, 6),
	}
	return out
}

func roundFloat(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}
