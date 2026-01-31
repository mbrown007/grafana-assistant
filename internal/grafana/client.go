package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// User is the subset of Grafana's user object we care about.
type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
	OrgID int64  `json:"orgId"`
}

// Dashboard is the wrapper Grafana returns from GET /api/dashboards/uid/:uid.
type Dashboard struct {
	Meta struct {
		Slug    string `json:"slug"`
		Folder  string `json:"folderTitle"`
		FolderUID string `json:"folderUid"`
	} `json:"meta"`
	Dashboard json.RawMessage `json:"dashboard"`
}

// Client talks to the Grafana HTTP API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a Grafana API client.
// token is a service-account token used for API calls that aren't session-based.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetCurrentUser returns the user associated with the service-account token.
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/user", nil)
	if err != nil {
		return nil, err
	}
	c.setTokenAuth(req)

	var u User
	if err := c.do(req, &u); err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	return &u, nil
}

// ResolveUserFromSession calls Grafana's /api/user using the provided session
// cookies, returning the user that owns the session.
func (c *Client) ResolveUserFromSession(ctx context.Context, cookies []*http.Cookie) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/user", nil)
	if err != nil {
		return nil, err
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	var u User
	if err := c.do(req, &u); err != nil {
		return nil, fmt.Errorf("resolve user from session: %w", err)
	}
	return &u, nil
}

// GetDashboard fetches a dashboard by UID.
func (c *Client) GetDashboard(ctx context.Context, uid string) (*Dashboard, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/dashboards/uid/"+uid, nil)
	if err != nil {
		return nil, err
	}
	c.setTokenAuth(req)

	var d Dashboard
	if err := c.do(req, &d); err != nil {
		return nil, fmt.Errorf("get dashboard %s: %w", uid, err)
	}
	return &d, nil
}

func (c *Client) setTokenAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func (c *Client) do(req *http.Request, dst interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("grafana API %s %s: %d %s", req.Method, req.URL.Path, resp.StatusCode, body)
	}

	return json.NewDecoder(resp.Body).Decode(dst)
}
