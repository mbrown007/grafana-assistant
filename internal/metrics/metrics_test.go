package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestPromptBudgetMetricsRegistered(t *testing.T) {
	RequestBudgetTripsTotal.WithLabelValues("registered")
	PromptCharsByIntent.WithLabelValues("registered").Observe(0)
	PromptToolCountByIntent.WithLabelValues("registered").Observe(0)
	SubAgentInvocationsTotal.WithLabelValues("registered").Inc()
	SubAgentDurationSeconds.WithLabelValues("registered").Observe(0.1)
	SubAgentTokensUsed.WithLabelValues("registered").Observe(120)
	SubAgentErrorsTotal.WithLabelValues("registered").Inc()

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	if !containsMetric(mfs, "assistant_prompt_chars") {
		t.Fatalf("expected assistant_prompt_chars metric to be registered")
	}
	if !containsMetric(mfs, "assistant_prompt_chars_by_intent") {
		t.Fatalf("expected assistant_prompt_chars_by_intent metric to be registered")
	}
	if !containsMetric(mfs, "assistant_prompt_tool_count") {
		t.Fatalf("expected assistant_prompt_tool_count metric to be registered")
	}
	if !containsMetric(mfs, "assistant_prompt_tool_count_by_intent") {
		t.Fatalf("expected assistant_prompt_tool_count_by_intent metric to be registered")
	}
	if !containsMetric(mfs, "assistant_prompt_tool_count_filtered") {
		t.Fatalf("expected assistant_prompt_tool_count_filtered metric to be registered")
	}
	if !containsMetric(mfs, "assistant_request_budget_trips_total") {
		t.Fatalf("expected assistant_request_budget_trips_total metric to be registered")
	}
	if !containsMetric(mfs, "assistant_request_budget_estimated_cost_usd") {
		t.Fatalf("expected assistant_request_budget_estimated_cost_usd metric to be registered")
	}
	if !containsMetric(mfs, "assistant_subagent_invocations_total") {
		t.Fatalf("expected assistant_subagent_invocations_total metric to be registered")
	}
	if !containsMetric(mfs, "assistant_subagent_duration_seconds") {
		t.Fatalf("expected assistant_subagent_duration_seconds metric to be registered")
	}
	if !containsMetric(mfs, "assistant_subagent_tokens_used") {
		t.Fatalf("expected assistant_subagent_tokens_used metric to be registered")
	}
	if !containsMetric(mfs, "assistant_subagent_errors_total") {
		t.Fatalf("expected assistant_subagent_errors_total metric to be registered")
	}
}

func TestPromptBudgetMetricsObserveSamples(t *testing.T) {
	before, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather before observe: %v", err)
	}
	beforeChars := histogramSampleCount(before, "assistant_prompt_chars")
	beforeCharsByIntent := histogramSampleCountWithLabels(before, "assistant_prompt_chars_by_intent", map[string]string{"intent": "how_to_docs"})
	beforeTools := histogramSampleCount(before, "assistant_prompt_tool_count")
	beforeToolsByIntent := histogramSampleCountWithLabels(before, "assistant_prompt_tool_count_by_intent", map[string]string{"intent": "how_to_docs"})
	beforeFilteredTools := histogramSampleCount(before, "assistant_prompt_tool_count_filtered")

	PromptChars.Observe(1024)
	PromptCharsByIntent.WithLabelValues("how_to_docs").Observe(640)
	PromptToolCount.Observe(7)
	PromptToolCountByIntent.WithLabelValues("how_to_docs").Observe(2)
	PromptToolCountFiltered.Observe(3)

	after, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather after observe: %v", err)
	}
	afterChars := histogramSampleCount(after, "assistant_prompt_chars")
	afterCharsByIntent := histogramSampleCountWithLabels(after, "assistant_prompt_chars_by_intent", map[string]string{"intent": "how_to_docs"})
	afterTools := histogramSampleCount(after, "assistant_prompt_tool_count")
	afterToolsByIntent := histogramSampleCountWithLabels(after, "assistant_prompt_tool_count_by_intent", map[string]string{"intent": "how_to_docs"})
	afterFilteredTools := histogramSampleCount(after, "assistant_prompt_tool_count_filtered")

	if afterChars <= beforeChars {
		t.Fatalf("assistant_prompt_chars sample count did not increase: before=%d after=%d", beforeChars, afterChars)
	}
	if afterCharsByIntent <= beforeCharsByIntent {
		t.Fatalf("assistant_prompt_chars_by_intent sample count did not increase: before=%d after=%d", beforeCharsByIntent, afterCharsByIntent)
	}
	if afterTools <= beforeTools {
		t.Fatalf("assistant_prompt_tool_count sample count did not increase: before=%d after=%d", beforeTools, afterTools)
	}
	if afterToolsByIntent <= beforeToolsByIntent {
		t.Fatalf("assistant_prompt_tool_count_by_intent sample count did not increase: before=%d after=%d", beforeToolsByIntent, afterToolsByIntent)
	}
	if afterFilteredTools <= beforeFilteredTools {
		t.Fatalf("assistant_prompt_tool_count_filtered sample count did not increase: before=%d after=%d", beforeFilteredTools, afterFilteredTools)
	}
}

