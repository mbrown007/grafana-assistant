package mcp

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTryDomainSummary_GrafanaAlerts(t *testing.T) {
	now := time.Now().UTC()
	input := map[string]any{
		"datasource": "alertmanager",
		"alerts": []any{
			map[string]any{
				"state":    "firing",
				"severity": "critical",
				"startsAt": now.Add(-3 * time.Hour).Format(time.RFC3339),
				"labels": map[string]any{
					"job": "api",
				},
			},
			map[string]any{
				"state":    "firing",
				"severity": "warning",
				"startsAt": now.Add(-2 * time.Hour).Format(time.RFC3339),
				"labels": map[string]any{
					"job": "api",
				},
			},
			map[string]any{
				"state": "resolved",
				"labels": map[string]any{
					"job":      "worker",
					"severity": "info",
				},
			},
		},
	}

	summary, ok := tryDomainSummary(input)
	if !ok {
		t.Fatalf("expected alert domain summary match")
	}

	for _, expected := range []string{
		"3 alerts",
		"states: firing=2, resolved=1",
		"severity: critical=1, warning=1, info=1",
		"firing oldest",
		"common labels: job=api (2), job=worker (1)",
		"datasource: alertmanager",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected %q in summary, got: %s", expected, summary)
		}
	}
}

func TestTryDomainSummary_AlertmanagerEnvelope(t *testing.T) {
	input := map[string]any{
		"status": "success",
		"data": map[string]any{
			"alerts": []any{
				map[string]any{
					"status": "active",
					"labels": map[string]any{
						"severity": "critical",
						"service":  "payments",
					},
				},
			},
		},
	}

	summary, ok := tryDomainSummary(input)
	if !ok {
		t.Fatalf("expected alertmanager envelope to match alert formatter")
	}
	for _, expected := range []string{
		"1 alerts",
		"states: firing=1",
		"severity: critical=1",
		"common labels: service=payments (1)",
		"datasource: alertmanager",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected %q in summary, got: %s", expected, summary)
		}
	}
}

func TestTryDomainSummary_NonAlertFallsThrough(t *testing.T) {
	input := map[string]any{
		"status": "ok",
		"rows":   []any{map[string]any{"name": "api", "errors": 2}},
	}

	if summary, ok := tryDomainSummary(input); ok || summary != "" {
		t.Fatalf("expected no alert domain match, got ok=%v summary=%q", ok, summary)
	}
}

func TestFormatToolResult_AlertDomainSummaryPreferred(t *testing.T) {
	input := map[string]any{
		"alerts": []any{
			map[string]any{
				"state":    "firing",
				"severity": "critical",
				"labels":   map[string]any{"job": "api"},
			},
		},
	}

	got := FormatToolResult(input)
	if !strings.Contains(got, "Summary: 1 alerts; states: firing=1; severity: critical=1; common labels: job=api (1).") {
		t.Fatalf("expected enriched alert summary, got: %s", got)
	}
	if strings.Contains(got, "Tool returned object (") {
		t.Fatalf("expected domain summary to bypass generic object summary, got: %s", got)
	}
}

func TestTryDomainSummary_PrometheusMatrix(t *testing.T) {
	input := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "matrix",
			"result": []any{
				map[string]any{
					"metric": map[string]any{
						"__name__": "http_requests_total",
						"job":      "api",
						"instance": "api-1",
					},
					"values": []any{
						[]any{float64(1710200000), "10"},
						[]any{float64(1710200300), "18"},
					},
				},
				map[string]any{
					"metric": map[string]any{
						"__name__": "http_requests_total",
						"job":      "api",
						"instance": "api-2",
					},
					"values": []any{
						[]any{float64(1710200000), "5"},
						[]any{float64(1710200300), "12"},
					},
				},
			},
		},
	}

	summary, ok := tryDomainSummary(input)
	if !ok {
		t.Fatalf("expected prometheus matrix domain summary match")
	}
	for _, expected := range []string{
		"Prometheus matrix query returned 2 series",
		"metrics: http_requests_total",
		"values min=5, max=18, latest=12",
		"label cardinality: instance=2, job=1",
		"sample points=4",
		"time range=5m",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected %q in summary, got: %s", expected, summary)
		}
	}
}

