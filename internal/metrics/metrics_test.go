package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestPromptBudgetMetricsRegistered(t *testing.T) {
	RequestBudgetTripsTotal.WithLabelValues("registered")

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	if !containsMetric(mfs, "assistant_prompt_chars") {
		t.Fatalf("expected assistant_prompt_chars metric to be registered")
	}
	if !containsMetric(mfs, "assistant_prompt_tool_count") {
		t.Fatalf("expected assistant_prompt_tool_count metric to be registered")
	}
	if !containsMetric(mfs, "assistant_request_budget_trips_total") {
		t.Fatalf("expected assistant_request_budget_trips_total metric to be registered")
	}
	if !containsMetric(mfs, "assistant_request_budget_estimated_cost_usd") {
		t.Fatalf("expected assistant_request_budget_estimated_cost_usd metric to be registered")
	}
}

func TestPromptBudgetMetricsObserveSamples(t *testing.T) {
	before, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather before observe: %v", err)
	}
	beforeChars := histogramSampleCount(before, "assistant_prompt_chars")
	beforeTools := histogramSampleCount(before, "assistant_prompt_tool_count")

	PromptChars.Observe(1024)
	PromptToolCount.Observe(7)

	after, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather after observe: %v", err)
	}
	afterChars := histogramSampleCount(after, "assistant_prompt_chars")
	afterTools := histogramSampleCount(after, "assistant_prompt_tool_count")

	if afterChars <= beforeChars {
		t.Fatalf("assistant_prompt_chars sample count did not increase: before=%d after=%d", beforeChars, afterChars)
	}
	if afterTools <= beforeTools {
		t.Fatalf("assistant_prompt_tool_count sample count did not increase: before=%d after=%d", beforeTools, afterTools)
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
