package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

// DefaultMaxMessageLength is the maximum character count for a chat message.
const DefaultMaxMessageLength = 16000

// ChatHandlerFunc is the signature for the agent's HandleChat method.
type ChatHandlerFunc func(r *http.Request, req ChatRequest, streamFn func(StreamChunk))

// ChatHandler returns an HTTP handler for POST /api/chat with SSE streaming.
func ChatHandler(handle ChatHandlerFunc) http.HandlerFunc {
	return ChatHandlerWithLimit(handle, DefaultMaxMessageLength)
}

// ChatHandlerWithLimit is like ChatHandler but accepts a custom max message length.
func ChatHandlerWithLimit(handle ChatHandlerFunc, maxMessageLength int) http.HandlerFunc {
	if maxMessageLength <= 0 {
		maxMessageLength = DefaultMaxMessageLength
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}

		if len([]rune(req.Message)) > maxMessageLength {
			http.Error(w, "message too long", http.StatusBadRequest)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		streamFn := func(chunk StreamChunk) {
			data, err := json.Marshal(chunk)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to marshal stream chunk", "error", err)
				return
			}
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}

		handle(r, req, streamFn)
	}
}

// AuthenticatedChatHandler resolves the Grafana user before starting a stream.
func AuthenticatedChatHandler(
	resolver UserResolver,
	handle func(r *http.Request, user *grafana.User, req ChatRequest, streamFn func(StreamChunk)),
) http.HandlerFunc {
	return AuthenticatedChatHandlerWithLimit(resolver, handle, DefaultMaxMessageLength)
}

// AuthenticatedChatHandlerWithLimit is like AuthenticatedChatHandler but accepts a custom max message length.
func AuthenticatedChatHandlerWithLimit(
	resolver UserResolver,
	handle func(r *http.Request, user *grafana.User, req ChatRequest, streamFn func(StreamChunk)),
	maxMessageLength int,
) http.HandlerFunc {
	if maxMessageLength <= 0 {
		maxMessageLength = DefaultMaxMessageLength
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}

		if len([]rune(req.Message)) > maxMessageLength {
			http.Error(w, "message too long", http.StatusBadRequest)
			return
		}

		user, err := resolver.Resolve(r.Context(), r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		streamFn := func(chunk StreamChunk) {
			data, err := json.Marshal(chunk)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to marshal stream chunk", "error", err)
				return
			}
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}

		handle(r, user, req, streamFn)
	}
}
