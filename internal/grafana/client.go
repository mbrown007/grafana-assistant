package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// DashboardHit represents a dashboard item from /api/search.
type DashboardHit struct {
	ID        int64    `json:"id"`
	UID       string   `json:"uid"`
	Title     string   `json:"title"`
	URI       string   `json:"uri"`
	URL       string   `json:"url"`
	FolderID  int64    `json:"folderId"`
	FolderUID string   `json:"folderUid"`
	Folder    string   `json:"folderTitle"`
	Tags      []string `json:"tags"`
}

// Folder represents a Grafana folder.
type Folder struct {
	ID    int64  `json:"id"`
	UID   string `json:"uid"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// DashboardSaveResponse represents Grafana's response from /api/dashboards/db.
type DashboardSaveResponse struct {
	ID     int64  `json:"id"`
	UID    string `json:"uid"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
	URL    string `json:"url"`
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

// SearchDashboards finds dashboards matching the provided tags.
func (c *Client) SearchDashboards(ctx context.Context, tags []string) ([]DashboardHit, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/search", nil)
	if err != nil {
		return nil, err
	}
	c.setTokenAuth(req)

	q := req.URL.Query()
	q.Set("type", "dash-db")
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		q.Add("tag", tag)
	}
	req.URL.RawQuery = q.Encode()

	var hits []DashboardHit
	if err := c.do(req, &hits); err != nil {
		return nil, fmt.Errorf("search dashboards: %w", err)
	}
	return hits, nil
}

// GetFolderByTitle returns the first folder matching the title, or nil if none.
func (c *Client) GetFolderByTitle(ctx context.Context, title string) (*Folder, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/folders", nil)
	if err != nil {
		return nil, err
	}
	c.setTokenAuth(req)

	q := req.URL.Query()
	q.Set("limit", "1000")
	req.URL.RawQuery = q.Encode()

	var folders []Folder
	if err := c.do(req, &folders); err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	for _, f := range folders {
		if f.Title == title {
			return &f, nil
		}
	}
	return nil, nil
}

// CreateFolder creates a Grafana folder with the given title.
func (c *Client) CreateFolder(ctx context.Context, title string) (*Folder, error) {
	payload, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/folders", io.NopCloser(strings.NewReader(string(payload))))
	if err != nil {
		return nil, err
	}
	c.setTokenAuth(req)
	req.Header.Set("Content-Type", "application/json")

	var folder Folder
	if err := c.do(req, &folder); err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}
	return &folder, nil
}

// CreateDashboard creates a new dashboard from full JSON.
func (c *Client) CreateDashboard(ctx context.Context, dashboard map[string]any, folderUID string, overwrite bool) (*DashboardSaveResponse, error) {
	return c.saveDashboard(ctx, "", dashboard, folderUID, overwrite)
}

// UpdateDashboard updates an existing dashboard by UID using full JSON.
func (c *Client) UpdateDashboard(ctx context.Context, uid string, dashboard map[string]any, folderUID string, overwrite bool) (*DashboardSaveResponse, error) {
	return c.saveDashboard(ctx, uid, dashboard, folderUID, overwrite)
}

// DeleteDashboard deletes a dashboard by UID.
func (c *Client) DeleteDashboard(ctx context.Context, uid string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/dashboards/uid/"+uid, nil)
	if err != nil {
		return err
	}
	c.setTokenAuth(req)
	if err := c.doNoBody(req); err != nil {
		return fmt.Errorf("delete dashboard %s: %w", uid, err)
	}
	return nil
}

func (c *Client) saveDashboard(ctx context.Context, uid string, dashboard map[string]any, folderUID string, overwrite bool) (*DashboardSaveResponse, error) {
	if uid != "" {
		dashboard["uid"] = uid
	}
	payload := map[string]any{
		"dashboard": dashboard,
		"overwrite": overwrite,
	}
	if folderUID != "" {
		payload["folderUid"] = folderUID
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/dashboards/db", io.NopCloser(strings.NewReader(string(data))))
	if err != nil {
		return nil, err
	}
	c.setTokenAuth(req)
	req.Header.Set("Content-Type", "application/json")

	var resp DashboardSaveResponse
	if err := c.do(req, &resp); err != nil {
		return nil, fmt.Errorf("save dashboard: %w", err)
	}
	return &resp, nil
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

func (c *Client) doNoBody(req *http.Request) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("grafana API %s %s: %d %s", req.Method, req.URL.Path, resp.StatusCode, body)
	}
	return nil
}
