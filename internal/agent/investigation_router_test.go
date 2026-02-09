package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/brownster/grafana-assistant/internal/api"
	"github.com/brownster/grafana-assistant/internal/mcp"
)

type investigationMockClient struct {
	tools         []mcp.Tool
	responses     map[string]func(args map[string]any) (any, error)
	calls         []investigationCall
	discoverCalls int
}

type investigationCall struct {
	name string
	args map[string]any
}

func (m *investigationMockClient) Connect(context.Context) error { return nil }
func (m *investigationMockClient) Health(context.Context) error  { return nil }
func (m *investigationMockClient) DiscoverTools(context.Context) ([]mcp.Tool, error) {
	m.discoverCalls++
	return m.tools, nil
}
func (m *investigationMockClient) InvokeTool(_ context.Context, name string, args map[string]any) (any, error) {
	m.calls = append(m.calls, investigationCall{name: name, args: cloneMap(args)})
	if m.responses == nil {
		return nil, errors.New("no responses configured")
	}
	fn, ok := m.responses[name]
	if !ok {
		return nil, errors.New("no response configured for tool " + name)
	}
	return fn(args)
}

func TestHandleInternalTool_InvestigationDisabled(t *testing.T) {
	disabled := false
	mgr := NewManager(nil, nil, nil, nil, nil, ManagerConfig{
		CompositeToolMode: &disabled,
	})

	resultRaw, err := mgr.handleInternalTool(context.Background(), "investigation__manage", map[string]any{
		"action": "plan",
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := resultRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resultRaw)
	}
	if result["status"] != "tool_unavailable" {
		t.Fatalf("expected status=tool_unavailable, got %v", result["status"])
	}
}

