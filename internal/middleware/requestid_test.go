package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brownster/grafana-assistant/internal/requestid"
)

func TestRequestID_GeneratesID(t *testing.T) {
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = requestid.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedID == "" {
		t.Fatal("expected request ID in context, got empty string")
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID response header")
	}
	if rec.Header().Get("X-Request-ID") != capturedID {
		t.Errorf("response header %q != context ID %q", rec.Header().Get("X-Request-ID"), capturedID)
	}
}

func TestRequestID_ReusesIncomingHeader(t *testing.T) {
	const existingID = "my-custom-id-123"
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = requestid.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedID != existingID {
		t.Errorf("expected context ID %q, got %q", existingID, capturedID)
	}
	if rec.Header().Get("X-Request-ID") != existingID {
		t.Errorf("expected response header %q, got %q", existingID, rec.Header().Get("X-Request-ID"))
	}
}
