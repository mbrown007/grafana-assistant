//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const appPort = 18080

func TestFullStack(t *testing.T) {
	requireService(t, grafanaURL, "/grafana/api/health")
	requireService(t, alertmanagerURL, "/-/healthy")

	root := projectRoot(t)

	// Build MCP servers.
	amBin := buildMCPServer(t, "alertmanager-mcp",
		"mcp_servers/alertmanager-mcp-go/cmd/server", "mcp-alertmanager-e2e-fs")
	grBin := buildMCPServer(t, "grafana-mcp",
		"mcp_servers/mcp-grafana/cmd/mcp-grafana", "mcp-grafana-e2e-fs")

	// Start MCP servers on dedicated ports.
	amPort := 18010
	grPort := 18011
	startMCPServer(t, amBin, []string{
		"MCP_TRANSPORT=sse",
		fmt.Sprintf("MCP_PORT=%d", amPort),
		fmt.Sprintf("ALERTMANAGER_URL=%s", alertmanagerURL),
	}, amPort)
	startMCPServer(t, grBin, []string{
		fmt.Sprintf("GRAFANA_URL=%s/grafana", grafanaURL),
	}, grPort,
		"-transport", "sse",
		"-address", fmt.Sprintf("localhost:%d", grPort),
	)

	// Build the assistant binary.
	appBin := filepath.Join(root, "bin", "assistant-e2e")
	buildCmd := exec.Command("go", "build", "-o", appBin, "./cmd/assistant")
	buildCmd.Dir = root
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("build assistant: %v", err)
	}
	t.Cleanup(func() { os.Remove(appBin) })

	// Write a temporary config.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := map[string]any{
		"listen_addr":    fmt.Sprintf(":%d", appPort),
		"grafana_url":    grafanaURL,
		"db_path":        dbPath,
		"kb_path":        filepath.Join(root, "KB"),
		"audit_log_path": filepath.Join(tmpDir, "audit.log"),
		"mcp_servers": []map[string]any{
			{
				"url":       fmt.Sprintf("http://localhost:%d", amPort),
				"type":      "alertmanager",
				"transport": "sse",
			},
			{
				"url":       fmt.Sprintf("http://localhost:%d", grPort),
				"type":      "grafana",
				"transport": "sse",
			},
		},
	}
	cfgBytes, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, cfgBytes, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Start the assistant app.
	appCmd := exec.Command(appBin, "-config", configPath)
	appCmd.Dir = root
	appCmd.Stdout = os.Stdout
	appCmd.Stderr = os.Stderr
	if err := appCmd.Start(); err != nil {
		t.Fatalf("start assistant: %v", err)
	}
	t.Cleanup(func() {
		_ = appCmd.Process.Kill()
		_ = appCmd.Wait()
	})

	appBase := fmt.Sprintf("http://localhost:%d", appPort)
	waitForEndpoint(t, appBase+"/healthz", 20*time.Second)

	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(appBase + "/healthz")
		if err != nil {
			t.Fatalf("GET /healthz: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["status"] != "ok" {
			t.Errorf("expected status=ok, got %q", body["status"])
		}
	})

	t.Run("DashboardContext", func(t *testing.T) {
		// Create a test dashboard via the Grafana API.
		uid := createTestDashboard(t)
		if uid == "" {
			t.Skip("could not create test dashboard")
		}

		resp, err := http.Get(appBase + "/api/dashboard-context/" + uid)
		if err != nil {
			t.Fatalf("GET /api/dashboard-context/%s: %v", uid, err)
		}
		defer resp.Body.Close()
		// The enricher returns JSON with dashboard info. Even if the enrichment
		// partially fails, we should get a response (possibly with error details).
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadGateway {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 or 502, got %d: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("GrafanaProxy", func(t *testing.T) {
		resp, err := http.Get(appBase + "/grafana/api/health")
		if err != nil {
			t.Fatalf("GET /grafana/api/health: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
		}
	})
}

// createTestDashboard creates a simple dashboard via the Grafana API and returns its UID.
func createTestDashboard(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"dashboard": map[string]any{
			"title": fmt.Sprintf("E2E Test Dashboard %d", time.Now().UnixNano()),
			"panels": []any{
				map[string]any{
					"type":  "text",
					"title": "Test Panel",
					"gridPos": map[string]any{
						"h": 8, "w": 12, "x": 0, "y": 0,
					},
				},
			},
		},
		"overwrite": false,
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(grafanaURL+"/grafana/api/dashboards/db",
		"application/json", bytes.NewReader(body))
	if err != nil {
		t.Logf("create dashboard: %v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Logf("create dashboard: status %d: %s", resp.StatusCode, string(respBody))
		return ""
	}
	var result struct {
		UID string `json:"uid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Logf("parse dashboard response: %v", err)
		return ""
	}
	t.Logf("created test dashboard: %s", result.UID)
	return result.UID
}