func TestInvestigationManageRoute_FetchMetrics(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name: "grafana__query_prometheus",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"expr":          map[string]any{"type": "string"},
					"datasourceUid": map[string]any{"type": "string"},
					"from":          map[string]any{"type": "string"},
					"to":            map[string]any{"type": "string"},
					"limit":         map[string]any{"type": "integer"},
				},
				"required": []any{"expr"},
			},
		},
	}

	mock := &investigationMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__query_prometheus": func(args map[string]any) (any, error) {
				return map[string]any{
					"series": []any{
						map[string]any{"metric": "http_requests_total", "value": 123},
					},
				}, nil
			},
		},
	}

	mgr := &Manager{
		mcp:   []mcp.Client{mock},
		tools: tools,
	}

	resultRaw, err := mgr.handleInternalTool(context.Background(), "investigation__manage", map[string]any{
		"action":          "fetch_metrics",
		"investigationId": "inv-123",
		"fetch_metrics": map[string]any{
			"query":      `rate(http_requests_total[5m])`,
			"datasource": map[string]any{"uid": "prometheus"},
			"timeRange":  map[string]any{"from": "now-15m", "to": "now"},
			"limit":      200,
		},
	}, nil, nil, &api.DashboardContext{
		TimeRange: map[string]string{"from": "now-30m", "to": "now"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := resultRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resultRaw)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	if result["action"] != investigationActionFetchMetrics {
		t.Fatalf("unexpected action: %v", result["action"])
	}
	if result["tool"] != "grafana__query_prometheus" {
		t.Fatalf("unexpected routed tool: %v", result["tool"])
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected one MCP call, got %d", len(mock.calls))
	}
	if got := mock.calls[0].args["expr"]; got != `rate(http_requests_total[5m])` {
		t.Fatalf("expected expr arg to be routed, got %v", got)
	}
}

func TestInvestigationManageRoute_FetchLogs(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name: "grafana__query_loki_logs",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":         map[string]any{"type": "string"},
					"datasourceUid": map[string]any{"type": "string"},
					"from":          map[string]any{"type": "string"},
					"to":            map[string]any{"type": "string"},
					"direction":     map[string]any{"type": "string"},
				},
				"required": []any{"query"},
			},
		},
	}
	mock := &investigationMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__query_loki_logs": func(args map[string]any) (any, error) {
				return `[{"line":"timeout while connecting to db"}]`, nil
			},
		},
	}
	mgr := &Manager{
		mcp:   []mcp.Client{mock},
		tools: tools,
	}

	resultRaw, err := mgr.handleInternalTool(context.Background(), "investigation__manage", map[string]any{
		"action": "fetch_logs",
		"fetch_logs": map[string]any{
			"query":      `{app="api"} |= "error"`,
			"datasource": map[string]any{"uid": "loki"},
			"direction":  "backward",
		},
	}, nil, nil, &api.DashboardContext{
		TimeRange: map[string]string{"from": "now-1h", "to": "now"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := resultRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resultRaw)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	if result["action"] != investigationActionFetchLogs {
		t.Fatalf("unexpected action: %v", result["action"])
	}
	if result["tool"] != "grafana__query_loki_logs" {
		t.Fatalf("unexpected routed tool: %v", result["tool"])
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected one MCP call, got %d", len(mock.calls))
	}
	if got := mock.calls[0].args["query"]; got != `{app="api"} |= "error"` {
		t.Fatalf("expected query arg to be routed, got %v", got)
	}
}

func TestInvestigationManageRoute_FetchMetricsRetryFallbackPath(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name: "grafana__query_prometheus",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []any{"query"},
			},
		},
	}

	mock := &investigationMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__query_prometheus": func(args map[string]any) (any, error) {
				if _, hasQuery := args["query"]; hasQuery {
					return nil, errors.New("query arg rejected by backend in this scenario")
				}
				if _, hasExpr := args["expr"]; hasExpr {
					return map[string]any{"series": []any{map[string]any{"value": 99}}}, nil
				}
				return nil, errors.New("missing expr")
			},
		},
	}

	mgr := &Manager{
		mcp:   []mcp.Client{mock},
		tools: tools,
	}

	result := mgr.routeInvestigationAction(context.Background(), investigationActionFetchMetrics, "inv-retry", map[string]any{
		"query": "up",
	}, nil)
	if result["status"] != "ok" {
		t.Fatalf("expected retry fallback to succeed, got status=%v", result["status"])
	}
	if len(mock.calls) < 2 {
		t.Fatalf("expected at least two MCP attempts (query then expr fallback), got %d", len(mock.calls))
	}
	if _, hasQuery := mock.calls[0].args["query"]; !hasQuery {
		t.Fatalf("expected first attempt to use query args, got %#v", mock.calls[0].args)
	}
	last := mock.calls[len(mock.calls)-1].args
	if _, hasExpr := last["expr"]; !hasExpr {
		t.Fatalf("expected final fallback attempt to use expr args, got %#v", last)
	}
	if mock.discoverCalls != 1 {
		t.Fatalf("expected one tool discovery with cached tool->client routing, got %d", mock.discoverCalls)
	}
}

func TestInvestigationManageRoute_FetchMetricsUnavailableTool(t *testing.T) {
	mgr := &Manager{
		tools: nil,
		mcp:   nil,
	}

	result := mgr.routeInvestigationAction(context.Background(), investigationActionFetchMetrics, "", map[string]any{
		"query": "up",
	}, nil)
	if result["status"] != "tool_unavailable" {
		t.Fatalf("expected tool_unavailable, got %v", result["status"])
	}
	if result["retryable"] != true {
		t.Fatalf("expected retryable=true, got %v", result["retryable"])
	}
}

func TestInvestigationManageRoute_FetchLogsToolError(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "grafana__query_loki_logs"},
	}
	mock := &investigationMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__query_loki_logs": func(args map[string]any) (any, error) {
				return nil, errors.New("parse error at line 1 col 7")
			},
		},
	}
	mgr := &Manager{
		mcp:   []mcp.Client{mock},
		tools: tools,
	}

	result := mgr.routeInvestigationAction(context.Background(), investigationActionFetchLogs, "inv-err", map[string]any{
		"query": `{app="api"} |~`,
	}, nil)
	if result["status"] != "tool_error" {
		t.Fatalf("expected tool_error, got %v", result["status"])
	}
	if result["retryable"] != true {
		t.Fatalf("expected retryable=true, got %v", result["retryable"])
	}
	if result["tool"] != "grafana__query_loki_logs" {
		t.Fatalf("unexpected tool in error result: %v", result["tool"])
	}
}

