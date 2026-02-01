package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/healthz", "/healthz"},
		{"/api/chat", "/api/chat"},
		{"/api/history", "/api/history"},
		{"/api/history/session-123", "/api/history/:id"},
		{"/api/dashboard-context/abc123", "/api/dashboard-context/:uid"},
		{"/grafana/d/abc/my-dashboard", "/grafana"},
		{"/grafana/api/dashboards", "/grafana"},
		{"/api/me", "/api/me"},
	}

	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMetricsMiddleware_RecordsStatus(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := Metrics(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestStatusRecorder_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, statusCode: http.StatusOK}

	sr.Write([]byte("hello"))

	if sr.statusCode != http.StatusOK {
		t.Errorf("expected default status 200, got %d", sr.statusCode)
	}
}
