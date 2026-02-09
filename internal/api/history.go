package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/brownster/grafana-assistant/internal/grafana"
	"github.com/brownster/grafana-assistant/internal/storage"
)

// UserResolver resolves the Grafana user for a request.
type UserResolver interface {
	Resolve(ctx context.Context, r *http.Request) (*grafana.User, error)
}

// HistoryListHandler returns all sessions for the current Grafana user.
func HistoryListHandler(store storage.Store, resolver UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		user, err := resolver.Resolve(r.Context(), r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		dashboardUID := r.URL.Query().Get("dashboard_uid")
		sessions, err := store.ListSessions(r.Context(), user.ID, user.OrgID, dashboardUID)
		if err != nil {
			http.Error(w, "failed to load history", http.StatusInternalServerError)
			return
		}

		resp := make([]HistorySession, 0, len(sessions))
		for _, sess := range sessions {
			resp = append(resp, HistorySession{
				ID:          sess.ID,
				Title:       sess.Title,
				DashboardUID: sess.DashboardUID,
				CreatedAt:   sess.CreatedAt,
				UpdatedAt:   sess.UpdatedAt,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HistoryDetailHandler returns a session and its messages for the current user.
func HistoryDetailHandler(store storage.Store, resolver UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		user, err := resolver.Resolve(r.Context(), r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		sess, err := store.GetSession(r.Context(), id)
		if err != nil {
			http.Error(w, "failed to load session", http.StatusInternalServerError)
			return
		}
		if sess == nil || sess.UserID != user.ID || sess.OrgID != user.OrgID {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		msgs, err := store.GetMessages(r.Context(), id)
		if err != nil {
			http.Error(w, "failed to load messages", http.StatusInternalServerError)
			return
		}

		resp := HistoryDetail{
			Session: HistorySession{
				ID:          sess.ID,
				Title:       sess.Title,
				DashboardUID: sess.DashboardUID,
				CreatedAt:   sess.CreatedAt,
				UpdatedAt:   sess.UpdatedAt,
			},
		}
		resp.Messages = make([]HistoryMessage, 0, len(msgs))
		for _, msg := range msgs {
			resp.Messages = append(resp.Messages, HistoryMessage{
				ID:        msg.ID,
				Role:      msg.Role,
				Content:   msg.Content,
				CreatedAt: msg.CreatedAt,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HistoryDeleteHandler deletes a session for the current user.
func HistoryDeleteHandler(store storage.Store, resolver UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		user, err := resolver.Resolve(r.Context(), r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}

		sess, err := store.GetSession(r.Context(), id)
		if err != nil {
			http.Error(w, "failed to load session", http.StatusInternalServerError)
			return
		}
		if sess == nil || sess.UserID != user.ID || sess.OrgID != user.OrgID {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if err := store.DeleteSession(r.Context(), id); err != nil {
			http.Error(w, "failed to delete session", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