func TestInvestigationManageRoute_SyntaxRetrySuccess(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name: "grafana__query_prometheus",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []any{"query"},
			},
		},
	}

	invalid := "sum(rate(http_requests_total[5m])"
	corrected := "sum(rate(http_requests_total[5m]))"
	mock := &investigationMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__query_prometheus": func(args map[string]any) (any, error) {
				q, _ := args["query"].(string)
				if q == corrected {
					return map[string]any{"series": []any{map[string]any{"value": 7}}}, nil
				}
				return nil, errors.New("parse error: unexpected end of input")
			},
		},
	}

	mgr := &Manager{
		mcp:   []mcp.Client{mock},
		tools: tools,
	}

	result := mgr.routeInvestigationAction(context.Background(), investigationActionFetchMetrics, "inv-syntax-success", map[string]any{
		"query": invalid,
	}, nil)
	if result["status"] != "ok" {
		t.Fatalf("expected syntax retry to self-heal and succeed, got status=%v", result["status"])
	}

	syntaxRetry, ok := result["syntaxRetry"].(map[string]any)
	if !ok {
		t.Fatalf("expected syntaxRetry metadata in result, got %T", result["syntaxRetry"])
	}
	if syntaxRetry["detected"] != true || syntaxRetry["applied"] != true {
		t.Fatalf("expected detected/applied=true, got %#v", syntaxRetry)
	}
	if syntaxRetry["correctedQuery"] != corrected {
		t.Fatalf("expected corrected query %q, got %v", corrected, syntaxRetry["correctedQuery"])
	}
	if len(mock.calls) < 2 {
		t.Fatalf("expected at least two attempts (initial + syntax retry), got %d", len(mock.calls))
	}
}

func TestInvestigationManageRoute_NoSyntaxRetryOnNonSyntaxError(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name: "grafana__query_prometheus",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []any{"query"},
			},
		},
	}

	mock := &investigationMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__query_prometheus": func(args map[string]any) (any, error) {
				return nil, errors.New("upstream timeout talking to datasource")
			},
		},
	}

	mgr := &Manager{
		mcp:   []mcp.Client{mock},
		tools: tools,
	}

	query := "up"
	datasource := selectInvestigationDatasource(nil, nil, investigationDefaultMetricDS)
	timeRange := selectInvestigationTimeRange(nil, nil)
	contract := lookupInvestigationToolContract(tools, "grafana__query_prometheus")
	expectedInitialAttempts := len(buildInvestigationArgAttempts(investigationActionFetchMetrics, query, map[string]any{"query": query}, datasource, timeRange, contract))

	result := mgr.routeInvestigationAction(context.Background(), investigationActionFetchMetrics, "inv-nosyntax", map[string]any{
		"query": query,
	}, nil)
	if result["status"] != "tool_error" {
		t.Fatalf("expected tool_error, got %v", result["status"])
	}

	syntaxRetry, ok := result["syntaxRetry"].(map[string]any)
	if !ok {
		t.Fatalf("expected syntaxRetry metadata map, got %T", result["syntaxRetry"])
	}
	if syntaxRetry["detected"] != false || syntaxRetry["applied"] != false {
		t.Fatalf("expected no syntax retry, got %#v", syntaxRetry)
	}
	if len(mock.calls) != expectedInitialAttempts {
		t.Fatalf("expected no extra syntax-retry attempts (calls=%d, expected=%d)", len(mock.calls), expectedInitialAttempts)
	}
}

