package logging

import (
	"context"
	"log/slog"

	"github.com/marcusz/monitoring-assistant/internal/requestid"
)

// ContextHandler wraps an slog.Handler to automatically inject the request_id
// from the context into every log record.
type ContextHandler struct {
	inner slog.Handler
}

// NewContextHandler creates a new ContextHandler wrapping the given handler.
func NewContextHandler(inner slog.Handler) *ContextHandler {
	return &ContextHandler{inner: inner}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, record slog.Record) error {
	if id := requestid.FromContext(ctx); id != "" {
		record.AddAttrs(slog.String("request_id", id))
	}
	return h.inner.Handle(ctx, record)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{inner: h.inner.WithGroup(name)}
}
