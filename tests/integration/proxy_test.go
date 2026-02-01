//go:build integration

package integration

import (
	"io"
	"net/http"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/proxy"
)

func TestProxy_Returns200(t *testing.T) {
	grafanaURL := testGrafanaURL(t)
	cfg := testConfig(grafanaURL)

	mux := http.NewServeMux()
	grafanaHandler, err := proxy.GrafanaHandler(cfg.GrafanaURL)
	if err != nil {
		t.Fatalf("failed to create grafana proxy: %v", err)
	}
	mux.Handle("/grafana/", grafanaHandler)

	ts := setupTestServer(t, mux, cfg)

	resp, err := http.Get(ts.URL + "/grafana/api/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestProxy_XFrameOptionsStripped(t *testing.T) {
	grafanaURL := testGrafanaURL(t)
	cfg := testConfig(grafanaURL)

	mux := http.NewServeMux()
	grafanaHandler, err := proxy.GrafanaHandler(cfg.GrafanaURL)
	if err != nil {
		t.Fatalf("failed to create grafana proxy: %v", err)
	}
	mux.Handle("/grafana/", grafanaHandler)

	ts := setupTestServer(t, mux, cfg)

	resp, err := http.Get(ts.URL + "/grafana/login")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if val := resp.Header.Get("X-Frame-Options"); val != "" {
		t.Errorf("expected X-Frame-Options to be stripped from proxied response, got %q", val)
	}
}

func TestProxy_CookiePathRewriting(t *testing.T) {
	grafanaURL := testGrafanaURL(t)
	cfg := testConfig(grafanaURL)

	mux := http.NewServeMux()
	grafanaHandler, err := proxy.GrafanaHandler(cfg.GrafanaURL)
	if err != nil {
		t.Fatalf("failed to create grafana proxy: %v", err)
	}
	mux.Handle("/grafana/", grafanaHandler)

	ts := setupTestServer(t, mux, cfg)

	// Login page typically sets cookies.
	resp, err := http.Get(ts.URL + "/grafana/login")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	for _, cookie := range resp.Header.Values("Set-Cookie") {
		if cookie != "" {
			// Verify no cookies have Path=/grafana (they should be rewritten to Path=/).
			if contains(cookie, "Path=/grafana") || contains(cookie, "path=/grafana") {
				t.Errorf("cookie path should be rewritten from /grafana to /: %s", cookie)
			}
		}
	}
}

func TestProxy_RequestIDHeader(t *testing.T) {
	grafanaURL := testGrafanaURL(t)
	cfg := testConfig(grafanaURL)

	mux := http.NewServeMux()
	grafanaHandler, err := proxy.GrafanaHandler(cfg.GrafanaURL)
	if err != nil {
		t.Fatalf("failed to create grafana proxy: %v", err)
	}
	mux.Handle("/grafana/", grafanaHandler)

	ts := setupTestServer(t, mux, cfg)

	resp, err := http.Get(ts.URL + "/grafana/api/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	rid := resp.Header.Get("X-Request-ID")
	if rid == "" {
		t.Error("expected X-Request-ID header in response")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
