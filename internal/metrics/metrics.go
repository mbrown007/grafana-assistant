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
		MCPToolCallsTotal,
		MCPToolCallDuration,
		ErrorsTotal,
	)
}

// Handler returns an HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