func TestInvestigationManageRoute_SyntaxRetryOnlyOnceOnPersistentSyntaxErrors(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name: "grafana__query_loki_logs",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []any{"query"},
			},
		},
	}

	query := `{app="api"} |~`
	corrected, _ := suggestSyntaxRetryQuery(investigationActionFetchLogs, query)
	if corrected == query {
		t.Fatalf("expected corrected query to differ for test setup")
	}

	mock := &investigationMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__query_loki_logs": func(args map[string]any) (any, error) {
				return nil, errors.New("syntax error: unexpected end of input")
			},
		},
	}
	mgr := &Manager{
		mcp:   []mcp.Client{mock},
		tools: tools,
	}

	payload := map[string]any{"query": query}
	datasource := selectInvestigationDatasource(nil, nil, investigationDefaultLogsDS)
	timeRange := selectInvestigationTimeRange(nil, nil)
	contract := lookupInvestigationToolContract(tools, "grafana__query_loki_logs")
	initialAttempts := len(buildInvestigationArgAttempts(investigationActionFetchLogs, query, payload, datasource, timeRange, contract))
	retryAttempts := len(buildInvestigationArgAttempts(investigationActionFetchLogs, corrected, payload, datasource, timeRange, contract))

	result := mgr.routeInvestigationAction(context.Background(), investigationActionFetchLogs, "inv-once", payload, nil)
	if result["status"] != "tool_error" {
		t.Fatalf("expected persistent failure to return tool_error, got %v", result["status"])
	}

	syntaxRetry, ok := result["syntaxRetry"].(map[string]any)
	if !ok {
		t.Fatalf("expected syntaxRetry metadata map, got %T", result["syntaxRetry"])
	}
	if syntaxRetry["detected"] != true || syntaxRetry["applied"] != true {
		t.Fatalf("expected syntax retry detected/applied=true, got %#v", syntaxRetry)
	}

	expectedTotalCalls := initialAttempts + retryAttempts
	if len(mock.calls) != expectedTotalCalls {
		t.Fatalf("expected one retry pass only (calls=%d, expected=%d)", len(mock.calls), expectedTotalCalls)
	}
}

func TestIsSyntaxRelatedToolError(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectedMatch  bool
		expectedReason string
	}{
		{
			name:           "nil_error",
			err:            nil,
			expectedMatch:  false,
			expectedReason: "",
		},
		{
			name:           "parse_error",
			err:            errors.New("parse error at line 1"),
			expectedMatch:  true,
			expectedReason: "parse_failure",
		},
		{
			name:           "unterminated_expression",
			err:            errors.New("unexpected EOF in expression"),
			expectedMatch:  true,
			expectedReason: "unterminated_expression",
		},
		{
			name:           "invalid_query_syntax",
			err:            errors.New("bad_data: invalid query"),
			expectedMatch:  true,
			expectedReason: "invalid_query_syntax",
		},
		{
			name:           "non_syntax_error",
			err:            errors.New("upstream timeout"),
			expectedMatch:  false,
			expectedReason: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotMatch, gotReason := isSyntaxRelatedToolError(tc.err)
			if gotMatch != tc.expectedMatch {
				t.Fatalf("expected match=%v, got %v", tc.expectedMatch, gotMatch)
			}
			if gotReason != tc.expectedReason {
				t.Fatalf("expected reason=%q, got %q", tc.expectedReason, gotReason)
			}
		})
	}
}

func TestSuggestSyntaxRetryQuery(t *testing.T) {
	t.Run("empty_query", func(t *testing.T) {
		corrected, hint := suggestSyntaxRetryQuery(investigationActionFetchMetrics, "   ")
		if corrected != "" {
			t.Fatalf("expected empty corrected query, got %q", corrected)
		}
		if !strings.Contains(hint, "non-empty query") {
			t.Fatalf("expected non-empty guidance in hint, got %q", hint)
		}
	})

	t.Run("trim_wrappers_and_terminator", func(t *testing.T) {
		corrected, hint := suggestSyntaxRetryQuery(investigationActionFetchMetrics, " `up`; ")
		if corrected != "up" {
			t.Fatalf("expected corrected query 'up', got %q", corrected)
		}
		if !strings.Contains(strings.ToLower(hint), "conservative syntax correction") {
			t.Fatalf("expected correction hint, got %q", hint)
		}
	})

	t.Run("balance_delimiters", func(t *testing.T) {
		input := "sum(rate(http_requests_total[5m])"
		corrected, _ := suggestSyntaxRetryQuery(investigationActionFetchMetrics, input)
		if corrected != "sum(rate(http_requests_total[5m]))" {
			t.Fatalf("expected balanced query, got %q", corrected)
		}
	})

	t.Run("logs_operator_suffix", func(t *testing.T) {
		input := `{app="api"} |~`
		corrected, _ := suggestSyntaxRetryQuery(investigationActionFetchLogs, input)
		if corrected != `{app="api"} |~ ""` {
			t.Fatalf("expected logs suffix correction, got %q", corrected)
		}
	})

	t.Run("no_safe_change", func(t *testing.T) {
		corrected, hint := suggestSyntaxRetryQuery(investigationActionFetchMetrics, "up")
		if corrected != "up" {
			t.Fatalf("expected unchanged query, got %q", corrected)
		}
		if !strings.Contains(strings.ToLower(hint), "no safe automatic rewrite") {
			t.Fatalf("expected no-safe-rewrite hint, got %q", hint)
		}
	})
}

