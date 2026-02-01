package alertmanager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		username string
		password string
		tenant   string
	}{
		{
			name:     "basic client",
			url:      "http://localhost:9093",
			username: "",
			password: "",
			tenant:   "",
		},
		{
			name:     "with auth",
			url:      "http://localhost:9093",
			username: "admin",
			password: "secret",
			tenant:   "",
		},
		{
			name:     "with tenant",
			url:      "http://localhost:9093",
			username: "",
			password: "",
			tenant:   "tenant-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.url, tt.username, tt.password, tt.tenant)
			if client == nil {
				t.Fatal("NewClient() returned nil")
			}
			if client.baseURL != tt.url {
				t.Errorf("Client baseURL = %v, want %v", client.baseURL, tt.url)
			}
			if client.username != tt.username {
				t.Errorf("Client username = %v, want %v", client.username, tt.username)
			}
			if client.tenantID != tt.tenant {
				t.Errorf("Client tenantID = %v, want %v", client.tenantID, tt.tenant)
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	mockStatus := Status{
		Uptime: "24h",
	}
	mockStatus.Cluster.Name = "cluster-1"
	mockStatus.Cluster.Status = "ready"
	mockStatus.Cluster.Peers = []string{"peer1", "peer2"}
	mockStatus.VersionInfo.Version = "0.25.0"
	mockStatus.VersionInfo.Revision = "abc123"
	mockStatus.VersionInfo.Branch = "main"
	mockStatus.VersionInfo.BuildDate = "2024-01-01"
	mockStatus.VersionInfo.GoVersion = "go1.21"
	mockStatus.Config.Original = "config content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/status" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockStatus)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	status, err := client.GetStatus("")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Cluster.Status != "ready" {
		t.Errorf("Status.Cluster.Status = %v, want ready", status.Cluster.Status)
	}

	if status.VersionInfo.Version != "0.25.0" {
		t.Errorf("Status.VersionInfo.Version = %v, want 0.25.0", status.VersionInfo.Version)
	}
}

func TestListAlerts(t *testing.T) {
	mockAlerts := []Alert{
		{
			Labels: map[string]string{
				"alertname": "HighCPU",
				"severity":  "critical",
				"instance":  "server-1",
			},
			Annotations: map[string]string{
				"summary":     "CPU usage is high",
				"description": "CPU usage above 90%",
			},
			StartsAt: time.Now().Add(-1 * time.Hour),
			EndsAt:   time.Now().Add(1 * time.Hour),
			Status: map[string]any{
				"state": "active",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/alerts" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockAlerts)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	alerts, err := client.ListAlerts(AlertsFilter{}, "")
	if err != nil {
		t.Fatalf("ListAlerts() error = %v", err)
	}

	if len(alerts) != 1 {
		t.Errorf("ListAlerts() returned %d alerts, want 1", len(alerts))
	}

	if alerts[0].Labels["alertname"] != "HighCPU" {
		t.Errorf("Alert alertname = %v, want HighCPU", alerts[0].Labels["alertname"])
	}
}

func TestListAlertsWithFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check query parameters
		query := r.URL.Query()
		if query.Get("filter") == "" {
			t.Error("Expected filter query parameter")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Alert{})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	filter := AlertsFilter{
		Filter: `alertname="HighCPU"`,
	}

	_, err := client.ListAlerts(filter, "")
	if err != nil {
		t.Fatalf("ListAlerts() error = %v", err)
	}
}

func TestCreateSilence(t *testing.T) {
	mockSilenceID := "silence-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/silences" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		// Decode and validate body
		var silence Silence
		if err := json.NewDecoder(r.Body).Decode(&silence); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Return silence ID
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SilenceResponse{SilenceID: mockSilenceID})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	silence := Silence{
		Matchers: []Matcher{
			{Name: "alertname", Value: "Test", IsRegex: false},
		},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(1 * time.Hour),
		CreatedBy: "test-user",
		Comment:   "Test silence",
	}

	response, err := client.CreateSilence(silence, "")
	if err != nil {
		t.Fatalf("CreateSilence() error = %v", err)
	}

	if response.SilenceID != mockSilenceID {
		t.Errorf("CreateSilence() returned %v, want %v", response.SilenceID, mockSilenceID)
	}
}

func TestDeleteSilence(t *testing.T) {
	silenceID := "silence-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		expectedPath := "/api/v2/silence/" + silenceID
		if r.URL.Path != expectedPath {
			t.Errorf("Unexpected path: %s, want %s", r.URL.Path, expectedPath)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	err := client.DeleteSilence(silenceID, "")
	if err != nil {
		t.Fatalf("DeleteSilence() error = %v", err)
	}
}

func TestClientWithAuth(t *testing.T) {
	expectedUsername := "admin"
	expectedPassword := "secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("Basic auth not provided")
		}
		if username != expectedUsername {
			t.Errorf("Username = %v, want %v", username, expectedUsername)
		}
		if password != expectedPassword {
			t.Errorf("Password = %v, want %v", password, expectedPassword)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Alert{})
	}))
	defer server.Close()

	client := NewClient(server.URL, expectedUsername, expectedPassword, "")

	_, err := client.ListAlerts(AlertsFilter{}, "")
	if err != nil {
		t.Fatalf("ListAlerts() error = %v", err)
	}
}

