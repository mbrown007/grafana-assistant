package context

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/grafana"
)

const (
	maxPanels          = 30
	maxQueriesPerPanel = 5
	maxQueryLength     = 500
)

// PanelSummary is a minified representation of a Grafana panel for LLM consumption.
type PanelSummary struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Queries     []string `json:"queries,omitempty"`
}

// DashboardSummary is a minified representation of a Grafana dashboard.
type DashboardSummary struct {
	UID    string         `json:"uid"`
	Title  string         `json:"title"`
	Folder string         `json:"folder,omitempty"`
	Tags   []string       `json:"tags,omitempty"`
	Panels []PanelSummary `json:"panels"`
}

type cacheEntry struct {
	summary   *DashboardSummary
	expiresAt time.Time
}

// Enricher fetches Grafana dashboards and produces minified summaries with caching.
type Enricher struct {
	client   *grafana.Client
	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[string]*cacheEntry
}

// NewEnricher creates a new Enricher with the given Grafana client and cache TTL.
func NewEnricher(client *grafana.Client, cacheTTL time.Duration) *Enricher {
	return &Enricher{
		client:   client,
		cacheTTL: cacheTTL,
		cache:    make(map[string]*cacheEntry),
	}
}

// GetDashboardSummary returns a minified summary of the dashboard with the given UID.
// Results are cached for the configured TTL duration.
func (e *Enricher) GetDashboardSummary(ctx context.Context, uid string) (*DashboardSummary, error) {
	// Check cache.
	e.mu.RLock()
	if entry, ok := e.cache[uid]; ok && time.Now().Before(entry.expiresAt) {
		e.mu.RUnlock()
		slog.Debug("dashboard cache hit", "uid", uid)
		return entry.summary, nil
	}
	e.mu.RUnlock()

	// Fetch from Grafana.
	dash, err := e.client.GetDashboard(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("fetch dashboard %s: %w", uid, err)
	}

	summary, err := buildSummary(dash)
	if err != nil {
		return nil, fmt.Errorf("parse dashboard %s: %w", uid, err)
	}

	// Store in cache.
	e.mu.Lock()
	e.cache[uid] = &cacheEntry{
		summary:   summary,
		expiresAt: time.Now().Add(e.cacheTTL),
	}
	e.mu.Unlock()

	slog.Debug("dashboard cached", "uid", uid, "panels", len(summary.Panels))
	return summary, nil
}

// dashboardJSON is the structure inside the "dashboard" field of the Grafana API response.
type dashboardJSON struct {
	UID    string          `json:"uid"`
	Title  string          `json:"title"`
	Tags   []string        `json:"tags"`
	Panels []panelJSON     `json:"panels"`
}

type panelJSON struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Type        string      `json:"type"`
	Targets     []targetJSON `json:"targets"`
	Panels      []panelJSON `json:"panels"` // nested panels in row type
}

type targetJSON struct {
	Expr   string `json:"expr"`   // PromQL
	RawSQL string `json:"rawSql"` // SQL
}

func buildSummary(dash *grafana.Dashboard) (*DashboardSummary, error) {
	var d dashboardJSON
	if err := json.Unmarshal(dash.Dashboard, &d); err != nil {
		return nil, err
	}

	summary := &DashboardSummary{
		UID:    d.UID,
		Title:  d.Title,
		Folder: dash.Meta.Folder,
		Tags:   d.Tags,
		Panels: make([]PanelSummary, 0),
	}

	flatPanels := flattenPanels(d.Panels)

	// Enforce max panels limit.
	if len(flatPanels) > maxPanels {
		flatPanels = flatPanels[:maxPanels]
	}

	for _, p := range flatPanels {
		ps := PanelSummary{
			Title:       p.Title,
			Description: p.Description,
			Type:        p.Type,
			Queries:     extractQueries(p.Targets),
		}
		summary.Panels = append(summary.Panels, ps)
	}

	return summary, nil
}

// flattenPanels expands row panels that contain nested panels.
func flattenPanels(panels []panelJSON) []panelJSON {
	var result []panelJSON
	for _, p := range panels {
		if p.Type == "row" && len(p.Panels) > 0 {
			result = append(result, p.Panels...)
		} else {
			result = append(result, p)
		}
	}
	return result
}

func extractQueries(targets []targetJSON) []string {
	var queries []string
	for _, t := range targets {
		q := t.Expr
		if q == "" {
			q = t.RawSQL
		}
		if q == "" {
			continue
		}
		if len(q) > maxQueryLength {
			q = q[:maxQueryLength]
		}
		queries = append(queries, q)
		if len(queries) >= maxQueriesPerPanel {
			break
		}
	}
	return queries
}
