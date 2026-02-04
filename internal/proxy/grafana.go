package proxy

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// proxyContextKey is used to pass request metadata to ModifyResponse.
type proxyContextKey struct{}

type proxyContext struct {
	forwardedPrefix string
	forwardedProto  string
}

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

		// Get proxy context from request (set by GrafanaHandler).
		var pctx proxyContext
		if v := resp.Request.Context().Value(proxyContextKey{}); v != nil {
			pctx = v.(proxyContext)
		}

		// Determine cookie path: use X-Forwarded-Prefix if set, otherwise root.
		cookiePath := "/"
		if pctx.forwardedPrefix != "" {
			cookiePath = pctx.forwardedPrefix
			if !strings.HasSuffix(cookiePath, "/") {
				cookiePath += "/"
			}
		}

		// Determine SameSite policy: use None for HTTPS (enables cross-origin/iframe),
		// Strict for HTTP (more secure for direct access).
		isHTTPS := pctx.forwardedProto == "https"

		// Grafana sets cookies scoped to /grafana; update path and security attributes.
		if cookies := resp.Header.Values("Set-Cookie"); len(cookies) > 0 {
			resp.Header.Del("Set-Cookie")
			for _, raw := range cookies {
				updated := raw

				// Replace Grafana's /grafana path with the correct path.
				updated = strings.ReplaceAll(updated, "Path=/grafana", "Path="+cookiePath)
				updated = strings.ReplaceAll(updated, "path=/grafana", "path="+cookiePath)

				// If no path is set, add one.
				lower := strings.ToLower(updated)
				if !strings.Contains(lower, "path=") {
					updated += "; Path=" + cookiePath
				}

				// Set SameSite based on protocol.
				if !strings.Contains(lower, "samesite") {
					if isHTTPS {
						updated += "; SameSite=None; Secure"
					} else {
						updated += "; SameSite=Lax"
					}
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
		slog.ErrorContext(r.Context(), "grafana proxy error", "path", r.URL.Path, "error", err)
		http.Error(w, "grafana proxy error", http.StatusBadGateway)
	}

	return proxy
}

// GrafanaHandler returns an http.Handler that proxies to Grafana.
// It handles both regular HTTP requests and WebSocket upgrades.
// It reads X-Forwarded-Prefix and X-Forwarded-Proto headers from the
// reverse proxy to set correct cookie paths and security attributes.
func GrafanaHandler(grafanaURL string) (http.Handler, error) {
	target, err := url.Parse(grafanaURL)
	if err != nil {
		return nil, err
	}

	proxy := NewGrafanaProxy(target)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture forwarded headers and pass to ModifyResponse via context.
		pctx := proxyContext{
			forwardedPrefix: r.Header.Get("X-Forwarded-Prefix"),
			forwardedProto:  r.Header.Get("X-Forwarded-Proto"),
		}
		ctx := context.WithValue(r.Context(), proxyContextKey{}, pctx)
		r = r.WithContext(ctx)

		// WebSocket upgrade is handled transparently by the reverse proxy
		// since httputil.ReverseProxy passes Upgrade headers through.
		proxy.ServeHTTP(w, r)
	}), nil
}
