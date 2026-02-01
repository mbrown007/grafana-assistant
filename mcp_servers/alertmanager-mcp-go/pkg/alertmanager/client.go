package alertmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// Client represents an Alertmanager HTTP client
type Client struct {
	baseURL  string
	username string
	password string
	tenantID string
	client   *resty.Client
}

// NewClient creates a new Alertmanager client
func NewClient(baseURL, username, password, tenantID string) *Client {
	client := resty.New()
	client.SetTimeout(60 * time.Second)
	client.SetHeader("Content-Type", "application/json")

	// Set basic auth if credentials provided
	if username != "" && password != "" {
		client.SetBasicAuth(username, password)
	}

	return &Client{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
		tenantID: tenantID,
		client:   client,
	}
}

// urlJoin joins base URL with path, preserving base path
func (c *Client) urlJoin(path string) string {
	return c.baseURL + "/" + strings.TrimPrefix(path, "/")
}

// prepareRequest creates a request with tenant header if needed
func (c *Client) prepareRequest(tenantID string) *resty.Request {
	req := c.client.R()

	// Use request-specific tenant ID or fall back to client default
	tid := tenantID
	if tid == "" {
		tid = c.tenantID
	}

	if tid != "" {
		req.SetHeader("X-Scope-OrgId", tid)
	}

	return req
}

// Status represents Alertmanager status
type Status struct {
	Cluster struct {
		Name   string   `json:"name"`
		Status string   `json:"status"`
		Peers  []string `json:"peers"`
	} `json:"cluster"`
	VersionInfo struct {
		Version   string `json:"version"`
		Revision  string `json:"revision"`
		Branch    string `json:"branch"`
		BuildUser string `json:"buildUser"`
		BuildDate string `json:"buildDate"`
		GoVersion string `json:"goVersion"`
	} `json:"versionInfo"`
	Config struct {
		Original string `json:"original"`
	} `json:"config"`
	Uptime string `json:"uptime"`
}

// GetStatus retrieves the Alertmanager status
func (c *Client) GetStatus(tenantID string) (*Status, error) {
	var status Status
	resp, err := c.prepareRequest(tenantID).
		SetResult(&status).
		Get(c.urlJoin("/api/v2/status"))

	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return &status, nil
}

// Alert represents an Alertmanager alert
type Alert struct {
	Annotations  map[string]string   `json:"annotations,omitempty"`
	EndsAt       time.Time           `json:"endsAt,omitempty"`
	Fingerprint  string              `json:"fingerprint,omitempty"`
	Receivers    []map[string]string `json:"receivers,omitempty"`
	StartsAt     time.Time           `json:"startsAt,omitempty"`
	Status       map[string]any      `json:"status,omitempty"`
	UpdatedAt    time.Time           `json:"updatedAt,omitempty"`
	GeneratorURL string              `json:"generatorURL,omitempty"`
	Labels       map[string]string   `json:"labels,omitempty"`
}

// AlertsFilter represents filters for listing alerts
type AlertsFilter struct {
	Filter    string
	Silenced  *bool
	Inhibited *bool
	Active    *bool
}

