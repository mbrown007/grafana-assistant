package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewGrafanaProxy creates a reverse proxy that forwards requests to Grafana,
// stripping headers that prevent iframe embedding and passing cookies through.
// The /grafana prefix is kept on the forwarded path because Grafana is configured
// with serve_from_sub_path=true and root_url including /grafana/.
func NewGrafanaProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")
		// Grafana sets cookies scoped to /grafana; widen to / so /api can see them.
		// Also harden with SameSite=Strict and HttpOnly if not already present.
		if cookies := resp.Header.Values("Set-Cookie"); len(cookies) > 0 {
			resp.Header.Del("Set-Cookie")
			for _, raw := range cookies {
				updated := strings.ReplaceAll(raw, "Path=/grafana", "Path=/")
				updated = strings.ReplaceAll(updated, "path=/grafana", "path=/")
				lower := strings.ToLower(updated)
				if !strings.Contains(lower, "samesite") {
					updated += "; SameSite=Strict"
				}
				if !strings.Contains(lower, "httponly") {
					updated += "; HttpOnly"
				}
				resp.Header.Add("Set-Cookie", updated)
			}
		}
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("grafana proxy error", "path", r.URL.Path, "error", err)
		http.Error(w, "grafana proxy error", http.StatusBadGateway)
	}

	return proxy
}

// GrafanaHandler returns an http.Handler that proxies to Grafana.
// It handles both regular HTTP requests and WebSocket upgrades.
func GrafanaHandler(grafanaURL string) (http.Handler, error) {
	target, err := url.Parse(grafanaURL)
	if err != nil {
		return nil, err
	}

	proxy := NewGrafanaProxy(target)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WebSocket upgrade is handled transparently by the reverse proxy
		// since httputil.ReverseProxy passes Upgrade headers through.
		proxy.ServeHTTP(w, r)
	}), nil
}
