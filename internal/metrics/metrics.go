package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ChatsStartedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "monitoring_assistant_chats_started_total",
			Help: "Total number of new chat sessions initiated.",
		},
	)

	ToolCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monitoring_assistant_tool_calls_total",
			Help: "Total number of tool calls.",
		},
		[]string{"tool_name"},
	)

	LLMTokensTotalByType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monitoring_assistant_llm_tokens_total",
			Help: "Total number of LLM tokens processed.",
		},
		[]string{"model", "token_type"},
	)

	ScratchpadsCreatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "monitoring_assistant_scratchpads_created_total",
			Help: "Total number of scratchpad dashboards created.",
		},
	)

	ErrorsTotalBySource = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "monitoring_assistant_errors_total",
			Help: "Total number of errors by source.",
		},
		[]string{"source"},
	)

	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "assistant_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "assistant_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	LLMRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "assistant_llm_requests_total",
			Help: "Total number of LLM API requests.",
		},
		[]string{"model", "status"},
	)

	LLMRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "assistant_llm_request_duration_seconds",
			Help:    "LLM request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"model"},
	)

	LLMTokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "assistant_llm_tokens_total",
			Help: "Total number of LLM tokens used.",
		},
		[]string{"model", "direction"},
	)

	PromptChars = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "assistant_prompt_chars",
			Help:    "Character length of system prompt sent per chat request.",
			Buckets: []float64{256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536},
		},
	)

	PromptToolCount = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "assistant_prompt_tool_count",
			Help:    "Number of tools exposed to the LLM per chat request.",
			Buckets: []float64{0, 2, 4, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192},
		},
	)

	RequestBudgetTripsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "assistant_request_budget_trips_total",
			Help: "Total number of request budget guardrail trips.",
		},
		[]string{"reason"},
	)

	RequestBudgetEstimatedCostUSD = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "assistant_request_budget_estimated_cost_usd",
			Help:    "Estimated per-request LLM cost (USD) observed during tool loop iterations.",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1},
		},
	)

	MCPToolCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "assistant_mcp_tool_calls_total",
			Help: "Total number of MCP tool calls.",
		},
		[]string{"tool", "status"},
	)

	MCPToolCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "assistant_mcp_tool_call_duration_seconds",
			Help:    "MCP tool call duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tool"},
	)

	ErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "assistant_errors_total",
			Help: "Total number of errors by component.",
		},
		[]string{"component"},
	)
)

func init() {
	prometheus.MustRegister(
		ChatsStartedTotal,
		ToolCallsTotal,
		LLMTokensTotalByType,
		ScratchpadsCreatedTotal,
		ErrorsTotalBySource,
		HTTPRequestsTotal,
		HTTPRequestDuration,
		LLMRequestsTotal,
		LLMRequestDuration,
		LLMTokensTotal,
		PromptChars,
		PromptToolCount,
		RequestBudgetTripsTotal,
		RequestBudgetEstimatedCostUSD,
		MCPToolCallsTotal,
		MCPToolCallDuration,
		ErrorsTotal,
	)
}

// Handler returns an HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
