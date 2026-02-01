package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sabio/genesys-cloud-mcp-go/pkg/genesys"
)

func withTestRegion(t *testing.T, url string, fn func()) {
	t.Helper()

	prevRegion, hadRegion := genesys.RegionURLs["test"]
	prevLogin, hadLogin := genesys.LoginURLs["test"]

	genesys.RegionURLs["test"] = url
	genesys.LoginURLs["test"] = url

	defer func() {
		if hadRegion {
			genesys.RegionURLs["test"] = prevRegion
		} else {
			delete(genesys.RegionURLs, "test")
		}
		if hadLogin {
			genesys.LoginURLs["test"] = prevLogin
		} else {
			delete(genesys.LoginURLs, "test")
		}
	}()

	fn()
}

func newAuthServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token-123","token_type":"bearer","expires_in":3600}`))
			return
		}
		handler(w, r)
	}))
}

func toolResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("empty tool result")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return text.Text
}

func TestHandleSearchQueuesSuccess(t *testing.T) {
	server := newAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/routing/queues/query" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["pageNumber"].(float64) != 2 || payload["pageSize"].(float64) != 5 {
			t.Fatalf("unexpected paging: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(genesys.QueueSearchResult{
			Total:      1,
			PageNumber: 2,
			PageSize:   5,
			PageCount:  1,
			Entities: []genesys.Queue{{
				ID:   "q1",
				Name: "Sales",
			}},
		})
	})
	t.Cleanup(server.Close)

	withTestRegion(t, server.URL, func() {
		client, err := genesys.NewClient("test", "id", "secret")
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		mcpServer := NewMCPServer(client)

		result, err := mcpServer.handleSearchQueues(map[string]interface{}{
			"name":       "Sales",
			"pageNumber": 2,
			"pageSize":   5,
		})
		if err != nil {
			t.Fatalf("handleSearchQueues error: %v", err)
		}
		if result.IsError {
			t.Fatal("expected success result")
		}
		text := toolResultText(t, result)
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if payload["total"].(float64) != 1 {
			t.Fatalf("unexpected total: %#v", payload["total"])
		}
	})
}

func TestHandleQueryQueueVolumesBadQueueIDs(t *testing.T) {
	server := newAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	t.Cleanup(server.Close)

	withTestRegion(t, server.URL, func() {
		client, err := genesys.NewClient("test", "id", "secret")
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		mcpServer := NewMCPServer(client)

		result, err := mcpServer.handleQueryQueueVolumes(map[string]interface{}{
			"queueIds":  "not-an-array",
			"startTime": time.Now().Format(time.RFC3339),
			"endTime":   time.Now().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("handleQueryQueueVolumes error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error result")
		}
	})
}

func TestHandleSampleConversationsInvalidTime(t *testing.T) {
	server := newAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	t.Cleanup(server.Close)

	withTestRegion(t, server.URL, func() {
		client, err := genesys.NewClient("test", "id", "secret")
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		mcpServer := NewMCPServer(client)

		result, err := mcpServer.handleSampleConversations(map[string]interface{}{
			"queueId":   "q1",
			"startTime": "not-a-time",
			"endTime":   time.Now().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("handleSampleConversations error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error result")
		}
	})
}

func TestHandleOAuthClientsSuccess(t *testing.T) {
	server := newAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/oauth/clients" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"entities":[{"id":"c1","name":"Client 1","description":"desc"}]}`))
	})
	t.Cleanup(server.Close)

	withTestRegion(t, server.URL, func() {
		client, err := genesys.NewClient("test", "id", "secret")
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		mcpServer := NewMCPServer(client)

		result, err := mcpServer.handleOAuthClients(map[string]interface{}{})
		if err != nil {
			t.Fatalf("handleOAuthClients error: %v", err)
		}
		if result.IsError {
			t.Fatal("expected success result")
		}
		text := toolResultText(t, result)
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if payload["count"].(float64) != 1 {
			t.Fatalf("unexpected count: %#v", payload["count"])
		}
	})
}

func TestHandleSearchVoiceConversationsWithPhone(t *testing.T) {
	server := newAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/analytics/conversations/details/query" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, ok := payload["segmentFilters"]; !ok {
			t.Fatal("expected segmentFilters in payload")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	t.Cleanup(server.Close)

	withTestRegion(t, server.URL, func() {
		client, err := genesys.NewClient("test", "id", "secret")
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		mcpServer := NewMCPServer(client)

		result, err := mcpServer.handleSearchVoiceConversations(map[string]interface{}{
			"startTime":   time.Now().Add(-time.Hour).Format(time.RFC3339),
			"endTime":     time.Now().Format(time.RFC3339),
			"phoneNumber": "+15551234567",
			"pageSize":    10,
			"pageNumber":  1,
		})
		if err != nil {
			t.Fatalf("handleSearchVoiceConversations error: %v", err)
		}
		if result.IsError {
			t.Fatal("expected success result")
		}
	})
}

func TestHandleSearchVoiceConversationsInvalidTime(t *testing.T) {
	server := newAuthServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	t.Cleanup(server.Close)

	withTestRegion(t, server.URL, func() {
		client, err := genesys.NewClient("test", "id", "secret")
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		mcpServer := NewMCPServer(client)

		result, err := mcpServer.handleSearchVoiceConversations(map[string]interface{}{
			"startTime": "not-a-time",
			"endTime":   time.Now().Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("handleSearchVoiceConversations error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error result")
		}
	})
}