// ListAlerts retrieves alerts with optional filters
func (c *Client) ListAlerts(filter AlertsFilter, tenantID string) ([]Alert, error) {
	var alerts []Alert

	req := c.prepareRequest(tenantID).SetResult(&alerts)

	// Build query parameters
	if filter.Filter != "" {
		req.SetQueryParam("filter", filter.Filter)
	}
	if filter.Silenced != nil {
		req.SetQueryParam("silenced", fmt.Sprintf("%t", *filter.Silenced))
	}
	if filter.Inhibited != nil {
		req.SetQueryParam("inhibited", fmt.Sprintf("%t", *filter.Inhibited))
	}
	if filter.Active != nil {
		req.SetQueryParam("active", fmt.Sprintf("%t", *filter.Active))
	} else {
		// Default to active=true if not specified
		req.SetQueryParam("active", "true")
	}

	resp, err := req.Get(c.urlJoin("/api/v2/alerts"))
	if err != nil {
		return nil, fmt.Errorf("failed to list alerts: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return alerts, nil
}

// AlertGroup represents a group of alerts
type AlertGroup struct {
	Labels   map[string]string `json:"labels"`
	Receiver struct {
		Name string `json:"name"`
	} `json:"receiver"`
	Alerts []Alert `json:"alerts"`
}

// AlertGroupsFilter represents filters for listing alert groups
type AlertGroupsFilter struct {
	Silenced  *bool
	Inhibited *bool
	Active    *bool
}

// GetAlertGroups retrieves alert groups with optional filters
func (c *Client) GetAlertGroups(filter AlertGroupsFilter, tenantID string) ([]AlertGroup, error) {
	var groups []AlertGroup

	req := c.prepareRequest(tenantID).SetResult(&groups)

	// Build query parameters
	if filter.Silenced != nil {
		req.SetQueryParam("silenced", fmt.Sprintf("%t", *filter.Silenced))
	}
	if filter.Inhibited != nil {
		req.SetQueryParam("inhibited", fmt.Sprintf("%t", *filter.Inhibited))
	}
	if filter.Active != nil {
		req.SetQueryParam("active", fmt.Sprintf("%t", *filter.Active))
	} else {
		// Default to active=true if not specified
		req.SetQueryParam("active", "true")
	}

	resp, err := req.Get(c.urlJoin("/api/v2/alerts/groups"))
	if err != nil {
		return nil, fmt.Errorf("failed to get alert groups: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return groups, nil
}

// Silence represents an Alertmanager silence
type Silence struct {
	ID        string    `json:"id,omitempty"`
	Status    any       `json:"status,omitempty"`
	Matchers  []Matcher `json:"matchers"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	CreatedBy string    `json:"createdBy"`
	Comment   string    `json:"comment"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// Matcher represents a label matcher for silences
type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex,omitempty"`
	IsEqual *bool  `json:"isEqual,omitempty"`
}

// ListSilences retrieves all silences with optional filter
func (c *Client) ListSilences(filter string, tenantID string) ([]Silence, error) {
	var silences []Silence

	req := c.prepareRequest(tenantID).SetResult(&silences)

	if filter != "" {
		req.SetQueryParam("filter", filter)
	}

	resp, err := req.Get(c.urlJoin("/api/v2/silences"))
	if err != nil {
		return nil, fmt.Errorf("failed to list silences: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return silences, nil
}

// SilenceResponse represents the response from creating/updating a silence
type SilenceResponse struct {
	SilenceID string `json:"silenceID"`
}

// CreateSilence creates a new silence or updates an existing one
func (c *Client) CreateSilence(silence Silence, tenantID string) (*SilenceResponse, error) {
	var response SilenceResponse

	resp, err := c.prepareRequest(tenantID).
		SetBody(silence).
		SetResult(&response).
		Post(c.urlJoin("/api/v2/silences"))

	if err != nil {
		return nil, fmt.Errorf("failed to create silence: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return &response, nil
}

// DeleteSilence deletes a silence by ID
func (c *Client) DeleteSilence(silenceID string, tenantID string) error {
	resp, err := c.prepareRequest(tenantID).
		Delete(c.urlJoin("/api/v2/silence/" + silenceID))

	if err != nil {
		return fmt.Errorf("failed to delete silence: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// CreateAlert posts one or more alerts to Alertmanager
func (c *Client) CreateAlert(alerts []Alert, tenantID string) error {
	resp, err := c.prepareRequest(tenantID).
		SetBody(alerts).
		Post(c.urlJoin("/api/v2/alerts"))

	if err != nil {
		return fmt.Errorf("failed to create alert: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// GetReceivers retrieves all receivers
func (c *Client) GetReceivers(tenantID string) ([]map[string]string, error) {
	var receivers []map[string]string

	resp, err := c.prepareRequest(tenantID).
		SetResult(&receivers).
		Get(c.urlJoin("/api/v2/receivers"))

	if err != nil {
		return nil, fmt.Errorf("failed to get receivers: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return receivers, nil
}

// MarshalJSON custom marshaler to handle empty slices
func (a *Alert) MarshalJSON() ([]byte, error) {
	type Alias Alert
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(a),
	})
}
