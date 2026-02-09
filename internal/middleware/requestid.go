package middleware

import (
	"net/http"

	"github.com/brownster/grafana-assistant/internal/requestid"
)

const requestIDHeader = "X-Request-ID"

// RequestID is middleware that generates a request ID (or reads it from the
// X-Request-ID header), stores it in the request context, and sets the
// response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = requestid.New()
		}
		ctx := requestid.NewContext(r.Context(), id)
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
