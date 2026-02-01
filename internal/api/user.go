package api

import (
	"encoding/json"
	"net/http"
)

// CurrentUser is the Grafana user identity for the current session.
type CurrentUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	OrgID int64  `json:"org_id"`
}

// CurrentUserHandler returns the Grafana user for the current session.
func CurrentUserHandler(resolver UserResolver) http.HandlerFunc {
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

		resp := CurrentUser{
			ID:    user.ID,
			Login: user.Login,
			Name:  user.Name,
			Email: user.Email,
			OrgID: user.OrgID,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
