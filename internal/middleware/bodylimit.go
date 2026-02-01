package middleware

import (
	"net/http"
	"strings"
)

// BodyLimit wraps request bodies with http.MaxBytesReader for /api/ routes.
// This prevents clients from sending excessively large payloads. Grafana
// proxy routes are excluded since they have their own upstream limits.
func BodyLimit(maxBytes int64, next http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 65536 // 64KB default
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}