func TestRequestBudgetMetricsObserveSamples(t *testing.T) {
	before, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather before observe: %v", err)
	}
	beforeCost := histogramSampleCount(before, "assistant_request_budget_estimated_cost_usd")
	beforeTrips := counterSampleValue(before, "assistant_request_budget_trips_total", map[string]string{"reason": "tool_calls_exceeded"})

	RequestBudgetEstimatedCostUSD.Observe(0.012)
	RequestBudgetTripsTotal.WithLabelValues("tool_calls_exceeded").Inc()

	after, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather after observe: %v", err)
	}
	afterCost := histogramSampleCount(after, "assistant_request_budget_estimated_cost_usd")
	afterTrips := counterSampleValue(after, "assistant_request_budget_trips_total", map[string]string{"reason": "tool_calls_exceeded"})

	if afterCost <= beforeCost {
		t.Fatalf("assistant_request_budget_estimated_cost_usd sample count did not increase: before=%d after=%d", beforeCost, afterCost)
	}
	if afterTrips <= beforeTrips {
		t.Fatalf("assistant_request_budget_trips_total did not increase for reason label: before=%f after=%f", beforeTrips, afterTrips)
	}
}

func TestSubAgentMetricsObserveSamples(t *testing.T) {
	before, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather before observe: %v", err)
	}
	beforeInvocations := counterSampleValue(before, "assistant_subagent_invocations_total", map[string]string{"agent_name": "dashboard"})
	beforeErrors := counterSampleValue(before, "assistant_subagent_errors_total", map[string]string{"agent_name": "dashboard"})
	beforeDuration := histogramSampleCountWithLabels(before, "assistant_subagent_duration_seconds", map[string]string{"agent_name": "dashboard"})
	beforeTokens := histogramSampleCountWithLabels(before, "assistant_subagent_tokens_used", map[string]string{"agent_name": "dashboard"})

	SubAgentInvocationsTotal.WithLabelValues("dashboard").Inc()
	SubAgentErrorsTotal.WithLabelValues("dashboard").Inc()
	SubAgentDurationSeconds.WithLabelValues("dashboard").Observe(1.3)
	SubAgentTokensUsed.WithLabelValues("dashboard").Observe(850)

	after, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather after observe: %v", err)
	}
	afterInvocations := counterSampleValue(after, "assistant_subagent_invocations_total", map[string]string{"agent_name": "dashboard"})
	afterErrors := counterSampleValue(after, "assistant_subagent_errors_total", map[string]string{"agent_name": "dashboard"})
	afterDuration := histogramSampleCountWithLabels(after, "assistant_subagent_duration_seconds", map[string]string{"agent_name": "dashboard"})
	afterTokens := histogramSampleCountWithLabels(after, "assistant_subagent_tokens_used", map[string]string{"agent_name": "dashboard"})

	if afterInvocations <= beforeInvocations {
		t.Fatalf("assistant_subagent_invocations_total did not increase: before=%f after=%f", beforeInvocations, afterInvocations)
	}
	if afterErrors <= beforeErrors {
		t.Fatalf("assistant_subagent_errors_total did not increase: before=%f after=%f", beforeErrors, afterErrors)
	}
	if afterDuration <= beforeDuration {
		t.Fatalf("assistant_subagent_duration_seconds sample count did not increase: before=%d after=%d", beforeDuration, afterDuration)
	}
	if afterTokens <= beforeTokens {
		t.Fatalf("assistant_subagent_tokens_used sample count did not increase: before=%d after=%d", beforeTokens, afterTokens)
	}
}

func containsMetric(metricFamilies []*dto.MetricFamily, name string) bool {
	for _, mf := range metricFamilies {
		if mf.GetName() == name {
			return true
		}
	}
	return false
}

func histogramSampleCount(metricFamilies []*dto.MetricFamily, name string) uint64 {
	for _, mf := range metricFamilies {
		if mf.GetName() != name {
			continue
		}
		if len(mf.GetMetric()) == 0 || mf.GetMetric()[0].GetHistogram() == nil {
			return 0
		}
		return mf.GetMetric()[0].GetHistogram().GetSampleCount()
	}
	return 0
}

func histogramSampleCountWithLabels(metricFamilies []*dto.MetricFamily, name string, labels map[string]string) uint64 {
	for _, mf := range metricFamilies {
		if mf.GetName() != name {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if metric.GetHistogram() == nil || !metricLabelsMatch(metric.GetLabel(), labels) {
				continue
			}
			return metric.GetHistogram().GetSampleCount()
		}
	}
	return 0
}

func counterSampleValue(metricFamilies []*dto.MetricFamily, name string, labels map[string]string) float64 {
	for _, mf := range metricFamilies {
		if mf.GetName() != name {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if metric.GetCounter() == nil || !metricLabelsMatch(metric.GetLabel(), labels) {
				continue
			}
			return metric.GetCounter().GetValue()
		}
	}
	return 0
}

func metricLabelsMatch(pairs []*dto.LabelPair, labels map[string]string) bool {
	for key, expected := range labels {
		matched := false
		for _, pair := range pairs {
			if pair.GetName() == key && pair.GetValue() == expected {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
