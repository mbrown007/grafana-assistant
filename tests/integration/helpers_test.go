//go:build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/config"
	"github.com/marcusz/monitoring-assistant/internal/middleware"
)

// testGrafanaURL returns the Grafana URL from the environment or skips the test.
func testGrafanaURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_GRAFANA_URL")
	if url == "" {
		t.Skip("TEST_GRAFANA_URL not set, skipping integration test")
	}
	// Quick reachability check.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url + "/api/health")
	if err != nil {
		t.Skipf("Grafana not reachable at %s: %v", url, err)
	}
	resp.Body.Close()
	return url
}

// testConfig returns a minimal config suitable for integration testing.
func testConfig(grafanaURL string) *config.Config {
	return &config.Config{
		ListenAddr:        ":0",
		GrafanaURL:        grafanaURL,
		GrafanaToken:      os.Getenv("TEST_GRAFANA_TOKEN"),
		DataRetentionDays: 30,
		DBPath:            ":memory:",
		OpenAIModel:       "gpt-4o",
		MetricsEnabled:    true,
		MaxMessageLength:  16000,
		MaxBodySize:       65536,
		RateLimitPerMin:   1000,
		RateLimitBurst:    100,
	}
}

// setupTestMux builds the HTTP mux with middleware, similar to main.go.
// Returns the test server and a cleanup function.
func setupTestServer(t *testing.T, mux *http.ServeMux, cfg *config.Config) *httptest.Server {
	t.Helper()

	allowedOrigin := cfg.AllowedOrigin
	if allowedOrigin == "" {
		allowedOrigin = middleware.DeriveAllowedOrigin(cfg.ListenAddr)
	}

	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitPerMin, cfg.RateLimitBurst)

	var handler http.Handler = mux
	handler = middleware.BodyLimit(cfg.MaxBodySize, handler)
	handler = rateLimiter.Middleware(handler)
	handler = middleware.CSRFCheck(handler)
	handler = middleware.CORS(allowedOrigin, handler)
	handler = middleware.SecurityHeaders(handler)
	if cfg.MetricsEnabled {
		handler = middleware.Metrics(handler)
	}
	handler = middleware.RequestID(handler)

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}
