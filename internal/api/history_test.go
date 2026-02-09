package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/brownster/grafana-assistant/internal/grafana"
	"github.com/brownster/grafana-assistant/internal/storage"
)

type stubResolver struct {
	user *grafana.User
	err  error
}

func (s stubResolver) Resolve(ctx context.Context, r *http.Request) (*grafana.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}

func newTestStore(t *testing.T) *storage.SQLite {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestHistoryListHandler(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().Truncate(time.Second)
	user := &grafana.User{ID: 10, OrgID: 1}

	_ = store.CreateSession(context.Background(), &storage.Session{
		ID: "sess-1", UserID: 10, OrgID: 1, DashboardUID: "dash-a",
		Title: "Alpha", CreatedAt: now, UpdatedAt: now,
	})
	_ = store.CreateSession(context.Background(), &storage.Session{
		ID: "sess-2", UserID: 10, OrgID: 1, DashboardUID: "dash-b",
		Title: "Beta", CreatedAt: now, UpdatedAt: now,
	})
	_ = store.CreateSession(context.Background(), &storage.Session{
		ID: "sess-3", UserID: 99, OrgID: 1,
		Title: "Other", CreatedAt: now, UpdatedAt: now,
	})

	handler := HistoryListHandler(store, stubResolver{user: user})
	req := httptest.NewRequest(http.MethodGet, "/api/history?dashboard_uid=dash-b", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var sessions []HistorySession
	if err := json.NewDecoder(rec.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "sess-2" {
		t.Fatalf("sessions = %+v, want sess-2", sessions)
	}
}

func TestHistoryDetailHandler(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().Truncate(time.Second)
	user := &grafana.User{ID: 10, OrgID: 1}

	_ = store.CreateSession(context.Background(), &storage.Session{
		ID: "sess-1", UserID: 10, OrgID: 1, DashboardUID: "dash-a",
		Title: "Alpha", CreatedAt: now, UpdatedAt: now,
	})
	_ = store.AddMessage(context.Background(), &storage.Message{
		ID: "m1", SessionID: "sess-1", Role: "user", Content: "hello", CreatedAt: now,
	})
	_ = store.AddMessage(context.Background(), &storage.Message{
		ID: "m2", SessionID: "sess-1", Role: "assistant", Content: "hi", CreatedAt: now.Add(time.Second),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/history/{id}", HistoryDetailHandler(store, stubResolver{user: user}))
	req := httptest.NewRequest(http.MethodGet, "/api/history/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var detail HistoryDetail
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.Session.ID != "sess-1" || len(detail.Messages) != 2 {
		t.Fatalf("detail = %+v", detail)
	}
}

func TestHistoryDeleteHandler(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().Truncate(time.Second)
	user := &grafana.User{ID: 10, OrgID: 1}

	_ = store.CreateSession(context.Background(), &storage.Session{
		ID: "sess-1", UserID: 10, OrgID: 1,
		Title: "Alpha", CreatedAt: now, UpdatedAt: now,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/history/{id}", HistoryDeleteHandler(store, stubResolver{user: user}))
	req := httptest.NewRequest(http.MethodDelete, "/api/history/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	sess, _ := store.GetSession(context.Background(), "sess-1")
	if sess != nil {
		t.Fatal("session should be deleted")
	}
}

func TestHistoryUnauthorized(t *testing.T) {
	store := newTestStore(t)
	handler := HistoryListHandler(store, stubResolver{err: errors.New("nope")})
	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
