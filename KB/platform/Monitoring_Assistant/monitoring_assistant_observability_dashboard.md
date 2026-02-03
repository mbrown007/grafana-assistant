# Monitoring Assistant Observability Dashboard

## Overview
This dashboard provides an operational view of the Monitoring Assistant: chat volume, tool usage, LLM token usage, error rates, audit logs, and user feedback. It is the primary place to review agent behavior and audit trails.

**Dashboard title**: Monitoring Assistant Observability

---

## High-Level KPIs

### Total Chats
**Query**:
```promql
sum(monitoring_assistant_chats_started_total) or vector(0)
```
**Use**: Overall chat volume.

### Total Tool Calls
**Query**:
```promql
sum(monitoring_assistant_tool_calls_total) or vector(0)
```
**Use**: How frequently tools are being invoked.

### Total LLM Tokens
**Query**:
```promql
sum(monitoring_assistant_llm_tokens_total) or vector(0)
```
**Use**: Aggregate token usage across prompts and completions.

### Total Errors
**Query**:
```promql
sum(monitoring_assistant_errors_total) or vector(0)
```
**Use**: Error volume across all sources.

### Avg Input / Output Tokens per Chat
**Queries**:
```promql
(sum(monitoring_assistant_llm_tokens_total{token_type="prompt"}) or vector(0))
/ clamp_min(sum(monitoring_assistant_chats_started_total) or vector(1), 1)

(sum(monitoring_assistant_llm_tokens_total{token_type="completion"}) or vector(0))
/ clamp_min(sum(monitoring_assistant_chats_started_total) or vector(1), 1)
```
**Use**: Efficiency of chats (token cost per conversation).

### LLM Cost (GBP)
**Query**:
```promql
(sum(monitoring_assistant_llm_tokens_total{token_type="prompt"}) or vector(0)) / 1e6 * 0.80 * 0.79 +
(sum(monitoring_assistant_llm_tokens_total{token_type="completion"}) or vector(0)) / 1e6 * 3.20 * 0.79
```
**Use**: Estimated total cost of LLM usage.

---

## Usage and Error Trends

### Tool Usage
**Panel type**: Bar chart

**Query**:
```promql
sum by (tool_name) (monitoring_assistant_tool_calls_total)
```
**Use**: Which tools are used most.

### Usage Over Time
**Panel type**: Timeseries

**Queries**:
```promql
sum(increase(monitoring_assistant_chats_started_total[$__rate_interval]))
sum(increase(monitoring_assistant_tool_calls_total[$__rate_interval]))
sum(increase(monitoring_assistant_llm_tokens_total[$__rate_interval])) / 1000
```
**Use**: Chat, tool, and token volume trend.

### Errors Over Time (by source)
**Query**:
```promql
sum by (source) (increase(monitoring_assistant_errors_total[$__rate_interval]))
```
**Use**: Identify error sources (llm, mcp, tool_call, etc.).

---

## Audit Logs

### Chat Transcript (Audit Log)
**LogQL**:
```logql
{app="monitoring-assistant", job="audit"}
| json
| session_id=~"$session_id"
| event=~"user_message|assistant_response|tool_call|context_injection|system_prompt|user_feedback"
```
**Use**: Full end-to-end audit trail for a session.

### Context Injections
Shows KB snippets and dashboard context injected into prompts.

### System Prompt Metadata
Shows system prompt length, tool count, and dashboard context flags.

### Recent Scratchpads
Shows where the assistant created scratchpad panels in Grafana.

---

## User Feedback

### Feedback Count
**LogQL**:
```logql
sum(count_over_time({app="monitoring-assistant", job="audit"} | json | event="user_feedback"[$__range]))
```
**Use**: Total feedback events in the current time range.

### Avg Feedback Rating
**LogQL**:
```logql
avg_over_time(
  {app="monitoring-assistant", job="audit"}
  | json
  | event="user_feedback"
  | regexp ".*\\\"rating\\\":(?P<rating>\\d+).*"
  | unwrap rating [$__range]
)
```
**Use**: Average usefulness rating from users.

### User Feedback Log
**LogQL**:
```logql
{app="monitoring-assistant", job="audit"} | json | event="user_feedback"
```
**Use**: Inspect comments and rating payloads.

---

## Common Troubleshooting

### Chat transcript missing entries
- Check the audit log path and promtail config.
- Verify the `job="audit"` label is present.

### Tool usage looks low
- Confirm MCP servers are connected and tools are discoverable.
- Check for tool call errors in the audit log.

### Token usage spike
- Look for unusually long prompts or large tool outputs.
- Review the chat transcript for large payloads.

### Feedback count is zero
- Confirm the feedback API is enabled and reachable.
- Check audit log entries with `event="user_feedback"`.

---

## Related Metrics and Logs
- `monitoring_assistant_chats_started_total`
- `monitoring_assistant_tool_calls_total`
- `monitoring_assistant_llm_tokens_total{token_type="prompt"|"completion"}`
- `monitoring_assistant_errors_total{source}`
- Audit log stream: `{app="monitoring-assistant", job="audit"}`
