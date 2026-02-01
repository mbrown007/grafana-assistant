//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

const (
	grafanaURL          = "http://localhost:13000"
	alertmanagerURL     = "http://localhost:19093"
	mcpAlertmanagerPort = 18000
	mcpGrafanaPort      = 18001
)

// requireService skips the test if the backend service is not reachable.
func requireService(t *testing.T, baseURL, healthPath string) {
	t.Helper()
	url := baseURL + healthPath
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Skipf("service not reachable at %s: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("service not healthy at %s: status %d", url, resp.StatusCode)
	}
}

// projectRoot returns the absolute path to the project root (two levels up from tests/e2e).
func projectRoot(t *testing.T) string {
	t.Helper()
	// This file lives in tests/e2e/, so go up two levels.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..")
}

// buildMCPServer compiles an MCP server binary and returns the path.
// The binary is removed on test cleanup.
func buildMCPServer(t *testing.T, name, sourceDir, outputBin string) string {
	t.Helper()
	root := projectRoot(t)
	srcDir := filepath.Join(root, sourceDir)
	binPath := filepath.Join(root, "bin", outputBin)

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	t.Cleanup(func() { os.Remove(binPath) })
	return binPath
}

// startMCPServer starts an MCP server subprocess in SSE mode and waits for
// the /sse endpoint to be reachable. It kills the process on test cleanup.
// Extra args are passed as command-line arguments to the binary.
func startMCPServer(t *testing.T, bin string, env []string, port int, args ...string) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start MCP server %s: %v", bin, err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// Wait for the SSE endpoint to become reachable.
	sseURL := fmt.Sprintf("http://localhost:%d/sse", port)
	waitForEndpoint(t, sseURL, 15*time.Second)
}

// waitForEndpoint polls a URL until it gets a non-error response or times out.
func waitForEndpoint(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("endpoint %s not reachable within %s", url, timeout)
}

// connectMCPClient creates an SSE MCP client, connects it, and returns it.
// The SSE connection context stays alive until test cleanup so tool invocations
// can receive responses on the SSE stream.
func connectMCPClient(t *testing.T, baseURL, serverType string) *mcp.SSEClient {
	t.Helper()
	client := mcp.NewSSEClient(baseURL, serverType)
	// The SSE stream must remain open for tool calls, so use a context that
	// lives until the test finishes.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := client.Connect(ctx); err != nil {
		cancel()
		t.Fatalf("connect MCP client (%s): %v", serverType, err)
	}
	return client
}
