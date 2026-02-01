package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiter_AllowsBurst(t *testing.T) {
	rl := NewRateLimiter(20, 3)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(inner)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, w.Code)
		}
	}
}

func TestRateLimiter_RejectsOverBurst(t *testing.T) {
	rl := NewRateLimiter(20, 2)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(inner)

	// Exhaust burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Next request should be rate-limited
	req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestRateLimiter_SkipsGET(t *testing.T) {
	rl := NewRateLimiter(20, 1)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(inner)

	// Exhaust burst for POST
	req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// GET should still work
	req = httptest.NewRequest(http.MethodGet, "/api/chat", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", w.Code)
	}
}

func TestRateLimiter_SkipsNonChatPath(t *testing.T) {
	rl := NewRateLimiter(20, 1)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(inner)

	// Exhaust burst for chat
	req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// POST to other endpoint should still work
	req = httptest.NewRequest(http.MethodPost, "/api/other", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for non-chat path, got %d", w.Code)
	}
}

func TestRateLimiter_UsesCookie(t *testing.T) {
	rl := NewRateLimiter(20, 1)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(inner)

	// Request with cookie
	req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	req.AddCookie(&http.Cookie{Name: "grafana_session", Value: "abc123"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Same cookie, should be rate limited
	req = httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	req.AddCookie(&http.Cookie{Name: "grafana_session", Value: "abc123"})
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for same cookie, got %d", w.Code)
	}

	// Different cookie should still work
	req = httptest.NewRequest(http.MethodPost, "/api/chat", nil)
	req.AddCookie(&http.Cookie{Name: "grafana_session", Value: "different"})
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for different cookie, got %d", w.Code)
	}
}
