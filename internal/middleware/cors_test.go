package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_BlocksCrossOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS("http://localhost:8080", inner)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCORS_AllowsSameOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS("http://localhost:8080", inner)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8080" {
		t.Errorf("expected ACAO header, got %q", got)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS("http://localhost:8080", inner)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCORS_Preflight(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS("http://localhost:8080", inner)

	req := httptest.NewRequest(http.MethodOptions, "/api/chat", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestCORS_SkipsNonAPIPath(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS("http://localhost:8080", inner)

	req := httptest.NewRequest(http.MethodGet, "/grafana/something", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for non-api path, got %d", w.Code)
	}
}

func TestCORS_EmptyAllowedOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS("", inner)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when CORS disabled, got %d", w.Code)
	}
}

func TestDeriveAllowedOrigin(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{":8080", "http://localhost:8080"},
		{"0.0.0.0:8080", "http://localhost:8080"},
		{"192.168.1.1:9090", "http://192.168.1.1:9090"},
	}
	for _, tt := range tests {
		got := DeriveAllowedOrigin(tt.addr)
		if got != tt.want {
			t.Errorf("DeriveAllowedOrigin(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}
