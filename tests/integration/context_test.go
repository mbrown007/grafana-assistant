//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

func TestDashboardContext_EnrichedResponse(t *testing.T) {
	grafanaURL := testGrafanaURL(t)
	token := os.Getenv("TEST_GRAFANA_TOKEN")
	if token == "" {
		t.Skip("TEST_GRAFANA_TOKEN not set, skipping dashboard context test")
	}
	cfg := testConfig(grafanaURL)

	grafanaClient := grafana.NewClient(cfg.GrafanaURL, cfg.GrafanaToken)
	enricher := appcontext.NewEnricher(grafanaClient, 5*time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/dashboard-context/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		summary, err := enricher.GetDashboardSummary(r.Context(), uid)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	})

	ts := setupTestServer(t, mux, cfg)

	// Create a test dashboard via Grafana API.
	dashUID := createTestDashboard(t, grafanaURL, token)
	t.Cleanup(func() {
		deleteTestDashboard(t, grafanaURL, token, dashUID)
	})

	// Fetch enriched context.
	resp, err := http.Get(ts.URL + "/api/dashboard-context/" + dashUID)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var summary map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if summary["title"] == nil || summary["title"] == "" {
		t.Error("expected non-empty title in dashboard summary")
	}
}

func createTestDashboard(t *testing.T, grafanaURL, token string) string {
	t.Helper()

	payload := `{
		"dashboard": {
			"title": "Integration Test Dashboard",
			"panels": [{"id": 1, "type": "graph", "title": "Test Panel"}],
			"schemaVersion": 30
		},
		"overwrite": true
	}`

	req, err := http.NewRequest(http.MethodPost, grafanaURL+"/api/dashboards/db", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to create test dashboard: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("failed to create test dashboard: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		UID string `json:"uid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode dashboard creation response: %v", err)
	}

	return result.UID
}

func deleteTestDashboard(t *testing.T, grafanaURL, token, uid string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/dashboards/uid/%s", grafanaURL, uid), nil)
	if err != nil {
		t.Logf("failed to create delete request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("failed to delete test dashboard: %v", err)
		return
	}
	resp.Body.Close()
}