func TestInvestigationManageRoute_PlanAction(t *testing.T) {
	mgr := &Manager{}
	result := mgr.routeInvestigationAction(context.Background(), investigationActionPlan, "inv-plan", map[string]any{
		"goal":       "Find cause of elevated API errors",
		"scope":      "payments-api",
		"hypotheses": []any{"database latency spike"},
		"constraints": []any{
			"read-only actions",
		},
		"maxSteps": 3,
	}, &api.DashboardContext{
		TimeRange: map[string]string{"from": "now-2h", "to": "now"},
	})

	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	plan, ok := result["plan"].(map[string]any)
	if !ok {
		t.Fatalf("expected plan object, got %T", result["plan"])
	}
	if plan["goal"] != "Find cause of elevated API errors" {
		t.Fatalf("unexpected goal: %v", plan["goal"])
	}
	steps, ok := plan["steps"].([]string)
	if !ok || len(steps) == 0 {
		t.Fatalf("expected non-empty plan steps, got %#v", plan["steps"])
	}
	if len(steps) > 3 {
		t.Fatalf("expected max 3 steps after clamp, got %d", len(steps))
	}
}

func TestInvestigationManageRoute_SummarizeAction(t *testing.T) {
	mgr := &Manager{}
	result := mgr.routeInvestigationAction(context.Background(), investigationActionSummarize, "inv-sum", map[string]any{
		"objective": "Summarize investigation output",
		"findings": []any{
			"Error rate spiked at 10:12 UTC",
			"DB connection saturation matched spike",
		},
		"severity":        "critical",
		"includeEvidence": true,
	}, nil)

	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary object, got %T", result["summary"])
	}
	if summary["severity"] != "critical" {
		t.Fatalf("unexpected severity: %v", summary["severity"])
	}
	if summary["findingsCount"] != 2 {
		t.Fatalf("expected findingsCount=2, got %v", summary["findingsCount"])
	}
}

func TestInvestigationManageRoute_NextStepAction(t *testing.T) {
	mgr := &Manager{}
	result := mgr.routeInvestigationAction(context.Background(), investigationActionNextStep, "inv-next", map[string]any{
		"currentState":    "Metrics and logs indicate DB pressure",
		"options":         []any{"scale db pool", "rollback deploy"},
		"preferredOption": "rollback deploy",
		"needsApproval":   true,
		"blockers":        []any{"change window requires on-call lead approval"},
	}, nil)

	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	decision, ok := result["decision"].(map[string]any)
	if !ok {
		t.Fatalf("expected decision object, got %T", result["decision"])
	}
	if decision["recommendedOption"] != "rollback deploy" {
		t.Fatalf("unexpected recommended option: %v", decision["recommendedOption"])
	}
	if decision["approvalRequired"] != true {
		t.Fatalf("expected approvalRequired=true, got %v", decision["approvalRequired"])
	}
}

func TestInvestigationManageRoute_StrictListParsingRejectsMapShape(t *testing.T) {
	mgr := &Manager{}
	result := mgr.routeInvestigationAction(context.Background(), investigationActionSummarize, "inv-shape", map[string]any{
		"findings": map[string]any{
			"items": []any{"a", "b"},
		},
	}, nil)

	if result["status"] != "input_error" {
		t.Fatalf("expected input_error for non-array findings payload, got %v", result["status"])
	}
}

