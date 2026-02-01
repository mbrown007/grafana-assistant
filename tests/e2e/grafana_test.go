//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestGrafanaMCP(t *testing.T) {
	requireService(t, grafanaURL, "/grafana/api/health")

	// Build and start the Grafana MCP server.
	// The Grafana MCP uses CLI flags for transport/address and env vars for config.
	bin := buildMCPServer(t, "grafana-mcp",
		"mcp_servers/mcp-grafana/cmd/mcp-grafana", "mcp-grafana-e2e")
	startMCPServer(t, bin, []string{
		fmt.Sprintf("GRAFANA_URL=%s/grafana", grafanaURL),
	}, mcpGrafanaPort,
		"-transport", "sse",
		"-address", fmt.Sprintf("localhost:%d", mcpGrafanaPort),
	)

	// Connect MCP client.
	client := connectMCPClient(t,
		fmt.Sprintf("http://localhost:%d", mcpGrafanaPort), "grafana")

	t.Run("DiscoverTools", func(t *testing.T) {
		tools, err := client.DiscoverTools(context.Background())
		if err != nil {
			t.Fatalf("discover tools: %v", err)
		}
		if len(tools) == 0 {
			t.Fatal("expected at least one tool from Grafana MCP")
		}
		// Verify a few expected tools exist.
		expected := []string{"search_dashboards", "list_datasources"}
		toolNames := toolNameSet(tools)
		for _, name := range expected {
			prefixed := "grafana__" + name
			if !toolNames[prefixed] {
				t.Errorf("expected tool %q not found; available: %v", prefixed, toolNameKeys(toolNames))
			}
		}
		t.Logf("discovered %d Grafana tools", len(tools))
	})

	t.Run("ListDatasources", func(t *testing.T) {
		result, err := client.InvokeTool(context.Background(),
			"grafana__list_datasources", nil)
		if err != nil {
			t.Fatalf("list_datasources: %v", err)
		}
		text, ok := result.(string)
		if !ok {
			t.Fatalf("expected string result, got %T", result)
		}
		// The dev-test Grafana provisioning includes a Prometheus datasource.
		if !findInJSON(text, "prometheus") && !findInJSON(text, "Prometheus") {
			t.Errorf("expected Prometheus datasource in response: %s", truncate(text, 300))
		}
	})

	t.Run("SearchDashboards", func(t *testing.T) {
		result, err := client.InvokeTool(context.Background(),
			"grafana__search_dashboards", map[string]any{})
		if err != nil {
			t.Fatalf("search_dashboards: %v", err)
		}
		text, ok := result.(string)
		if !ok {
			t.Fatalf("expected string result, got %T", result)
		}
		// Verify the response is valid JSON (array or object).
		if !json.Valid([]byte(text)) {
			t.Errorf("search_dashboards returned invalid JSON: %s", truncate(text, 200))
		}
	})
}

func toolNameKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
