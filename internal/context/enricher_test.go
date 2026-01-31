package context

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

const dashboardResponse = `{
	"meta": {"slug": "test-dash", "folderTitle": "Production"},
	"dashboard": {
		"uid": "abc123",
		"title": "Node Metrics",
		"tags": ["linux", "prometheus"],
		"panels": [
			{
				"title": "CPU Usage",
				"description": "CPU utilization over time",
				"type": "graph",
				"targets": [
					{"expr": "rate(node_cpu_seconds_total{mode=\"idle\"}[5m])"},
					{"expr": "rate(node_cpu_seconds_total{mode=\"system\"}[5m])"}
				]
			},
			{
				"title": "Memory",
				"type": "stat",
				"targets": [
					{"expr": "node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes"}
				]
			}
		]
	}
}`

func newTestServer(response string, callCount *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if callCount != nil {
			callCount.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, response)
	}))
}

func TestEnricher_BasicDashboard(t *testing.T) {
	srv := newTestServer(dashboardResponse, nil)
	defer srv.Close()

	client := grafana.NewClient(srv.URL, "token")
	enricher := NewEnricher(client, 5*time.Minute)

	summary, err := enricher.GetDashboardSummary(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.UID != "abc123" {
		t.Errorf("UID = %q, want %q", summary.UID, "abc123")
	}
	if summary.Title != "Node Metrics" {
		t.Errorf("Title = %q, want %q", summary.Title, "Node Metrics")
	}
	if summary.Folder != "Production" {
		t.Errorf("Folder = %q, want %q", summary.Folder, "Production")
	}
	if len(summary.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(summary.Tags))
	}
	if len(summary.Panels) != 2 {
		t.Fatalf("len(Panels) = %d, want 2", len(summary.Panels))
	}
	if summary.Panels[0].Title != "CPU Usage" {
		t.Errorf("Panels[0].Title = %q, want %q", summary.Panels[0].Title, "CPU Usage")
	}
	if len(summary.Panels[0].Queries) != 2 {
		t.Errorf("len(Panels[0].Queries) = %d, want 2", len(summary.Panels[0].Queries))
	}
}

func TestEnricher_NestedRowPanels(t *testing.T) {
	response := `{
		"meta": {"slug": "rows", "folderTitle": ""},
		"dashboard": {
			"uid": "row1",
			"title": "Row Dashboard",
			"tags": [],
			"panels": [
				{
					"title": "Row A",
					"type": "row",
					"panels": [
						{"title": "Inside Row A", "type": "graph", "targets": [{"expr": "up"}]},
						{"title": "Also in Row A", "type": "stat", "targets": []}
					]
				},
				{
					"title": "Standalone",
					"type": "graph",
					"targets": [{"expr": "process_resident_memory_bytes"}]
				}
			]
		}
	}`

	srv := newTestServer(response, nil)
	defer srv.Close()

	client := grafana.NewClient(srv.URL, "token")
	enricher := NewEnricher(client, 5*time.Minute)

	summary, err := enricher.GetDashboardSummary(context.Background(), "row1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Row panel should be flattened: 2 from row + 1 standalone = 3
	if len(summary.Panels) != 3 {
		t.Fatalf("len(Panels) = %d, want 3", len(summary.Panels))
	}
	if summary.Panels[0].Title != "Inside Row A" {
		t.Errorf("Panels[0].Title = %q, want %q", summary.Panels[0].Title, "Inside Row A")
	}
	if summary.Panels[2].Title != "Standalone" {
		t.Errorf("Panels[2].Title = %q, want %q", summary.Panels[2].Title, "Standalone")
	}
}

func TestEnricher_CacheHit(t *testing.T) {
	var calls atomic.Int32
	srv := newTestServer(dashboardResponse, &calls)
	defer srv.Close()

	client := grafana.NewClient(srv.URL, "token")
	enricher := NewEnricher(client, 5*time.Minute)

	_, err := enricher.GetDashboardSummary(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	_, err = enricher.GetDashboardSummary(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("server called %d times, want 1 (cache hit)", calls.Load())
	}
}

func TestEnricher_CacheExpiry(t *testing.T) {
	var calls atomic.Int32
	srv := newTestServer(dashboardResponse, &calls)
	defer srv.Close()

	client := grafana.NewClient(srv.URL, "token")
	enricher := NewEnricher(client, 1*time.Millisecond) // very short TTL

	_, err := enricher.GetDashboardSummary(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	time.Sleep(5 * time.Millisecond) // wait for cache to expire

	_, err = enricher.GetDashboardSummary(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if calls.Load() != 2 {
		t.Errorf("server called %d times, want 2 (cache expired)", calls.Load())
	}
}

func TestEnricher_SizeLimits(t *testing.T) {
	// Build a dashboard with 35 panels, each with 7 targets, some with long queries.
	var panels []string
	for i := 0; i < 35; i++ {
		var targets []string
		for j := 0; j < 7; j++ {
			q := fmt.Sprintf("metric_%d_%d{%s}", i, j, strings.Repeat("x", 600))
			targets = append(targets, fmt.Sprintf(`{"expr": %q}`, q))
		}
		panels = append(panels, fmt.Sprintf(
			`{"title":"Panel %d","type":"graph","targets":[%s]}`,
			i, strings.Join(targets, ",")))
	}

	response := fmt.Sprintf(`{
		"meta": {"slug": "big", "folderTitle": ""},
		"dashboard": {
			"uid": "big1",
			"title": "Big Dashboard",
			"tags": [],
			"panels": [%s]
		}
	}`, strings.Join(panels, ","))

	srv := newTestServer(response, nil)
	defer srv.Close()

	client := grafana.NewClient(srv.URL, "token")
	enricher := NewEnricher(client, 5*time.Minute)

	summary, err := enricher.GetDashboardSummary(context.Background(), "big1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(summary.Panels) != 30 {
		t.Errorf("len(Panels) = %d, want 30 (max)", len(summary.Panels))
	}

	for i, p := range summary.Panels {
		if len(p.Queries) > 5 {
			t.Errorf("Panel %d has %d queries, want <= 5", i, len(p.Queries))
		}
		for j, q := range p.Queries {
			if len(q) > 500 {
				t.Errorf("Panel %d query %d len = %d, want <= 500", i, j, len(q))
			}
		}
	}
}
