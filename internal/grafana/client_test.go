package grafana

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSearchDashboards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("type"); got != "dash-db" {
			t.Errorf("type = %q, want %q", got, "dash-db")
		}
		tags := r.URL.Query()["tag"]
		if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
			t.Errorf("tags = %v, want [a b]", tags)
		}
		json.NewEncoder(w).Encode([]DashboardHit{{UID: "uid-1", Title: "Dash"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token")
	hits, err := c.SearchDashboards(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("SearchDashboards: %v", err)
	}
	if len(hits) != 1 || hits[0].UID != "uid-1" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestCreateFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/folders" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "\"title\":\"Assistant Scratchpads\"") {
			t.Errorf("unexpected body: %s", string(body))
		}
		json.NewEncoder(w).Encode(Folder{ID: 1, UID: "folder-1", Title: "Assistant Scratchpads"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token")
	f, err := c.CreateFolder(context.Background(), "Assistant Scratchpads")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if f.UID != "folder-1" {
		t.Fatalf("unexpected folder: %+v", f)
	}
}

func TestSaveAndDeleteDashboard(t *testing.T) {
	var gotDelete bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/dashboards/db":
			json.NewEncoder(w).Encode(DashboardSaveResponse{UID: "dash-1", Slug: "dash", Status: "success"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/dashboards/uid/dash-1":
			gotDelete = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token")
	resp, err := c.CreateDashboard(context.Background(), map[string]any{"title": "Scratchpad"}, "folder-1", true)
	if err != nil {
		t.Fatalf("CreateDashboard: %v", err)
	}
	if resp.UID != "dash-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if err := c.DeleteDashboard(context.Background(), "dash-1"); err != nil {
		t.Fatalf("DeleteDashboard: %v", err)
	}
	if !gotDelete {
		t.Fatal("delete not called")
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
