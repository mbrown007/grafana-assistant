package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brownster/grafana-assistant/internal/grafana"
)

func TestResolveSuccess(t *testing.T) {
	fakeGrafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("grafana_session")
		if err != nil || cookie.Value != "valid-session" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(grafana.User{
			ID: 10, Login: "testuser", OrgID: 1,
		})
	}))
	defer fakeGrafana.Close()

	gc := grafana.NewClient(fakeGrafana.URL, "")
	resolver := NewSessionResolver(gc)

	req := httptest.NewRequest("GET", "/api/chat", nil)
	req.AddCookie(&http.Cookie{Name: "grafana_session", Value: "valid-session"})

	user, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if user.ID != 10 {
		t.Errorf("user ID = %d, want 10", user.ID)
	}
}

func TestResolveNoCookies(t *testing.T) {
	gc := grafana.NewClient("http://localhost:9999", "")
	resolver := NewSessionResolver(gc)

	req := httptest.NewRequest("GET", "/api/chat", nil)
	_, err := resolver.Resolve(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for request with no cookies")
	}
}
