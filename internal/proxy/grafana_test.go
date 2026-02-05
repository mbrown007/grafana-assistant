package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeaderStripping(t *testing.T) {
	// Fake Grafana that sets headers we want stripped
	fakeGrafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("X-Custom-Header", "keep-me")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer fakeGrafana.Close()

	handler, err := GrafanaHandler(fakeGrafana.URL)
	if err != nil {
		t.Fatalf("GrafanaHandler: %v", err)
	}

	// Wrap handler to add /grafana prefix (matching how mux routes)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/grafana" + r.URL.Path
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if v := resp.Header.Get("X-Frame-Options"); v != "" {
		t.Errorf("X-Frame-Options should be stripped, got %q", v)
	}
	if v := resp.Header.Get("Content-Security-Policy"); v != "" {
		t.Errorf("Content-Security-Policy should be stripped, got %q", v)
	}
	if v := resp.Header.Get("X-Custom-Header"); v != "keep-me" {
		t.Errorf("X-Custom-Header should be preserved, got %q", v)
	}
}

func TestCookiePassthrough(t *testing.T) {
	// Fake Grafana that checks for a cookie and sets one in the response
	fakeGrafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("grafana_session")
		if err != nil || cookie.Value != "abc123" {
			t.Errorf("expected grafana_session cookie 'abc123', got err=%v cookie=%v", err, cookie)
		}
		http.SetCookie(w, &http.Cookie{
			Name:  "grafana_session_expiry",
			Value: "tomorrow",
		})
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeGrafana.Close()

	handler, err := GrafanaHandler(fakeGrafana.URL)
	if err != nil {
		t.Fatalf("GrafanaHandler: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/grafana" + r.URL.Path
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.AddCookie(&http.Cookie{Name: "grafana_session", Value: "abc123"})

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	found := false
	for _, c := range resp.Cookies() {
		if c.Name == "grafana_session_expiry" && c.Value == "tomorrow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("response cookie grafana_session_expiry not found")
	}
}

func TestPathPassthrough(t *testing.T) {
	var receivedPath string
	fakeGrafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeGrafana.Close()

	handler, err := GrafanaHandler(fakeGrafana.URL)
	if err != nil {
		t.Fatalf("GrafanaHandler: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/grafana" + r.URL.Path
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()

	http.Get(srv.URL + "/d/abc123/my-dashboard")

	// Path keeps the /grafana prefix because Grafana is configured with serve_from_sub_path=true.
	if receivedPath != "/grafana/d/abc123/my-dashboard" {
		t.Errorf("expected path /grafana/d/abc123/my-dashboard, got %q", receivedPath)
	}
}

func TestCookiePathWithForwardedPrefix(t *testing.T) {
	// Fake Grafana that sets a cookie with path=/grafana
	fakeGrafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "grafana_session=abc123; Path=/grafana; HttpOnly")
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeGrafana.Close()

	handler, err := GrafanaHandler(fakeGrafana.URL)
	if err != nil {
		t.Fatalf("GrafanaHandler: %v", err)
	}

	// Test with X-Forwarded-Prefix=/assistant
	req := httptest.NewRequest("GET", "/grafana/", nil)
	req.Header.Set("X-Forwarded-Prefix", "/assistant")
	req.Header.Set("X-Forwarded-Proto", "https")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("expected Set-Cookie header")
	}

	cookie := cookies[0]
	// Path should be rewritten from /grafana to /assistant/
	if !contains(cookie, "Path=/assistant/") {
		t.Errorf("expected cookie path /assistant/, got: %s", cookie)
	}
	// Should have SameSite=None for HTTPS
	if !contains(cookie, "SameSite=None") {
		t.Errorf("expected SameSite=None for HTTPS, got: %s", cookie)
	}
	// Should have Secure flag
	if !contains(cookie, "Secure") {
		t.Errorf("expected Secure flag for HTTPS, got: %s", cookie)
	}
}

func TestCookieSameSiteForHTTP(t *testing.T) {
	// Fake Grafana that sets a cookie
	fakeGrafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "grafana_session=abc123")
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeGrafana.Close()

	handler, err := GrafanaHandler(fakeGrafana.URL)
	if err != nil {
		t.Fatalf("GrafanaHandler: %v", err)
	}

	// Test without X-Forwarded-Proto (HTTP)
	req := httptest.NewRequest("GET", "/grafana/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("expected Set-Cookie header")
	}

	cookie := cookies[0]
	// Should have SameSite=Lax for HTTP (not None, which requires Secure)
	if !contains(cookie, "SameSite=Lax") {
		t.Errorf("expected SameSite=Lax for HTTP, got: %s", cookie)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