func TestTryDomainSummary_PrometheusVector(t *testing.T) {
	input := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "vector",
			"result": []any{
				map[string]any{
					"metric": map[string]any{
						"__name__": "up",
						"job":      "api",
						"instance": "api-1",
					},
					"value": []any{float64(1710200300), "1"},
				},
				map[string]any{
					"metric": map[string]any{
						"__name__": "up",
						"job":      "worker",
						"instance": "worker-1",
					},
					"value": []any{float64(1710200300), "0"},
				},
			},
		},
	}

	summary, ok := tryDomainSummary(input)
	if !ok {
		t.Fatalf("expected prometheus vector domain summary match")
	}
	for _, expected := range []string{
		"Prometheus vector query returned 2 series",
		"metrics: up",
		"values min=0, max=1, latest=0",
		"label cardinality: instance=2, job=2",
		"sample points=2",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected %q in summary, got: %s", expected, summary)
		}
	}
}

func TestTryDomainSummary_PrometheusScalar(t *testing.T) {
	input := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "scalar",
			"result":     []any{float64(1710200300), "0.9234"},
		},
	}

	summary, ok := tryDomainSummary(input)
	if !ok {
		t.Fatalf("expected prometheus scalar domain summary match")
	}
	if !strings.Contains(summary, "Prometheus scalar query returned 1 value; value=0.9234") {
		t.Fatalf("unexpected scalar summary: %s", summary)
	}
}

func TestFormatToolResult_PrometheusDomainSummaryPreferred(t *testing.T) {
	input := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "vector",
			"result": []any{
				map[string]any{
					"metric": map[string]any{
						"__name__": "up",
						"job":      "api",
					},
					"value": []any{float64(1710200300), "1"},
				},
			},
		},
	}

	got := FormatToolResult(input)
	if !strings.Contains(got, "Summary: Prometheus vector query returned 1 series; metrics: up; values min=1, max=1, latest=1") {
		t.Fatalf("expected enriched prometheus summary, got: %s", got)
	}
	if strings.Contains(got, "Tool returned object (") {
		t.Fatalf("expected domain summary to bypass generic object summary, got: %s", got)
	}
}

func TestTryDomainSummary_LokiStreams(t *testing.T) {
	start := int64(1700000000000000000)
	input := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "streams",
			"result": []any{
				map[string]any{
					"stream": map[string]any{"app": "api-server", "level": "error"},
					"values": []any{
						[]any{strconv.FormatInt(start, 10), "error: timeout"},
						[]any{strconv.FormatInt(start+60000000000, 10), "error: db"},
					},
				},
				map[string]any{
					"stream": map[string]any{"service": "worker", "level": "info"},
					"values": []any{
						[]any{strconv.FormatInt(start+120000000000, 10), "info: done"},
					},
				},
				map[string]any{
					"stream": map[string]any{"app": "payments", "level": "warn"},
					"values": []any{
						[]any{strconv.FormatInt(start+180000000000, 10), "warn: slow"},
						[]any{strconv.FormatInt(start+2700000000000, 10), "warn: queue"},
					},
				},
			},
		},
	}

	summary, ok := tryDomainSummary(input)
	if !ok {
		t.Fatalf("expected loki domain summary match")
	}
	for _, expected := range []string{
		"Loki query returned 5 log lines across 3 streams",
		"levels: error=2, warn=2, info=1",
		"apps/services: api-server (2), payments (2), worker (1)",
		"time span:",
		"(45m)",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected %q in summary, got: %s", expected, summary)
		}
	}
}

func TestFormatToolResult_LokiDomainSummaryPreferred(t *testing.T) {
	input := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "streams",
			"result": []any{
				map[string]any{
					"stream": map[string]any{"app": "api-server", "level": "error"},
					"values": []any{
						[]any{"1700000000000000000", "error one"},
						[]any{"1700000001000000000", "error two"},
					},
				},
			},
		},
	}

	got := FormatToolResult(input)
	if !strings.Contains(got, "Summary: Loki query returned 2 log lines across 1 stream; levels: error=2; apps/services: api-server (2)") {
		t.Fatalf("expected enriched loki summary, got: %s", got)
	}
	if strings.Contains(got, "Tool returned object (") {
		t.Fatalf("expected domain summary to bypass generic object summary, got: %s", got)
	}
}
