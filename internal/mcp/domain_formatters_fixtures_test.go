package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTryDomainSummary_Fixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		expected []string
	}{
		{
			name:    "grafana alerts",
			fixture: "alerts_grafana.json",
			expected: []string{
				"3 alerts",
				"states: firing=1, pending=1, resolved=1",
				"severity: critical=1, warning=1, info=1",
				"common labels: job=api (2), job=worker (1)",
				"datasource: alertmanager",
			},
		},
		{
			name:    "alertmanager envelope",
			fixture: "alerts_alertmanager.json",
			expected: []string{
				"2 alerts",
				"states: firing=1, resolved=1",
				"severity: critical=1, warning=1",
				"common labels: service=payments (2)",
				"datasource: alertmanager",
			},
		},
		{
			name:    "prometheus matrix",
			fixture: "prometheus_matrix.json",
			expected: []string{
				"Prometheus matrix query returned 2 series",
				"metrics: http_requests_total",
				"values min=5, max=18, latest=12",
				"label cardinality: instance=2, job=1",
				"sample points=4",
				"time range=5m",
			},
		},
		{
			name:    "prometheus vector",
			fixture: "prometheus_vector.json",
			expected: []string{
				"Prometheus vector query returned 2 series",
				"metrics: up",
				"values min=0, max=1, latest=0",
				"label cardinality: instance=2, job=2",
				"sample points=2",
			},
		},
		{
			name:    "loki streams",
			fixture: "loki_streams.json",
			expected: []string{
				"Loki query returned 5 log lines across 3 streams",
				"levels: error=2, warn=2, info=1",
				"apps/services: api-server (2), payments (2), worker (1)",
				"time span:",
				"(4m)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := loadDomainFixture(t, tt.fixture)
			summary, ok := tryDomainSummary(input)
			if !ok {
				t.Fatalf("expected domain summary for fixture %q", tt.fixture)
			}
			for _, expected := range tt.expected {
				if !strings.Contains(summary, expected) {
					t.Fatalf("fixture %q expected %q in summary, got: %s", tt.fixture, expected, summary)
				}
			}
		})
	}
}

func TestTryDomainSummary_FixtureGenericFallsThrough(t *testing.T) {
	input := loadDomainFixture(t, "generic_object.json")
	if summary, ok := tryDomainSummary(input); ok || summary != "" {
		t.Fatalf("expected generic fixture to fall through, got ok=%v summary=%q", ok, summary)
	}
}

func TestFormatToolResult_FixtureGenericFallbackUnchanged(t *testing.T) {
	input := loadDomainFixture(t, "generic_object.json")
	got := FormatToolResult(input)

	for _, expected := range []string{
		"Summary: Tool returned object (status=ok, rows=1)",
		"Machine details:",
		"\"source\": \"unit-test\"",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected %q in generic fallback output, got: %s", expected, got)
		}
	}
}

func TestTryDomainSummary_EmptyAndUnsupportedInputs(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{name: "nil", input: nil},
		{name: "empty map", input: map[string]any{}},
		{name: "empty slice", input: []any{}},
		{name: "plain string", input: "not a domain payload"},
		{name: "unknown result type", input: map[string]any{
			"data": map[string]any{"resultType": "unknown", "result": []any{}},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if summary, ok := tryDomainSummary(tt.input); ok || summary != "" {
				t.Fatalf("expected no domain match for %s, got ok=%v summary=%q", tt.name, ok, summary)
			}
		})
	}
}

func TestTryDomainSummary_EmptyRecognizedDomainShapes(t *testing.T) {
	prom := map[string]any{
		"data": map[string]any{
			"resultType": "matrix",
			"result":     []any{},
		},
	}
	if summary, ok := tryDomainSummary(prom); !ok || !strings.Contains(summary, "returned 0 series") {
		t.Fatalf("expected empty matrix summary, got ok=%v summary=%q", ok, summary)
	}

	loki := map[string]any{
		"data": map[string]any{
			"resultType": "streams",
			"result":     []any{},
		},
	}
	if summary, ok := tryDomainSummary(loki); !ok || !strings.Contains(summary, "0 log lines across 0 streams") {
		t.Fatalf("expected empty streams summary, got ok=%v summary=%q", ok, summary)
	}
}

func loadDomainFixture(t *testing.T, name string) any {
	t.Helper()

	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return value
}
