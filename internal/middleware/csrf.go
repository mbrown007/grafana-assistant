package middleware

import (
	"net/http"
	"strings"
)

// CSRFCheck requires an X-Requested-With header on all POST and DELETE
// requests to /api/ paths. Combined with CORS, this prevents cross-site
// request forgery because browsers block custom headers on cross-origin
// requests without an approved CORS preflight.
func CSRFCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		if r.Method == http.MethodPost || r.Method == http.MethodDelete {
			if r.Header.Get("X-Requested-With") == "" {
				http.Error(w, "forbidden: missing X-Requested-With header", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
