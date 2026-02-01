package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatHandler_MethodNotAllowed(t *testing.T) {
	handler := ChatHandler(func(r *http.Request, req ChatRequest, streamFn func(StreamChunk)) {})
	req := httptest.NewRequest(http.MethodGet, "/api/chat", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestChatHandler_EmptyMessage(t *testing.T) {
	handler := ChatHandler(func(r *http.Request, req ChatRequest, streamFn func(StreamChunk)) {})
	body := `{"message":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandler_InvalidJSON(t *testing.T) {
	handler := ChatHandler(func(r *http.Request, req ChatRequest, streamFn func(StreamChunk)) {})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandler_SSEStream(t *testing.T) {
	handler := ChatHandler(func(r *http.Request, req ChatRequest, streamFn func(StreamChunk)) {
		if req.Message != "hello" {
			t.Errorf("expected message 'hello', got %q", req.Message)
		}
		streamFn(StreamChunk{Type: "token", Message: "hi"})
		streamFn(StreamChunk{Type: "done"})
	})

	body := `{"message":"hello","session_id":"s1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", w.Header().Get("Content-Type"))
	}

	// Parse SSE events from body.
	events := strings.Split(strings.TrimSpace(w.Body.String()), "\n\n")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %q", len(events), w.Body.String())
	}

	// First event should be a token.
	data := strings.TrimPrefix(events[0], "data: ")
	var chunk StreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		t.Fatalf("failed to parse first event: %v", err)
	}
	if chunk.Type != "token" || chunk.Message != "hi" {
		t.Errorf("unexpected first chunk: %+v", chunk)
	}
}
