package genesys

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// RegionURLs maps region names to their base URLs
var RegionURLs = map[string]string{
	"mypurecloud.com":       "https://api.mypurecloud.com",
	"mypurecloud.ie":        "https://api.mypurecloud.ie",
	"mypurecloud.de":        "https://api.mypurecloud.de",
	"mypurecloud.com.au":    "https://api.mypurecloud.com.au",
	"mypurecloud.jp":        "https://api.mypurecloud.jp",
	"usw2.pure.cloud":       "https://api.usw2.pure.cloud",
	"cac1.pure.cloud":       "https://api.cac1.pure.cloud",
	"euw2.pure.cloud":       "https://api.euw2.pure.cloud",
	"apne2.pure.cloud":      "https://api.apne2.pure.cloud",
	"aps1.pure.cloud":       "https://api.aps1.pure.cloud",
}

// LoginURLs maps region names to their login URLs
var LoginURLs = map[string]string{
	"mypurecloud.com":       "https://login.mypurecloud.com",
	"mypurecloud.ie":        "https://login.mypurecloud.ie",
	"mypurecloud.de":        "https://login.mypurecloud.de",
	"mypurecloud.com.au":    "https://login.mypurecloud.com.au",
	"mypurecloud.jp":        "https://login.mypurecloud.jp",
	"usw2.pure.cloud":       "https://login.usw2.pure.cloud",
	"cac1.pure.cloud":       "https://login.cac1.pure.cloud",
	"euw2.pure.cloud":       "https://login.euw2.pure.cloud",
	"apne2.pure.cloud":      "https://login.apne2.pure.cloud",
	"aps1.pure.cloud":       "https://login.aps1.pure.cloud",
}

