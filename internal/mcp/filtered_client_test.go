package mcp

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type mockClient struct {
	tools         []Tool
	invokedNames  []string
	discoverCalls int
	mu            sync.Mutex
}

func (m *mockClient) Connect(context.Context) error { return nil }
func (m *mockClient) Health(context.Context) error  { return nil }

func (m *mockClient) DiscoverTools(context.Context) ([]Tool, error) {
	m.mu.Lock()
	m.discoverCalls++
	m.mu.Unlock()
	return m.tools, nil
}

func (m *mockClient) InvokeTool(_ context.Context, name string, _ map[string]any) (any, error) {
	m.mu.Lock()
	m.invokedNames = append(m.invokedNames, name)
	m.mu.Unlock()
	return "ok", nil
}

func (m *mockClient) discoverCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.discoverCalls
}

func TestNewFilteredClient_NoFiltersReturnsBase(t *testing.T) {
	base := &mockClient{}
	got := NewFilteredClient(base, "grafana", nil, nil)
	if got != base {
		t.Fatalf("expected original client when no filters are set")
	}
}

func TestFilteredClient_AllowlistAndDenylist(t *testing.T) {
	base := &mockClient{
		tools: []Tool{
			{Name: "grafana__query_prometheus"},
			{Name: "grafana__update_dashboard"},
			{Name: "grafana__search_dashboards"},
		},
	}

	client := NewFilteredClient(base, "grafana",
		[]string{"query_prometheus", "search_dashboards", "update_dashboard"},
		[]string{"update_dashboard"},
	)

	tools, err := client.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("DiscoverTools() error = %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 filtered tools, got %d", len(tools))
	}
	if tools[0].Name != "grafana__query_prometheus" || tools[1].Name != "grafana__search_dashboards" {
		t.Fatalf("unexpected filtered tool list: %#v", tools)
	}
}

func TestFilteredClient_DenylistOnlyBlocksInvoke(t *testing.T) {
	base := &mockClient{
		tools: []Tool{
			{Name: "grafana__query_prometheus"},
			{Name: "grafana__update_dashboard"},
		},
	}

	client := NewFilteredClient(base, "grafana", nil, []string{"grafana__update_dashboard"})

	_, err := client.InvokeTool(context.Background(), "grafana__update_dashboard", nil)
	if err == nil {
		t.Fatalf("expected blocked invoke error for denylisted tool")
	}

	_, err = client.InvokeTool(context.Background(), "grafana__query_prometheus", nil)
	if err != nil {
		t.Fatalf("unexpected invoke error for allowed tool: %v", err)
	}
	if len(base.invokedNames) != 1 || base.invokedNames[0] != "grafana__query_prometheus" {
		t.Fatalf("unexpected invoked tool sequence: %#v", base.invokedNames)
	}
}

func TestFilteredClient_CachesFilteredDiscoverTools(t *testing.T) {
	base := &mockClient{
		tools: []Tool{
			{Name: "grafana__query_prometheus"},
			{Name: "grafana__update_dashboard"},
		},
	}

	client := NewFilteredClient(base, "grafana", []string{"query_prometheus"}, nil)

	first, err := client.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("first DiscoverTools() error = %v", err)
	}
	second, err := client.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("second DiscoverTools() error = %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected 1 filtered tool each time, got first=%d second=%d", len(first), len(second))
	}
	if base.discoverCallCount() != 1 {
		t.Fatalf("expected wrapped client to cache filtered discovery result, calls=%d", base.discoverCallCount())
	}
}

func TestFilteredClient_ConcurrentDiscoverToolsSingleUpstreamCall(t *testing.T) {
	base := &mockClient{
		tools: []Tool{
			{Name: "grafana__query_prometheus"},
			{Name: "grafana__search_dashboards"},
		},
	}

	client := NewFilteredClient(base, "grafana", []string{"query_prometheus", "search_dashboards"}, nil)

	var wg sync.WaitGroup
	errCh := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.DiscoverTools(context.Background())
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected DiscoverTools error: %v", err)
		}
	}

	if base.discoverCallCount() != 1 {
		t.Fatalf("expected one upstream DiscoverTools call under concurrency, calls=%d", base.discoverCallCount())
	}
}

func TestShortToolName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "grafana__query_prometheus", want: "query_prometheus"},
		{in: "query_prometheus", want: "query_prometheus"},
		{in: "__query_prometheus", want: "query_prometheus"},
	}

	for _, tc := range tests {
		got := shortToolName(tc.in)
		if got != tc.want {
			t.Fatalf("shortToolName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFilteredClient_PassesThroughBaseErrors(t *testing.T) {
	base := &mockClientWithError{
		err: errors.New("boom"),
	}
	client := NewFilteredClient(base, "grafana", []string{"query_prometheus"}, nil)
	if _, err := client.DiscoverTools(context.Background()); err == nil {
		t.Fatalf("expected DiscoverTools to return base error")
	}
}

type mockClientWithError struct {
	err error
}

func (m *mockClientWithError) Connect(context.Context) error { return nil }
func (m *mockClientWithError) Health(context.Context) error  { return nil }
func (m *mockClientWithError) DiscoverTools(context.Context) ([]Tool, error) {
	return nil, m.err
}
func (m *mockClientWithError) InvokeTool(context.Context, string, map[string]any) (any, error) {
	return nil, nil
}
