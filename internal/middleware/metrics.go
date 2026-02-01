package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/metrics"
)

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap returns the underlying ResponseWriter for http.Flusher detection.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Metrics is HTTP middleware that records request count and duration.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := normalizePath(r.URL.Path)

		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rec.statusCode)

		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// normalizePath reduces path cardinality for metrics by replacing dynamic
// segments with placeholders.
func normalizePath(path string) string {
	if strings.HasPrefix(path, "/grafana") {
		return "/grafana"
	}
	if strings.HasPrefix(path, "/api/history/") {
		return "/api/history/:id"
	}
	if strings.HasPrefix(path, "/api/dashboard-context/") {
		return "/api/dashboard-context/:uid"
	}
	return path
}