func TestClientWithTenant(t *testing.T) {
	expectedTenant := "tenant-1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := r.Header.Get("X-Scope-OrgId")
		if tenant != expectedTenant {
			t.Errorf("X-Scope-OrgId = %v, want %v", tenant, expectedTenant)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Alert{})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", expectedTenant)

	_, err := client.ListAlerts(AlertsFilter{}, "")
	if err != nil {
		t.Fatalf("ListAlerts() error = %v", err)
	}
}

func TestClientErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode([]Alert{})
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "", "", "")

			_, err := client.ListAlerts(AlertsFilter{}, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("ListAlerts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListSilences(t *testing.T) {
	mockSilences := []Silence{
		{
			ID:        "silence-1",
			CreatedBy: "admin",
			Comment:   "Maintenance window",
			StartsAt:  time.Now(),
			EndsAt:    time.Now().Add(2 * time.Hour),
			Matchers: []Matcher{
				{Name: "alertname", Value: ".*", IsRegex: true},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/silences" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockSilences)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	silences, err := client.ListSilences("", "")
	if err != nil {
		t.Fatalf("ListSilences() error = %v", err)
	}

	if len(silences) != 1 {
		t.Errorf("ListSilences() returned %d silences, want 1", len(silences))
	}

	if silences[0].ID != "silence-1" {
		t.Errorf("Silence ID = %v, want silence-1", silences[0].ID)
	}
}

func TestGetAlertGroups(t *testing.T) {
	mockGroups := []AlertGroup{
		{
			Labels: map[string]string{
				"alertname": "TestAlert",
			},
			Alerts: []Alert{
				{
					Labels: map[string]string{
						"alertname": "TestAlert",
						"instance":  "server-1",
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/alerts/groups" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockGroups)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	groups, err := client.GetAlertGroups(AlertGroupsFilter{}, "")
	if err != nil {
		t.Fatalf("GetAlertGroups() error = %v", err)
	}

	if len(groups) != 1 {
		t.Errorf("GetAlertGroups() returned %d groups, want 1", len(groups))
	}
}

func TestGetReceivers(t *testing.T) {
	mockReceivers := []map[string]string{
		{"name": "email-receiver"},
		{"name": "slack-receiver"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/receivers" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockReceivers)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	receivers, err := client.GetReceivers("")
	if err != nil {
		t.Fatalf("GetReceivers() error = %v", err)
	}

	if len(receivers) != 2 {
		t.Errorf("GetReceivers() returned %d receivers, want 2", len(receivers))
	}
}

func TestCreateAlert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/alerts" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		var alerts []Alert
		if err := json.NewDecoder(r.Body).Decode(&alerts); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if len(alerts) != 1 {
			t.Errorf("Expected 1 alert, got %d", len(alerts))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	alerts := []Alert{
		{
			Labels: map[string]string{
				"alertname": "TestAlert",
				"severity":  "warning",
			},
			Annotations: map[string]string{
				"summary": "Test alert",
			},
		},
	}

	err := client.CreateAlert(alerts, "")
	if err != nil {
		t.Fatalf("CreateAlert() error = %v", err)
	}
}

// Benchmark tests

func BenchmarkNewClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewClient("http://localhost:9093", "", "", "")
	}
}

func BenchmarkListAlerts(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Alert{})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ListAlerts(AlertsFilter{}, "")
	}
}
