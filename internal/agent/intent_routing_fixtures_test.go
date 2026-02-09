package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

const (
	routeSchemaFirst       = "schema_first"
	routeDashboardMetadata = "dashboard_metadata_first"
	routeDocsKB            = "docs_kb"
	routeNone              = "none"
)

type intentRoutingFixture struct {
	Name             string                `json:"name"`
	Message          string                `json:"message"`
	DashboardContext *api.DashboardContext `json:"dashboard_context,omitempty"`
	IsNewSession     bool                  `json:"is_new_session,omitempty"`
	DashboardChanged bool                  `json:"dashboard_changed,omitempty"`
	ExpectedIntent   IntentClass           `json:"expected_intent"`
	ExpectedRoute    string                `json:"expected_route"`
}

func TestIntentRoutingFixtures(t *testing.T) {
	fixtures := loadIntentRoutingFixtures(t)
	if len(fixtures) == 0 {
		t.Fatal("expected routing fixtures")
	}

	for _, fx := range fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			intent := ClassifyIntent(fx.Message, fx.DashboardContext)
			if intent.Label != fx.ExpectedIntent {
				t.Fatalf("intent=%q want=%q rationale=%q", intent.Label, fx.ExpectedIntent, intent.Rationale)
			}

			kbDecision := decideKBRouting(intent, fx.Message, fx.DashboardContext, fx.IsNewSession, fx.DashboardChanged)
			route := deriveRetrievalRoute(intent, kbDecision)
			if route != fx.ExpectedRoute {
				t.Fatalf("route=%q want=%q intent=%q kb_decision=%+v", route, fx.ExpectedRoute, intent.Label, kbDecision)
			}

			ordered := orderMCPToolsForIntent(sampleRoutingTools(), intent.Label)
			assertOrderedPrefixForRoute(t, fx.ExpectedRoute, ordered)
		})
	}
}

func TestIntentRoutingFixtures_CoverAllIntentClasses(t *testing.T) {
	fixtures := loadIntentRoutingFixtures(t)
	seen := map[IntentClass]bool{}
	for _, fx := range fixtures {
		seen[fx.ExpectedIntent] = true
	}

	all := []IntentClass{
		IntentLiveData,
		IntentQueryHelp,
		IntentDashboardLookup,
		IntentHowToDocs,
		IntentIncidentSummary,
	}
	for _, intent := range all {
		if !seen[intent] {
			t.Fatalf("fixture coverage missing intent %q", intent)
		}
	}
}

func loadIntentRoutingFixtures(t *testing.T) []intentRoutingFixture {
	t.Helper()
	path := filepath.Join("testdata", "intent_routing_fixtures.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixtures %s: %v", path, err)
	}

	var fixtures []intentRoutingFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("parse fixtures %s: %v", path, err)
	}
	return fixtures
}

func deriveRetrievalRoute(intent IntentResult, kbDecision kbRoutingDecision) string {
	switch {
	case shouldRouteDashboardLookup(intent.Label):
		return routeDashboardMetadata
	case shouldRouteSchemaFirst(intent.Label):
		return routeSchemaFirst
	case kbDecision.Inject:
		return routeDocsKB
	default:
		return routeNone
	}
}

func sampleRoutingTools() []mcp.Tool {
	return []mcp.Tool{
		{Name: "alertmanager__list_alerts"},
		{Name: "grafana__search_dashboards"},
		{Name: "kb__search_kb_semantic"},
		{Name: "kb__search_kb"},
		{Name: "grafana__list_datasources"},
		{Name: "grafana__list_prometheus_metric_names"},
		{Name: "grafana__query_prometheus"},
	}
}

func assertOrderedPrefixForRoute(t *testing.T, route string, ordered []mcp.Tool) {
	t.Helper()
	if len(ordered) == 0 {
		t.Fatalf("ordered tools should not be empty")
	}

	switch route {
	case routeSchemaFirst:
		if !isSchemaTool(ordered[0].Name) {
			t.Fatalf("schema route should prioritize schema tool, first=%q", ordered[0].Name)
		}
	case routeDashboardMetadata:
		if !isDashboardMetadataTool(ordered[0].Name) {
			t.Fatalf("dashboard route should prioritize metadata tool, first=%q", ordered[0].Name)
		}
	case routeDocsKB:
		if !isDocsRetrievalTool(ordered[0].Name) {
			t.Fatalf("docs route should prioritize KB retrieval tool, first=%q", ordered[0].Name)
		}
	case routeNone:
		if ordered[0].Name != "alertmanager__list_alerts" {
			t.Fatalf("none route should keep baseline order, first=%q", ordered[0].Name)
		}
	default:
		t.Fatalf("unknown route %q", route)
	}
}
