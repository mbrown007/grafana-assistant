//go:build integration

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/agent"
	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/grafana"
	"github.com/marcusz/monitoring-assistant/internal/llm"
	"github.com/marcusz/monitoring-assistant/internal/storage"
)

// mockLLMServer creates a mock OpenAI-compatible API server that returns a
// fixed response.
func mockLLMServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "This is a test response from the mock LLM.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 15,
				"total_tokens":      25,
			},
		})
	}))
}

func TestChat_FullFlowWithMockLLM(t *testing.T) {
	// Set up mock LLM server.
	llmServer := mockLLMServer(t)
	t.Cleanup(llmServer.Close)

	// Create LLM client pointing to mock server.
	llmClient, err := llm.NewClientWithBaseURL("test-key", "gpt-4o", llmServer.URL+"/v1")
	if err != nil {
		t.Fatalf("failed to create LLM client: %v", err)
	}

	// Set up in-memory storage.
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// Create agent manager (no MCP clients for this test).
	agentMgr := agent.NewManager(llmClient, nil, nil, store)

	// Build the mux with the chat endpoint.
	mux := http.NewServeMux()

	// Use a mock session resolver that always returns a test user.
	resolver := &mockResolver{
		user: &grafana.User{
			ID:    1,
			Login: "testuser",
			Name:  "Test User",
			OrgID: 1,
		},
	}

	mux.HandleFunc("POST /api/chat", api.AuthenticatedChatHandlerWithLimit(
		resolver,
		func(r *http.Request, user *grafana.User, req api.ChatRequest, streamFn func(api.StreamChunk)) {
			agentMgr.HandleChat(r.Context(), user, req, streamFn)
		},
		16000,
	))

	cfg := testConfig("http://localhost:3000")
	ts := setupTestServer(t, mux, cfg)

	// Send a chat request.
	payload := `{"message":"Hello, test"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/chat", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify X-Request-ID header is present.
	if rid := resp.Header.Get("X-Request-ID"); rid == "" {
		t.Error("expected X-Request-ID header")
	}

	// Parse SSE chunks.
	scanner := bufio.NewScanner(resp.Body)
	var chunks []api.StreamChunk
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var chunk api.StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			t.Logf("failed to parse chunk: %v", err)
			continue
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one SSE chunk")
	}

	// Verify we got a start chunk with a session ID.
	hasStart := false
	hasContent := false
	sessionID := ""
	for _, c := range chunks {
		if c.Type == "start" {
			hasStart = true
			sessionID = c.SessionID
		}
		if c.Type == "token" || c.Type == "complete" {
			if c.Message != "" {
				hasContent = true
			}
		}
	}

	if !hasStart {
		t.Error("expected a 'start' chunk")
	}
	if sessionID == "" {
		t.Error("expected session ID in start chunk")
	}
	if !hasContent {
		t.Error("expected content in response chunks")
	}

	// Verify session was persisted.
	sess, err := store.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session to be persisted")
	}
}

// mockResolver implements api.UserResolver for testing.
type mockResolver struct {
	user *grafana.User
}

func (m *mockResolver) Resolve(_ context.Context, _ *http.Request) (*grafana.User, error) {
	return m.user, nil
}