func TestInvestigationManageRoute_ActionInputErrors(t *testing.T) {
	mgr := &Manager{}

	testCases := []struct {
		name           string
		action         string
		payload        map[string]any
		expectedErrMsg string
	}{
		{
			name:           "plan_missing_goal",
			action:         investigationActionPlan,
			payload:        map[string]any{},
			expectedErrMsg: "plan.goal is required",
		},
		{
			name:           "fetch_metrics_missing_query",
			action:         investigationActionFetchMetrics,
			payload:        map[string]any{},
			expectedErrMsg: "fetch_metrics.query is required",
		},
		{
			name:           "fetch_logs_missing_query",
			action:         investigationActionFetchLogs,
			payload:        map[string]any{},
			expectedErrMsg: "fetch_logs.query is required",
		},
		{
			name:           "summarize_missing_findings",
			action:         investigationActionSummarize,
			payload:        map[string]any{},
			expectedErrMsg: "summarize.findings must include at least one item",
		},
		{
			name:           "next_step_missing_current_state",
			action:         investigationActionNextStep,
			payload:        map[string]any{},
			expectedErrMsg: "next_step.currentState is required",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result := mgr.routeInvestigationAction(context.Background(), tc.action, "inv-input-errors", tc.payload, nil)
			if result["status"] != "input_error" {
				t.Fatalf("expected input_error, got %v", result["status"])
			}
			if result["action"] != tc.action {
				t.Fatalf("expected action %q, got %v", tc.action, result["action"])
			}
			if result["retryable"] != true {
				t.Fatalf("expected retryable=true, got %v", result["retryable"])
			}
			if msg := toString(result["message"]); !strings.Contains(msg, tc.expectedErrMsg) {
				t.Fatalf("expected message to contain %q, got %q", tc.expectedErrMsg, msg)
			}
		})
	}
}

func TestHandleInternalTool_InvestigationManage_InternalActionsHappyPath(t *testing.T) {
	mgr := &Manager{}

	testCases := []struct {
		name       string
		args       map[string]any
		payloadKey string
		action     string
	}{
		{
			name: "plan",
			args: map[string]any{
				"action":          "plan",
				"investigationId": "inv-plan-internal",
				"plan": map[string]any{
					"goal": "Investigate increased API latency",
				},
			},
			payloadKey: "plan",
			action:     investigationActionPlan,
		},
		{
			name: "summarize",
			args: map[string]any{
				"action": "summarize",
				"summarize": map[string]any{
					"objective": "Summarize findings",
					"findings":  []any{"error spike", "db saturation"},
				},
			},
			payloadKey: "summarize",
			action:     investigationActionSummarize,
		},
		{
			name: "next_step",
			args: map[string]any{
				"action": "next_step",
				"next_step": map[string]any{
					"currentState": "Initial evidence indicates db connection pressure",
				},
			},
			payloadKey: "next_step",
			action:     investigationActionNextStep,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resultRaw, err := mgr.handleInternalTool(context.Background(), "investigation__manage", tc.args, nil, nil, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			result, ok := resultRaw.(map[string]any)
			if !ok {
				t.Fatalf("expected map result, got %T", resultRaw)
			}
			if result["status"] != "ok" {
				t.Fatalf("expected status=ok, got %v", result["status"])
			}
			if result["action"] != tc.action {
				t.Fatalf("expected action %q, got %v", tc.action, result["action"])
			}
			if result["payloadKey"] != tc.payloadKey {
				t.Fatalf("expected payloadKey=%q, got %v", tc.payloadKey, result["payloadKey"])
			}
		})
	}
}