// Client wraps the Genesys Cloud API client
type Client struct {
	baseURL      string
	loginURL     string
	clientID     string
	clientSecret string
	accessToken  string
	tokenExpiry  time.Time
	httpClient   *resty.Client
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewClient creates a new Genesys Cloud client
func NewClient(region, clientID, clientSecret string) (*Client, error) {
	baseURL, ok := RegionURLs[region]
	if !ok {
		return nil, fmt.Errorf("unknown region: %s", region)
	}

	loginURL, ok := LoginURLs[region]
	if !ok {
		return nil, fmt.Errorf("unknown region for login: %s", region)
	}

	httpClient := resty.New()
	httpClient.SetTimeout(60 * time.Second)
	httpClient.SetHeader("Content-Type", "application/json")

	client := &Client{
		baseURL:      baseURL,
		loginURL:     loginURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   httpClient,
	}

	// Authenticate immediately
	if err := client.authenticate(); err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	return client, nil
}

// authenticate obtains an access token using client credentials
func (c *Client) authenticate() error {
	var tokenResp TokenResponse

	resp, err := c.httpClient.R().
		SetBasicAuth(c.clientID, c.clientSecret).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(map[string]string{
			"grant_type": "client_credentials",
		}).
		SetResult(&tokenResp).
		Post(c.loginURL + "/oauth/token")

	if err != nil {
		return fmt.Errorf("token request failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("token request returned status %d: %s", resp.StatusCode(), resp.String())
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return nil
}

// ensureAuthenticated checks if token is valid and refreshes if needed
func (c *Client) ensureAuthenticated() error {
	if time.Now().After(c.tokenExpiry) {
		return c.authenticate()
	}
	return nil
}

// request makes an authenticated API request
func (c *Client) request() *resty.Request {
	return c.httpClient.R().
		SetHeader("Authorization", "Bearer "+c.accessToken).
		SetHeader("Content-Type", "application/json")
}

// Queue represents a Genesys Cloud queue
type Queue struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MemberCount int    `json:"memberCount,omitempty"`
}

// QueueSearchResult represents queue search results
type QueueSearchResult struct {
	Total      int     `json:"total"`
	PageNumber int     `json:"pageNumber"`
	PageSize   int     `json:"pageSize"`
	PageCount  int     `json:"pageCount"`
	Entities   []Queue `json:"entities"`
}

// SearchQueues searches for queues by name
func (c *Client) SearchQueues(name string, pageNumber, pageSize int) (map[string]interface{}, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"pageNumber": pageNumber,
		"pageSize":   pageSize,
	}

	if name != "" {
		body["query"] = []map[string]interface{}{
			{
				"type":   "CONTAINS",
				"fields": []string{"name"},
				"value":  name,
			},
		}
	}

	var result QueueSearchResult
	resp, err := c.request().
		SetBody(body).
		SetResult(&result).
		Post(c.baseURL + "/api/v2/routing/queues/query")

	if err != nil {
		return nil, fmt.Errorf("failed to search queues: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("queue search returned status %d: %s", resp.StatusCode(), resp.String())
	}

	return map[string]interface{}{
		"total":      result.Total,
		"pageNumber": result.PageNumber,
		"pageSize":   result.PageSize,
		"pageCount":  result.PageCount,
		"queues":     result.Entities,
	}, nil
}

// QueryQueueVolumes gets conversation volumes for queues
func (c *Client) QueryQueueVolumes(queueIDs []string, startTime, endTime time.Time) (map[string]interface{}, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	interval := fmt.Sprintf("%s/%s", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// Build predicates for each queue ID
	predicates := make([]map[string]interface{}, len(queueIDs))
	for i, queueID := range queueIDs {
		predicates[i] = map[string]interface{}{
			"dimension": "queueId",
			"value":     queueID,
		}
	}

	body := map[string]interface{}{
		"interval":    interval,
		"granularity": "PT30M",
		"groupBy":     []string{"queueId"},
		"metrics":     []string{"nOffered", "nAnswered", "nAbandoned"},
		"filter": map[string]interface{}{
			"type": "or",
			"predicates": predicates,
		},
	}

	var result map[string]interface{}
	resp, err := c.request().
		SetBody(body).
		SetResult(&result).
		Post(c.baseURL + "/api/v2/analytics/conversations/aggregates/query")

	if err != nil {
		return nil, fmt.Errorf("failed to query queue volumes: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("queue volumes query returned status %d: %s", resp.StatusCode(), resp.String())
	}

	return result, nil
}

// ConversationDetail represents a conversation
type ConversationDetail struct {
	ConversationID string    `json:"conversationId"`
	ConversationStart time.Time `json:"conversationStart"`
	ConversationEnd   time.Time `json:"conversationEnd,omitempty"`
}

// SampleConversationsByQueue samples conversations from a queue
func (c *Client) SampleConversationsByQueue(queueID string, startTime, endTime time.Time, sampleSize int) ([]string, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	interval := fmt.Sprintf("%s/%s", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	body := map[string]interface{}{
		"interval": interval,
		"segmentFilters": []map[string]interface{}{
			{
				"type": "and",
				"predicates": []map[string]interface{}{
					{
						"dimension": "queueId",
						"value":     queueID,
					},
				},
			},
		},
		"paging": map[string]interface{}{
			"pageSize":   sampleSize,
			"pageNumber": 1,
		},
	}

	var result struct {
		Conversations []struct {
			ConversationID string `json:"conversationId"`
		} `json:"conversations"`
	}

	resp, err := c.request().
		SetBody(body).
		SetResult(&result).
		Post(c.baseURL + "/api/v2/analytics/conversations/details/query")

	if err != nil {
		return nil, fmt.Errorf("failed to sample conversations: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("conversation sample returned status %d: %s", resp.StatusCode(), resp.String())
	}

	var conversationIDs []string
	for _, conv := range result.Conversations {
		conversationIDs = append(conversationIDs, conv.ConversationID)
	}

	return conversationIDs, nil
}

// OAuthClient represents an OAuth client
type OAuthClient struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ListOAuthClients lists all OAuth clients
func (c *Client) ListOAuthClients() ([]interface{}, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	var result struct {
		Entities []json.RawMessage `json:"entities"`
	}

	resp, err := c.request().
		SetResult(&result).
		Get(c.baseURL + "/api/v2/oauth/clients")

	if err != nil {
		return nil, fmt.Errorf("failed to list OAuth clients: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("OAuth clients list returned status %d: %s", resp.StatusCode(), resp.String())
	}

	var clients []interface{}
	for _, entity := range result.Entities {
		var client map[string]interface{}
		if err := json.Unmarshal(entity, &client); err == nil {
			clients = append(clients, map[string]interface{}{
				"id":          client["id"],
				"name":        client["name"],
				"description": client["description"],
			})
		}
	}

	return clients, nil
}

// SearchVoiceConversations searches for voice conversations
func (c *Client) SearchVoiceConversations(startTime, endTime time.Time, phoneNumber string, pageSize, pageNumber int) (map[string]interface{}, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	interval := fmt.Sprintf("%s/%s", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	body := map[string]interface{}{
		"interval": interval,
		"paging": map[string]interface{}{
			"pageSize":   pageSize,
			"pageNumber": pageNumber,
		},
	}

	// Add phone number filter if provided
	if phoneNumber != "" {
		body["segmentFilters"] = []map[string]interface{}{
			{
				"type": "and",
				"predicates": []map[string]interface{}{
					{
						"dimension": "addressFrom",
						"value":     phoneNumber,
					},
				},
			},
		}
	}

	var result map[string]interface{}
	resp, err := c.request().
		SetBody(body).
		SetResult(&result).
		Post(c.baseURL + "/api/v2/analytics/conversations/details/query")

	if err != nil {
		return nil, fmt.Errorf("failed to search conversations: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("conversation search returned status %d: %s", resp.StatusCode(), resp.String())
	}

	return result, nil
}

// GetConversationTranscript retrieves conversation transcript
func (c *Client) GetConversationTranscript(conversationID string) (map[string]interface{}, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	// This is a simplified implementation
	// In production, you'd need to handle transcription job creation and polling
	return map[string]interface{}{
		"conversationId": conversationID,
		"message":        "Transcript retrieval requires async job handling - not implemented in this version",
	}, nil
}
