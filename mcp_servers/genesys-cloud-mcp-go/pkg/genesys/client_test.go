package genesys

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

func newTestClient(serverURL string) *Client {
	return &Client{
		baseURL:      serverURL,
		loginURL:     serverURL,
		clientID:     "client-id",
		clientSecret: "client-secret",
		httpClient:   resty.New(),
	}
}

func withTestRegion(t *testing.T, url string, fn func()) {
	t.Helper()

	prevRegion, hadRegion := RegionURLs["test"]
	prevLogin, hadLogin := LoginURLs["test"]

	RegionURLs["test"] = url
	LoginURLs["test"] = url

	defer func() {
		if hadRegion {
			RegionURLs["test"] = prevRegion
		} else {
			delete(RegionURLs, "test")
		}
		if hadLogin {
			LoginURLs["test"] = prevLogin
		} else {
			delete(LoginURLs, "test")
		}
	}()

	fn()
}

func TestNewClientUnknownRegion(t *testing.T) {
	if _, err := NewClient("unknown-region", "id", "secret"); err == nil {
		t.Fatal("expected error for unknown region")
	}
}

func TestAuthenticateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-123","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(server.URL)
	if err := client.authenticate(); err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if client.accessToken != "token-123" {
		t.Fatalf("expected access token set, got %q", client.accessToken)
	}
	if time.Now().After(client.tokenExpiry) {
		t.Fatal("expected token expiry in the future")
	}
}

func TestNewClientSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-abc","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(server.Close)

	withTestRegion(t, server.URL, func() {
		client, err := NewClient("test", "id", "secret")
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		if client.accessToken != "token-abc" {
			t.Fatalf("expected token set on NewClient, got %q", client.accessToken)
		}
	})
}

func TestEnsureAuthenticatedRefreshesToken(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-refresh","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(server.URL)
	client.tokenExpiry = time.Now().Add(-1 * time.Minute)
	if err := client.ensureAuthenticated(); err != nil {
		t.Fatalf("ensureAuthenticated failed: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected authenticate call, got %d", hits)
	}
}

func TestSearchQueues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if _, ok := payload["query"]; !ok {
			t.Fatal("expected query in payload")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(QueueSearchResult{
			Total:      1,
			PageNumber: 2,
			PageSize:   5,
			PageCount:  1,
			Entities: []Queue{{
				ID:   "q1",
				Name: "Sales",
			}},
		})
	}))
	t.Cleanup(server.Close)

	client := newTestClient(server.URL)
	client.accessToken = "token"
	client.tokenExpiry = time.Now().Add(time.Hour)

	result, err := client.SearchQueues("Sales", 2, 5)
	if err != nil {
		t.Fatalf("SearchQueues failed: %v", err)
	}
	switch total := result["total"].(type) {
	case int:
		if total != 1 {
			t.Fatalf("expected total 1, got %#v", total)
		}
	case float64:
		if total != 1 {
			t.Fatalf("expected total 1, got %#v", total)
		}
	default:
		t.Fatalf("unexpected total type: %T", total)
	}
}

func TestQueryQueueVolumes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/analytics/conversations/aggregates/query" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(server.URL)
	client.accessToken = "token"
	client.tokenExpiry = time.Now().Add(time.Hour)

	result, err := client.QueryQueueVolumes([]string{"q1"}, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("QueryQueueVolumes failed: %v", err)
	}
	if result["ok"].(bool) != true {
		t.Fatalf("expected ok true, got %#v", result["ok"])
	}
}

func TestSampleConversationsByQueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/analytics/conversations/details/query" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"conversations":[{"conversationId":"c1"},{"conversationId":"c2"}]}`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(server.URL)
	client.accessToken = "token"
	client.tokenExpiry = time.Now().Add(time.Hour)

	result, err := client.SampleConversationsByQueue("q1", time.Now().Add(-time.Hour), time.Now(), 2)
	if err != nil {
		t.Fatalf("SampleConversationsByQueue failed: %v", err)
	}
	if len(result) != 2 || result[0] != "c1" || result[1] != "c2" {
		t.Fatalf("unexpected conversation IDs: %#v", result)
	}
}

func TestListOAuthClients(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/oauth/clients" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"entities":[{"id":"c1","name":"Client 1","description":"desc"}]}`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(server.URL)
	client.accessToken = "token"
	client.tokenExpiry = time.Now().Add(time.Hour)

	result, err := client.ListOAuthClients()
	if err != nil {
		t.Fatalf("ListOAuthClients failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 client, got %d", len(result))
	}
	clientMap := result[0].(map[string]interface{})
	if clientMap["id"] != "c1" || clientMap["name"] != "Client 1" {
		t.Fatalf("unexpected client: %#v", clientMap)
	}
}

func TestSearchVoiceConversationsWithPhone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	t.Cleanup(server.Close)

	client := newTestClient(server.URL)
	client.accessToken = "token"
	client.tokenExpiry = time.Now().Add(time.Hour)

	result, err := client.SearchVoiceConversations(time.Now().Add(-time.Hour), time.Now(), "+15551234567", 10, 1)
	if err != nil {
		t.Fatalf("SearchVoiceConversations failed: %v", err)
	}
	if result["ok"].(bool) != true {
		t.Fatalf("expected ok true, got %#v", result["ok"])
	}
}

func TestSearchVoiceConversationsWithoutPhone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/analytics/conversations/details/query" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, ok := payload["segmentFilters"]; ok {
			t.Fatal("did not expect segmentFilters when phone is empty")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	client := newTestClient(server.URL)
	client.accessToken = "token"
	client.tokenExpiry = time.Now().Add(time.Hour)

	_, err := client.SearchVoiceConversations(time.Now().Add(-time.Hour), time.Now(), "", 10, 1)
	if err != nil {
		t.Fatalf("SearchVoiceConversations failed: %v", err)
	}
}
