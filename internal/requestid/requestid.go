package requestid

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

// New generates a new UUID v4 request ID.
func New() string {
	return uuid.New().String()
}

// NewContext stores a request ID in the context.
func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext retrieves the request ID from the context.
// Returns an empty string if no request ID is present.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(contextKey{}).(string); ok {
		return id
	}
	return ""
}
