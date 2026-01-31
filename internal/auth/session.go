package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

// SessionResolver resolves the Grafana user from an incoming HTTP request
// by forwarding the request's cookies to the Grafana API.
type SessionResolver struct {
	grafana *grafana.Client
}

// NewSessionResolver creates a new resolver backed by the given Grafana client.
func NewSessionResolver(gc *grafana.Client) *SessionResolver {
	return &SessionResolver{grafana: gc}
}

// Resolve extracts Grafana session cookies from the request and returns the
// corresponding Grafana user. Returns an error if the user cannot be resolved
// (e.g. no session cookie, expired session).
func (s *SessionResolver) Resolve(ctx context.Context, r *http.Request) (*grafana.User, error) {
	cookies := r.Cookies()
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no cookies in request")
	}

	user, err := s.grafana.ResolveUserFromSession(ctx, cookies)
	if err != nil {
		return nil, fmt.Errorf("resolve session: %w", err)
	}

	return user, nil
}
