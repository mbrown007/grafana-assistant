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

func TestPathStripping(t *testing.T) {
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

	if receivedPath != "/d/abc123/my-dashboard" {
		t.Errorf("expected path /d/abc123/my-dashboard, got %q", receivedPath)
	}
}
