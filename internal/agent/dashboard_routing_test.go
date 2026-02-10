package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/api"
	"github.com/brownster/grafana-assistant/internal/mcp"
)

type dashboardRoutingMockClient struct {
	tools     []mcp.Tool
	responses map[string]func(args map[string]any) (any, error)
	called    map[string]int
}

func (m *dashboardRoutingMockClient) Connect(context.Context) error { return nil }
func (m *dashboardRoutingMockClient) Health(context.Context) error  { return nil }
func (m *dashboardRoutingMockClient) DiscoverTools(context.Context) ([]mcp.Tool, error) {
	return m.tools, nil
}
func (m *dashboardRoutingMockClient) InvokeTool(_ context.Context, name string, args map[string]any) (any, error) {
	if m.called == nil {
		m.called = map[string]int{}
	}
	m.called[name]++
	if m.responses == nil {
		return nil, errors.New("no responses configured")
	}
	fn, ok := m.responses[name]
	if !ok {
		return nil, errors.New("no response for tool " + name)
	}
	return fn(args)
}

func TestBuildDashboardMetadataQueries_UIDTitleTagPaths(t *testing.T) {
	ctx := &api.DashboardContext{
		UID:  "fe9gm6guyzi0wd",
		Name: "Payments API SLO",
		Tags: []string{"payments", "prod"},
	}
	queries := buildDashboardMetadataQueries(`Find dashboard uid fe9gm6guyzi0wd titled "Payments API SLO"`, ctx)

	mustContain := []string{
		"fe9gm6guyzi0wd",
		"Payments API SLO",
		"payments",
		"prod",
	}
	for _, expected := range mustContain {
		if !containsString(queries, expected) {
			t.Fatalf("queries missing %q: %v", expected, queries)
		}
	}
}

func TestBuildDashboardMetadataQueries_CapsQueryCount(t *testing.T) {
	ctx := &api.DashboardContext{
		UID:  "uid-a",
		Name: "dashboard-a",
		Tags: []string{"t1", "t2", "t3", "t4", "t5"},
	}
	queries := buildDashboardMetadataQueries(`Find dashboard uid uid-a titled "Payments API SLO" for prod checkout service`, ctx)
	if len(queries) > dashboardLookupMaxMetadataQuery {
		t.Fatalf("expected at most %d queries, got %d: %v", dashboardLookupMaxMetadataQuery, len(queries), queries)
	}
}

func TestBuildDashboardLookupContext_MetadataHitDoesNotUseSemantic(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__search_dashboards"},
		{Name: "kb__search_kb_semantic"},
	}
	mock := &dashboardRoutingMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__search_dashboards": func(args map[string]any) (any, error) {
				q, _ := args["query"].(string)
				if strings.Contains(strings.ToLower(q), "payments") {
					return `[{
						"uid":"pay123",
						"title":"Payments API SLO",
						"folderTitle":"SRE",
						"tags":["payments","prod"],
						"url":"/d/pay123/payments-api-slo"
					}]`, nil
				}
				return `[]`, nil
			},
			"kb__search_kb_semantic": func(args map[string]any) (any, error) {
				return `[{
					"path":"platform/payments.md",
					"title":"Payments overview",
					"snippet":"semantic candidate",
					"score":0.81
				}]`, nil
			},
		},
	}

	mgr := &Manager{mcp: []mcp.Client{mock}, tools: tools}
	contextText, usedSemantic := mgr.buildDashboardLookupContext(context.Background(), IntentResult{Label: IntentDashboardLookup}, "find payments dashboard", nil)

	if usedSemantic {
		t.Fatalf("expected metadata path only, semantic fallback should be false")
	}
	if !strings.Contains(contextText, "Dashboard metadata matches") {
		t.Fatalf("expected metadata context, got: %s", contextText)
	}
	if mock.called["kb__search_kb_semantic"] != 0 {
		t.Fatalf("semantic tool should not be called on metadata hit, calls=%d", mock.called["kb__search_kb_semantic"])
	}
}

func TestBuildDashboardLookupContext_UsesSemanticFallbackWhenMetadataMisses(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__search_dashboards"},
		{Name: "kb__search_kb_semantic"},
	}
	mock := &dashboardRoutingMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__search_dashboards": func(args map[string]any) (any, error) {
				return `[]`, nil
			},
			"kb__search_kb_semantic": func(args map[string]any) (any, error) {
				return `[{
					"path":"platform/monitoring-assistant/overview.md",
					"title":"Monitoring Assistant Overview",
					"snippet":"contains dashboard discovery guidance",
					"score":0.72
				}]`, nil
			},
		},
	}

	mgr := &Manager{mcp: []mcp.Client{mock}, tools: tools}
	contextText, usedSemantic := mgr.buildDashboardLookupContext(context.Background(), IntentResult{Label: IntentDashboardLookup}, "find checkout dashboard", nil)

	if !usedSemantic {
		t.Fatalf("expected semantic fallback to be used")
	}
	if !strings.Contains(contextText, "semantic fallback") {
		t.Fatalf("expected semantic fallback context, got: %s", contextText)
	}
	if mock.called["kb__search_kb_semantic"] == 0 {
		t.Fatalf("expected semantic tool to be called")
	}
}

func TestOrderMCPToolsForIntent_DashboardLookupPrioritizesMetadataThenSemantic(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__query_prometheus"},
		{Name: "kb__search_kb_semantic"},
		{Name: "grafana__search_dashboards"},
		{Name: "grafana__list_datasources"},
	}
	ordered := orderMCPToolsForIntent(tools, IntentDashboardLookup)

	if len(ordered) != len(tools) {
		t.Fatalf("unexpected ordered length %d", len(ordered))
	}
	if ordered[0].Name != "grafana__search_dashboards" {
		t.Fatalf("expected metadata tool first, got %q", ordered[0].Name)
	}
	if ordered[1].Name != "kb__search_kb_semantic" {
		t.Fatalf("expected semantic tool second, got %q", ordered[1].Name)
	}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
