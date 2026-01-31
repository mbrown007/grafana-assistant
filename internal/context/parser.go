package context

import (
	"net/url"
	"strings"
)

// URLContext holds the extracted context from a Grafana dashboard URL.
type URLContext struct {
	UID       string
	TimeFrom  string            // e.g. "now-6h"
	TimeTo    string            // e.g. "now"
	Variables map[string]string // e.g. {"server": "prod-01"}
}

// ParseDashboardURL parses a Grafana iframe URL and extracts the dashboard UID,
// time range, and template variables. Returns nil if no dashboard path is found.
func ParseDashboardURL(rawURL string) *URLContext {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	// Extract UID from path patterns: /d/<UID>/... or /grafana/d/<UID>/...
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	uid := extractUID(segments)
	if uid == "" {
		return nil
	}

	ctx := &URLContext{
		UID:       uid,
		Variables: make(map[string]string),
	}

	q := u.Query()
	ctx.TimeFrom = q.Get("from")
	ctx.TimeTo = q.Get("to")

	for key, vals := range q {
		if strings.HasPrefix(key, "var-") && len(vals) > 0 {
			ctx.Variables[strings.TrimPrefix(key, "var-")] = vals[0]
		}
	}

	return ctx
}

// extractUID finds the dashboard UID from URL path segments.
// Looks for "d" followed by a UID segment.
func extractUID(segments []string) string {
	for i, seg := range segments {
		if seg == "d" && i+1 < len(segments) {
			return segments[i+1]
		}
	}
	return ""
}
