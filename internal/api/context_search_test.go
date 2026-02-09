package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

type mockContextSearcher struct {
	result []api.ContextEntity
	err    error
}

func (m *mockContextSearcher) SearchContext(_ context.Context, _ api.ContextEntityType, _ string) ([]api.ContextEntity, error) {
	return m.result, m.err
}

type mockUserResolver struct {
	user *grafana.User
	err  error
}

func (m *mockUserResolver) Resolve(_ context.Context, _ *http.Request) (*grafana.User, error) {
	return m.user, m.err
}

func TestContextSearchHandler_Unauthorized(t *testing.T) {
	handler := api.ContextSearchHandler(
		&mockContextSearcher{},
		&mockUserResolver{err: errors.New("no session")},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/context/search?type=datasource&q=prom", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestContextSearchHandler_InvalidType(t *testing.T) {
	handler := api.ContextSearchHandler(
		&mockContextSearcher{},
		&mockUserResolver{user: &grafana.User{ID: 1, OrgID: 1}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/context/search?type=invalid&q=test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestContextSearchHandler_MissingType(t *testing.T) {
	handler := api.ContextSearchHandler(
		&mockContextSearcher{},
		&mockUserResolver{user: &grafana.User{ID: 1, OrgID: 1}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/context/search?q=test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestContextSearchHandler_Datasources(t *testing.T) {
	entities := []api.ContextEntity{
		{Type: api.ContextEntityDatasource, ID: "prom-1", DisplayName: "Prometheus", Metadata: map[string]string{"ds_type": "prometheus"}},
	}
	handler := api.ContextSearchHandler(
		&mockContextSearcher{result: entities},
		&mockUserResolver{user: &grafana.User{ID: 1, OrgID: 1}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/context/search?type=datasource&q=prom", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp api.ContextSearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(resp.Entities))
	}
	if resp.Entities[0].ID != "prom-1" {
		t.Fatalf("expected prom-1, got %s", resp.Entities[0].ID)
	}
}

func TestContextSearchHandler_SearcherError(t *testing.T) {
	handler := api.ContextSearchHandler(
		&mockContextSearcher{err: errors.New("mcp timeout")},
		&mockUserResolver{user: &grafana.User{ID: 1, OrgID: 1}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/context/search?type=metric&q=cpu", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

func TestContextSearchHandler_MethodNotAllowed(t *testing.T) {
	handler := api.ContextSearchHandler(
		&mockContextSearcher{},
		&mockUserResolver{user: &grafana.User{ID: 1, OrgID: 1}},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/context/search?type=datasource", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
