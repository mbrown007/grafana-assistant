package grafana

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCurrentUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		json.NewEncoder(w).Encode(User{
			ID: 42, Login: "admin", Name: "Admin", Email: "admin@test.com", OrgID: 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token")
	u, err := c.GetCurrentUser(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if u.ID != 42 {
		t.Errorf("ID = %d, want 42", u.ID)
	}
	if u.Login != "admin" {
		t.Errorf("Login = %q, want %q", u.Login, "admin")
	}
}

func TestResolveUserFromSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("grafana_session")
		if err != nil || cookie.Value != "abc123" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Unauthorized"}`))
			return
		}
		json.NewEncoder(w).Encode(User{
			ID: 7, Login: "viewer", Name: "Viewer", OrgID: 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	cookies := []*http.Cookie{{Name: "grafana_session", Value: "abc123"}}
	u, err := c.ResolveUserFromSession(context.Background(), cookies)
	if err != nil {
		t.Fatalf("ResolveUserFromSession: %v", err)
	}
	if u.ID != 7 {
		t.Errorf("ID = %d, want 7", u.ID)
	}
}

func TestGetDashboard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/dashboards/uid/abc123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{
			"meta": {"slug": "test-dash", "folderTitle": "General"},
			"dashboard": {"id": 1, "uid": "abc123", "title": "Test Dashboard"}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token")
	d, err := c.GetDashboard(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if d.Meta.Slug != "test-dash" {
		t.Errorf("Slug = %q, want %q", d.Meta.Slug, "test-dash")
	}
}

func TestGetCurrentUserError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Forbidden"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-token")
	_, err := c.GetCurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}
