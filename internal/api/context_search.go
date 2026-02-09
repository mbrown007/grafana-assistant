package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// ContextSearcher searches for context entities using MCP tools.
type ContextSearcher interface {
	SearchContext(ctx context.Context, entityType ContextEntityType, query string) ([]ContextEntity, error)
}

// ContextSearchHandler returns an HTTP handler for GET /api/context/search?type=X&q=Y.
func ContextSearchHandler(searcher ContextSearcher, resolver UserResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		_, err := resolver.Resolve(r.Context(), r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		entityType := ContextEntityType(r.URL.Query().Get("type"))
		query := r.URL.Query().Get("q")

		switch entityType {
		case ContextEntityDatasource, ContextEntityDashboard, ContextEntityMetric, ContextEntityLabel:
			// valid
		default:
			http.Error(w, "invalid or missing type parameter", http.StatusBadRequest)
			return
		}

		entities, err := searcher.SearchContext(r.Context(), entityType, query)
		if err != nil {
			slog.ErrorContext(r.Context(), "context search failed",
				"type", entityType,
				"query", query,
				"error", err,
			)
			http.Error(w, "search failed", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ContextSearchResponse{Entities: entities})
	}
}
