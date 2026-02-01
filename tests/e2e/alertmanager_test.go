//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

func TestAlertmanagerMCP(t *testing.T) {
	requireService(t, alertmanagerURL, "/-/healthy")
	// Note: Grafana not needed for this test, only Alertmanager.

	// Build and start the Alertmanager MCP server.
	bin := buildMCPServer(t, "alertmanager-mcp",
		"mcp_servers/alertmanager-mcp-go/cmd/server", "mcp-alertmanager-e2e")
	startMCPServer(t, bin, []string{
		"MCP_TRANSPORT=sse",
		fmt.Sprintf("MCP_PORT=%d", mcpAlertmanagerPort),
		fmt.Sprintf("ALERTMANAGER_URL=%s", alertmanagerURL),
	}, mcpAlertmanagerPort)

	// Connect MCP client.
	client := connectMCPClient(t,
		fmt.Sprintf("http://localhost:%d", mcpAlertmanagerPort), "alertmanager")

	t.Run("DiscoverTools", func(t *testing.T) {
		tools, err := client.DiscoverTools(context.Background())
		if err != nil {
			t.Fatalf("discover tools: %v", err)
		}
		expected := []string{
			"get_status", "get_alerts", "post_alerts",
			"get_silences", "post_silence", "delete_silence",
			"get_receivers", "get_alert_groups",
		}
		toolNames := toolNameSet(tools)
		for _, name := range expected {
			// Tools are prefixed with "alertmanager__" by the client.
			prefixed := "alertmanager__" + name
			if !toolNames[prefixed] {
				t.Errorf("expected tool %q not found in %v", prefixed, toolNames)
			}
		}
	})

	t.Run("GetStatus", func(t *testing.T) {
		result, err := client.InvokeTool(context.Background(),
			"alertmanager__get_status", nil)
		if err != nil {
			t.Fatalf("get_status: %v", err)
		}
		text, ok := result.(string)
		if !ok {
			t.Fatalf("expected string result, got %T", result)
		}
		if !jsonContainsKey(text, "cluster") && !jsonContainsKey(text, "versionInfo") {
			t.Errorf("get_status response missing expected keys: %s", truncate(text, 200))
		}
	})

	t.Run("GetReceivers", func(t *testing.T) {
		result, err := client.InvokeTool(context.Background(),
			"alertmanager__get_receivers", nil)
		if err != nil {
			t.Fatalf("get_receivers: %v", err)
		}
		text, ok := result.(string)
		if !ok {
			t.Fatalf("expected string result, got %T", result)
		}
		// Default Alertmanager config has at least a "default" receiver.
		var receivers []any
		if err := json.Unmarshal([]byte(text), &receivers); err != nil {
			t.Fatalf("parse receivers: %v (body: %s)", err, truncate(text, 200))
		}
		if len(receivers) == 0 {
			t.Error("expected at least one receiver")
		}
	})

	t.Run("AlertLifecycle", func(t *testing.T) {
		ctx := context.Background()
		uniqueLabel := fmt.Sprintf("e2e-%d", time.Now().UnixNano())

		// Post a test alert.
		_, err := client.InvokeTool(ctx, "alertmanager__post_alerts", map[string]any{
			"alerts": []any{
				map[string]any{
					"labels": map[string]any{
						"alertname": "E2ETestAlert",
						"e2e_run":   uniqueLabel,
					},
					"annotations": map[string]any{
						"summary": "E2E test alert",
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("post_alerts: %v", err)
		}

		// Verify the alert appears.
		result, err := client.InvokeTool(ctx, "alertmanager__get_alerts", map[string]any{
			"active": true,
		})
		if err != nil {
			t.Fatalf("get_alerts: %v", err)
		}
		text, ok := result.(string)
		if !ok {
			t.Fatalf("expected string, got %T", result)
		}
		// The paginated response is an object with "data" array.
		var alertResp struct {
			Data []struct {
				Labels map[string]string `json:"labels"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(text), &alertResp); err != nil {
			// Might be a plain array.
			var alerts []struct {
				Labels map[string]string `json:"labels"`
			}
			if err2 := json.Unmarshal([]byte(text), &alerts); err2 != nil {
				t.Fatalf("parse alerts: %v (body: %s)", err, truncate(text, 300))
			}
			alertResp.Data = alerts
		}

		found := false
		for _, a := range alertResp.Data {
			if a.Labels["e2e_run"] == uniqueLabel {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("test alert with e2e_run=%s not found in alerts response", uniqueLabel)
		}
	})

	t.Run("SilenceLifecycle", func(t *testing.T) {
		ctx := context.Background()
		now := time.Now().UTC()

		// Create a silence.
		result, err := client.InvokeTool(ctx, "alertmanager__post_silence", map[string]any{
			"silence": map[string]any{
				"matchers": []any{
					map[string]any{
						"name":    "alertname",
						"value":   "E2ETestSilence",
						"isRegex": false,
						"isEqual": true,
					},
				},
				"startsAt":  now.Format(time.RFC3339),
				"endsAt":    now.Add(1 * time.Hour).Format(time.RFC3339),
				"createdBy": "e2e-test",
				"comment":   "E2E silence lifecycle test",
			},
		})
		if err != nil {
			t.Fatalf("post_silence: %v", err)
		}

		// Extract silence ID from response.
		text, ok := result.(string)
		if !ok {
			t.Fatalf("expected string, got %T", result)
		}
		var silenceResp struct {
			SilenceID string `json:"silenceID"`
		}
		if err := json.Unmarshal([]byte(text), &silenceResp); err != nil {
			t.Fatalf("parse silence response: %v (body: %s)", err, truncate(text, 200))
		}
		if silenceResp.SilenceID == "" {
			t.Fatalf("empty silence ID in response: %s", truncate(text, 200))
		}
		silenceID := silenceResp.SilenceID
		t.Logf("created silence: %s", silenceID)

		// Verify silence appears in list.
		result, err = client.InvokeTool(ctx, "alertmanager__get_silences", nil)
		if err != nil {
			t.Fatalf("get_silences: %v", err)
		}
		text, ok = result.(string)
		if !ok {
			t.Fatalf("expected string, got %T", result)
		}
		if !containsString(text, silenceID) {
			t.Errorf("silence %s not found in silences list", silenceID)
		}

		// Delete the silence.
		_, err = client.InvokeTool(ctx, "alertmanager__delete_silence", map[string]any{
			"silence_id": silenceID,
		})
		if err != nil {
			t.Fatalf("delete_silence: %v", err)
		}

		// Verify silence is expired/gone.
		result, err = client.InvokeTool(ctx, "alertmanager__get_silences", nil)
		if err != nil {
			t.Fatalf("get_silences after delete: %v", err)
		}
		text, ok = result.(string)
		if !ok {
			t.Fatalf("expected string, got %T", result)
		}
		// After deletion the silence state should be "expired" — verify
		// it's no longer in "active" state.
		if containsSilenceActive(text, silenceID) {
			t.Errorf("silence %s still active after deletion", silenceID)
		}
	})
}

// --- helpers ---

func toolNameSet(tools []mcp.Tool) map[string]bool {
	m := make(map[string]bool, len(tools))
	for _, t := range tools {
		m[t.Name] = true
	}
	return m
}

func jsonContainsKey(jsonStr, key string) bool {
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && json.Valid([]byte(s)) && // basic sanity
		findInJSON(s, substr)
}

func findInJSON(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsSilenceActive(jsonStr, silenceID string) bool {
	// Parse the paginated silences response and check if the given silence
	// is still in "active" state.
	var resp struct {
		Data []struct {
			ID     string `json:"id"`
			Status struct {
				State string `json:"state"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		// Try as plain array.
		var silences []struct {
			ID     string `json:"id"`
			Status struct {
				State string `json:"state"`
			} `json:"status"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &silences); err != nil {
			return false
		}
		resp.Data = silences
	}
	for _, s := range resp.Data {
		if s.ID == silenceID && s.Status.State == "active" {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
