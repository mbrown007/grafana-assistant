package middleware

import (
	"net/http"
	"strings"
)

// CORS blocks cross-origin requests to /api/ paths.
// If the Origin header is present and doesn't match allowedOrigin, the
// request is rejected with 403. OPTIONS preflight requests are handled.
// If allowedOrigin is empty, CORS enforcement is skipped.
func CORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowedOrigin == "" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			// Same-origin requests (or non-browser clients) omit Origin.
			next.ServeHTTP(w, r)
			return
		}

		if origin != allowedOrigin {
			http.Error(w, "forbidden: origin not allowed", http.StatusForbidden)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// DeriveAllowedOrigin builds a default origin from the listen address.
// E.g. ":8080" → "http://localhost:8080", "0.0.0.0:8080" → "http://localhost:8080".
func DeriveAllowedOrigin(listenAddr string) string {
	host, port, _ := strings.Cut(listenAddr, ":")
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	if port == "" {
		port = "8080"
	}
	return "http://" + host + ":" + port
}
