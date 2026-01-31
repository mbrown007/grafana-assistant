package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewGrafanaProxy creates a reverse proxy that forwards requests to Grafana,
// stripping headers that prevent iframe embedding and passing cookies through.
func NewGrafanaProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Strip the /grafana prefix before forwarding
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/grafana")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}

		req.Host = target.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("X-Frame-Options")
		resp.Header.Del("Content-Security-Policy")
		return nil
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