func TestHandleInternalTool_InvestigationManage_FetchMetricsSyntaxRetry(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name: "grafana__query_prometheus",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []any{"query"},
			},
		},
	}

	invalid := "sum(rate(http_requests_total[5m])"
	corrected := "sum(rate(http_requests_total[5m]))"
	mock := &investigationMockClient{
		tools: tools,
		responses: map[string]func(args map[string]any) (any, error){
			"grafana__query_prometheus": func(args map[string]any) (any, error) {
				q, _ := args["query"].(string)
				if q == corrected {
					return map[string]any{"series": []any{map[string]any{"value": 42}}}, nil
				}
				return nil, errors.New("parse error: unexpected end of input")
			},
		},
	}

	mgr := &Manager{
		mcp:   []mcp.Client{mock},
		tools: tools,
	}

	resultRaw, err := mgr.handleInternalTool(context.Background(), "investigation__manage", map[string]any{
		"action": "fetch_metrics",
		"fetch_metrics": map[string]any{
			"query": invalid,
		},
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := resultRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resultRaw)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok after syntax retry, got %v", result["status"])
	}
	if result["payloadKey"] != "fetch_metrics" {
		t.Fatalf("expected payloadKey=fetch_metrics, got %v", result["payloadKey"])
	}
	syntaxRetry, ok := result["syntaxRetry"].(map[string]any)
	if !ok {
		t.Fatalf("expected syntaxRetry metadata map, got %T", result["syntaxRetry"])
	}
	if syntaxRetry["detected"] != true || syntaxRetry["applied"] != true {
		t.Fatalf("expected detected/applied=true, got %#v", syntaxRetry)
	}
	if syntaxRetry["correctedQuery"] != corrected {
		t.Fatalf("expected correctedQuery=%q, got %v", corrected, syntaxRetry["correctedQuery"])
	}
	if len(mock.calls) < 2 {
		t.Fatalf("expected at least two calls (initial + retry), got %d", len(mock.calls))
	}
}

func TestHandleInternalTool_InvestigationManage_MissingPayloadIsStructuredInputError(t *testing.T) {
	mgr := &Manager{}

	resultRaw, err := mgr.handleInternalTool(context.Background(), "investigation__manage", map[string]any{
		"action": "fetch_logs",
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected structured input_error result, got err=%v", err)
	}

	result, ok := resultRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resultRaw)
	}
	if result["status"] != "input_error" {
		t.Fatalf("expected input_error, got %v", result["status"])
	}
	if result["action"] != investigationActionFetchLogs {
		t.Fatalf("expected action=%q, got %v", investigationActionFetchLogs, result["action"])
	}
	if result["retryable"] != true {
		t.Fatalf("expected retryable=true, got %v", result["retryable"])
	}
	msg := toString(result["message"])
	if !strings.Contains(msg, "fetch_logs payload is required") {
		t.Fatalf("expected missing payload message, got %q", msg)
	}
}

func TestHandleInternalTool_InvestigationParseErrorIsStructured(t *testing.T) {
	mgr := &Manager{}

	resultRaw, err := mgr.handleInternalTool(context.Background(), "investigation__manage", map[string]any{
		"action": "unknown_action",
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected structured input_error result, got err=%v", err)
	}
	result, ok := resultRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resultRaw)
	}
	if result["status"] != "input_error" {
		t.Fatalf("expected input_error status, got %v", result["status"])
	}
	if result["retryable"] != true {
		t.Fatalf("expected retryable=true, got %v", result["retryable"])
	}
}

func TestCloneMap_DeepCloneNestedMaps(t *testing.T) {
	original := map[string]any{
		"datasource": map[string]any{
			"uid":  "prometheus",
			"type": "prometheus",
		},
		"timeRange": map[string]string{
			"from": "now-1h",
			"to":   "now",
		},
	}

	cloned := cloneMap(original)
	clonedDatasource := cloned["datasource"].(map[string]any)
	clonedDatasource["uid"] = "mutated"

	clonedTimeRange := cloned["timeRange"].(map[string]string)
	clonedTimeRange["from"] = "now-2h"

	originalDatasource := original["datasource"].(map[string]any)
	if originalDatasource["uid"] != "prometheus" {
		t.Fatalf("expected original nested datasource map unchanged, got %v", originalDatasource["uid"])
	}
	originalTimeRange := original["timeRange"].(map[string]string)
	if originalTimeRange["from"] != "now-1h" {
		t.Fatalf("expected original nested time range unchanged, got %v", originalTimeRange["from"])
	}
}
