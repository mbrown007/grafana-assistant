package agent

import (
	"reflect"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

func TestShouldRouteSchemaFirst(t *testing.T) {
	tests := []struct {
		intent IntentClass
		want   bool
	}{
		{IntentLiveData, true},
		{IntentQueryHelp, true},
		{IntentDashboardLookup, false},
		{IntentHowToDocs, false},
		{IntentIncidentSummary, false},
	}

	for _, tc := range tests {
		if got := shouldRouteSchemaFirst(tc.intent); got != tc.want {
			t.Fatalf("intent=%q route=%v want=%v", tc.intent, got, tc.want)
		}
	}
}

func TestSelectSchemaTargets(t *testing.T) {
	t.Run("promql query help", func(t *testing.T) {
		targets := selectSchemaTargets(IntentQueryHelp, "Help fix this PromQL rate() query", nil)
		if !targets.prometheus || targets.loki {
			t.Fatalf("unexpected targets: %+v", targets)
		}
	})

	t.Run("logql query help", func(t *testing.T) {
		targets := selectSchemaTargets(IntentQueryHelp, "Write a LogQL query for error logs", nil)
		if !targets.loki {
			t.Fatalf("expected loki target, got %+v", targets)
		}
	})

	t.Run("explore datasource hint", func(t *testing.T) {
		targets := selectSchemaTargets(IntentQueryHelp, "improve this query", &api.DashboardContext{
			Explore: &api.ExploreContext{Datasource: "loki"},
		})
		if !targets.loki {
			t.Fatalf("expected loki target from explore context, got %+v", targets)
		}
	})

	t.Run("default to prometheus", func(t *testing.T) {
		targets := selectSchemaTargets(IntentLiveData, "what should I check", nil)
		if !targets.prometheus {
			t.Fatalf("expected prometheus default target, got %+v", targets)
		}
	})
}

func TestOrderMCPToolsForIntent_SchemaFirst(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__query_prometheus"},
		{Name: "grafana__list_prometheus_metric_names"},
		{Name: "alertmanager__list_alerts"},
		{Name: "grafana__list_datasources"},
	}

	ordered := orderMCPToolsForIntent(tools, IntentQueryHelp)
	got := []string{ordered[0].Name, ordered[1].Name, ordered[2].Name, ordered[3].Name}
	want := []string{
		"grafana__list_prometheus_metric_names",
		"grafana__list_datasources",
		"grafana__query_prometheus",
		"alertmanager__list_alerts",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered=%v want=%v", got, want)
	}
}

func TestOrderMCPToolsForIntent_NonSchemaIntentKeepsOrder(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__query_prometheus"},
		{Name: "grafana__list_datasources"},
		{Name: "alertmanager__list_alerts"},
	}
	ordered := orderMCPToolsForIntent(tools, IntentHowToDocs)
	for i := range tools {
		if tools[i].Name != ordered[i].Name {
			t.Fatalf("order changed at %d: got=%q want=%q", i, ordered[i].Name, tools[i].Name)
		}
	}
}

func TestParseDatasourceSummaries(t *testing.T) {
	input := `[
		{"uid":"prometheus","name":"Prometheus","type":"prometheus","isDefault":true},
		{"uid":"loki","name":"Loki","type":"loki","isDefault":false}
	]`

	out := parseDatasourceSummaries(input)
	if len(out) != 2 {
		t.Fatalf("len(out)=%d want=2", len(out))
	}
	if out[0].UID != "prometheus" || out[0].Type != "prometheus" || !out[0].IsDefault {
		t.Fatalf("unexpected first datasource: %+v", out[0])
	}
	if out[1].UID != "loki" || out[1].Type != "loki" {
		t.Fatalf("unexpected second datasource: %+v", out[1])
	}
}

func TestParseStringList(t *testing.T) {
	jsonInput := `["job","instance","pod"]`
	if got := parseStringList(jsonInput); !reflect.DeepEqual(got, []string{"job", "instance", "pod"}) {
		t.Fatalf("json parse got=%v", got)
	}

	spaceInput := `[job instance pod]`
	if got := parseStringList(spaceInput); !reflect.DeepEqual(got, []string{"job", "instance", "pod"}) {
		t.Fatalf("space parse got=%v", got)
	}

	csvInput := "job, instance, pod"
	if got := parseStringList(csvInput); !reflect.DeepEqual(got, []string{"job", "instance", "pod"}) {
		t.Fatalf("csv parse got=%v", got)
	}
}
