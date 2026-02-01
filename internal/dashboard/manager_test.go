package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

type apiState struct {
	mu        sync.Mutex
	dashboard map[string]any
	lastSave  map[string]any
}

func TestGetOrCreateScratchpadCreates(t *testing.T) {
	state := &apiState{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/search":
			json.NewEncoder(w).Encode([]grafana.DashboardHit{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/folders":
			json.NewEncoder(w).Encode([]grafana.Folder{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/folders":
			json.NewEncoder(w).Encode(grafana.Folder{ID: 1, UID: "folder-1", Title: "Assistant Scratchpads"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dashboards/db":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode save payload: %v", err)
			}
			state.mu.Lock()
			state.lastSave = payload
			state.mu.Unlock()
			json.NewEncoder(w).Encode(grafana.DashboardSaveResponse{UID: "dash-1", Slug: "dash-1", Status: "success", URL: "/d/dash-1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := grafana.NewClient(srv.URL, "token")
	mgr := NewManager(client, "Assistant Scratchpads")
	user := &grafana.User{ID: 42, Name: "Ada"}

	uid, panelID, url, err := mgr.GetOrCreateScratchpad(context.Background(), user, "session-123456")
	if err != nil {
		t.Fatalf("GetOrCreateScratchpad: %v", err)
	}
	if uid != "dash-1" || panelID != 1 || url != "/grafana/d/dash-1" {
		t.Fatalf("unexpected response: uid=%s panelID=%d url=%s", uid, panelID, url)
	}

	state.mu.Lock()
	payload := state.lastSave
	state.mu.Unlock()
	if payload == nil {
		t.Fatal("expected dashboard save payload")
	}
	db, ok := payload["dashboard"].(map[string]any)
	if !ok {
		t.Fatalf("dashboard payload missing: %+v", payload)
	}
	title, _ := db["title"].(string)
	if !strings.Contains(title, "Ada") {
		t.Fatalf("expected title to include user name, got %q", title)
	}
	tags, ok := db["tags"].([]any)
	if !ok {
		t.Fatalf("tags missing or invalid: %+v", db["tags"])
	}
	var hasScratchpadTag, hasUserTag, hasSessionTag, hasLastUsed bool
	for _, tag := range tags {
		s, _ := tag.(string)
		switch {
		case s == ScratchpadTag:
			hasScratchpadTag = true
		case strings.HasPrefix(s, UserIDTagPrefix):
			hasUserTag = true
		case strings.HasPrefix(s, SessionIDTagPrefix):
			hasSessionTag = true
		case strings.HasPrefix(s, LastUsedTagPrefix):
			hasLastUsed = true
		}
	}
	if !hasScratchpadTag || !hasUserTag || !hasSessionTag || !hasLastUsed {
		t.Fatalf("missing expected tags: %v", tags)
	}
}

func TestUpdatePanelUpdatesQueryAndTags(t *testing.T) {
	state := &apiState{
		dashboard: map[string]any{
			"uid":   "dash-1",
			"title": "Scratchpad",
			"tags":  []any{ScratchpadTag, LastUsedTagPrefix + "1"},
			"panels": []any{
				map[string]any{
					"id":    float64(1),
					"title": "Old",
					"targets": []any{
						map[string]any{"refId": "A", "expr": "old"},
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/dashboards/uid/dash-1":
			state.mu.Lock()
			defer state.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{
				"meta":      map[string]any{"folderUid": "folder-1"},
				"dashboard": state.dashboard,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dashboards/db":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode save payload: %v", err)
			}
			state.mu.Lock()
			state.lastSave = payload
			state.mu.Unlock()
			json.NewEncoder(w).Encode(grafana.DashboardSaveResponse{UID: "dash-1", Slug: "dash-1", Status: "success"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := grafana.NewClient(srv.URL, "token")
	mgr := NewManager(client, "Assistant Scratchpads")

	err := mgr.UpdatePanel(context.Background(), "dash-1", 1, "new_query", "New Title", "Desc", "stat", nil, nil)
	if err != nil {
		t.Fatalf("UpdatePanel: %v", err)
	}

	state.mu.Lock()
	payload := state.lastSave
	state.mu.Unlock()
	db, ok := payload["dashboard"].(map[string]any)
	if !ok {
		t.Fatalf("dashboard payload missing")
	}
	panels, _ := db["panels"].([]any)
	panel := panels[0].(map[string]any)
	if panel["title"] != "New Title" {
		t.Fatalf("expected updated title, got %v", panel["title"])
	}
	if panel["type"] != "stat" {
		t.Fatalf("expected panel type to be stat, got %v", panel["type"])
	}
	targets := panel["targets"].([]any)
	target := targets[0].(map[string]any)
	if target["expr"] != "new_query" {
		t.Fatalf("expected updated query, got %v", target["expr"])
	}
	tags := db["tags"].([]any)
	var hasLastUsed bool
	for _, tag := range tags {
		s, _ := tag.(string)
		if strings.HasPrefix(s, LastUsedTagPrefix) {
			hasLastUsed = true
		}
	}
	if !hasLastUsed {
		t.Fatalf("expected last-used tag update, got %v", tags)
	}
}

func TestTouchLastUsed(t *testing.T) {
	state := &apiState{
		dashboard: map[string]any{
			"uid":   "dash-1",
			"title": "Scratchpad",
			"tags":  []any{ScratchpadTag, LastUsedTagPrefix + "1"},
			"panels": []any{
				map[string]any{"id": float64(1)},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/dashboards/uid/dash-1":
			state.mu.Lock()
			defer state.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{
				"meta":      map[string]any{"folderUid": "folder-1"},
				"dashboard": state.dashboard,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/dashboards/db":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode save payload: %v", err)
			}
			state.mu.Lock()
			state.lastSave = payload
			state.mu.Unlock()
			json.NewEncoder(w).Encode(grafana.DashboardSaveResponse{UID: "dash-1", Status: "success"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := grafana.NewClient(srv.URL, "token")
	mgr := NewManager(client, "Assistant Scratchpads")
	if err := mgr.TouchLastUsed(context.Background(), "dash-1", time.Unix(100, 0)); err != nil {
		t.Fatalf("TouchLastUsed: %v", err)
	}

	state.mu.Lock()
	payload := state.lastSave
	state.mu.Unlock()
	db, ok := payload["dashboard"].(map[string]any)
	if !ok {
		t.Fatalf("dashboard payload missing")
	}
	tags := db["tags"].([]any)
	var hasLastUsed bool
	for _, tag := range tags {
		s, _ := tag.(string)
		if s == LastUsedTagPrefix+"100" {
			hasLastUsed = true
		}
	}
	if !hasLastUsed {
		t.Fatalf("expected last-used tag to be updated, got %v", tags)
	}
}
