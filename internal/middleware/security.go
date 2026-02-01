package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders adds standard security headers to all responses.
// Grafana proxy paths are excluded from X-Frame-Options so iframes work.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		if !strings.HasPrefix(r.URL.Path, "/grafana") {
			h.Set("X-Frame-Options", "DENY")
		}
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
