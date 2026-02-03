package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/storage"
)

// FeedbackHandler accepts user feedback on assistant replies.
func FeedbackHandler(store storage.Store, resolver UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		user, err := resolver.Resolve(r.Context(), r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req FeedbackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.SessionID == "" || req.MessageID == "" {
			http.Error(w, "session_id and message_id are required", http.StatusBadRequest)
			return
		}
		if req.Rating < 1 || req.Rating > 5 {
			http.Error(w, "rating must be between 1 and 5", http.StatusBadRequest)
			return
		}

		sess, err := store.GetSession(r.Context(), req.SessionID)
		if err != nil {
			http.Error(w, "failed to load session", http.StatusInternalServerError)
			return
		}
		if sess == nil || sess.UserID != user.ID || sess.OrgID != user.OrgID {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		now := time.Now()
		feedback := &storage.Feedback{
			ID:        fmt.Sprintf("%s-%d-feedback", req.SessionID, now.UnixMilli()),
			SessionID: req.SessionID,
			MessageID: req.MessageID,
			UserID:    user.ID,
			OrgID:     user.OrgID,
			Rating:    req.Rating,
			Comment:   req.Comment,
			CreatedAt: now,
		}
		if err := store.AddFeedback(r.Context(), feedback); err != nil {
			http.Error(w, "failed to store feedback", http.StatusInternalServerError)
			return
		}

		payload := map[string]any{
			"message_id": req.MessageID,
			"rating":     req.Rating,
			"comment":    req.Comment,
		}
		data, _ := json.Marshal(payload)
		_ = store.AddAuditEntry(r.Context(), &storage.AuditEntry{
			ID:           fmt.Sprintf("%s-%d-audit-feedback", req.SessionID, now.UnixMilli()),
			SessionID:    req.SessionID,
			UserID:       user.ID,
			OrgID:        user.OrgID,
			DashboardUID: sess.DashboardUID,
			EventType:    "user_feedback",
			Request:      string(data),
			CreatedAt:    now,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}
