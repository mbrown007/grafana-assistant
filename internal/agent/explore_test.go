package agent

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/api"
)

func TestRefIDFromIndex(t *testing.T) {
	cases := map[int]string{
		0:  "A",
		25: "Z",
		26: "AA",
		27: "AB",
		51: "AZ",
		52: "BA",
	}
	for index, want := range cases {
		if got := refIDFromIndex(index); got != want {
			t.Errorf("refIDFromIndex(%d) = %q, want %q", index, got, want)
		}
	}
}

func TestBuildExploreURLDefaults(t *testing.T) {
	reqCtx := &api.DashboardContext{
		TimeRange: map[string]string{"from": "now-2h", "to": "now"},
	}
	urlStr, err := buildExploreURL(map[string]any{"query": "up"}, reqCtx)
	if err != nil {
		t.Fatalf("buildExploreURL: %v", err)
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	panesParam := parsed.Query().Get("panes")
	if panesParam == "" {
		t.Fatalf("expected panes param")
	}
	var panes map[string]any
	if err := json.Unmarshal([]byte(panesParam), &panes); err != nil {
		t.Fatalf("parse panes: %v", err)
	}
	pane, ok := panes["A"].(map[string]any)
	if !ok {
		t.Fatalf("expected pane A")
	}
	rangeMap := pane["range"].(map[string]any)
	if rangeMap["from"] != "now-2h" || rangeMap["to"] != "now" {
		t.Fatalf("unexpected time range: %#v", rangeMap)
	}
	queries := pane["queries"].([]any)
	query := queries[0].(map[string]any)
	if query["expr"] != "up" {
		t.Fatalf("unexpected query expr: %#v", query)
	}
}

func TestBuildExploreURLBadArgs(t *testing.T) {
	_, err := buildExploreURL(map[string]any{"query": make(chan int)}, nil)
	if err == nil {
		t.Fatalf("expected error for unmarshalable args")
	}
}

func TestGuessDatasourceType(t *testing.T) {
	if got := guessDatasourceType("loki"); got != "loki" {
		t.Fatalf("expected loki, got %q", got)
	}
	if got := guessDatasourceType("tempo"); got != "tempo" {
		t.Fatalf("expected tempo, got %q", got)
	}
	if got := guessDatasourceType("prometheus"); got != "prometheus" {
		t.Fatalf("expected prometheus, got %q", got)
	}
	if got := guessDatasourceType("custom-prom"); !strings.Contains(got, "prom") {
		t.Fatalf("expected prometheus fallback, got %q", got)
	}
}
