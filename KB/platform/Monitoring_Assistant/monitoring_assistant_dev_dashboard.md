# Monitoring Assistant (Dev) Dashboard

## Overview
This dashboard tracks the runtime health and LLM usage for the Monitoring Assistant dev environment. It is focused on HTTP traffic, latency, and LLM token/cost metrics, plus host logs from Promtail.

**Dashboard UID**: `monitoring-assistant-dev`

---

## Panels and What They Mean

### HTTP Requests / s
**Panel type**: Timeseries

**Query**:
```promql
sum(rate(assistant_http_requests_total[5m]))
```
**Use**: Overall request throughput to the assistant API.

---

### Request Duration (p95)
**Panel type**: Timeseries

**Query**:
```promql
histogram_quantile(0.95, sum(rate(assistant_http_request_duration_seconds_bucket[5m])) by (le))
```
**Use**: API latency for the slowest 5% of requests.

---

### LLM Cost (GBP)
**Panel type**: Stat

**Query**:
```promql
(sum(assistant_llm_tokens_total{direction="prompt"}) or vector(0)) / 1e6 * 0.80 * 0.79 +
(sum(assistant_llm_tokens_total{direction="completion"}) or vector(0)) / 1e6 * 3.20 * 0.79
```
**Use**: Estimated cumulative LLM cost (GBP) for the dev environment.

---

### LLM Cost Over Time (GBP)
**Panel type**: Timeseries (stacked)

**Queries**:
```promql
sum(increase(assistant_llm_tokens_total{direction="prompt"}[5m])) / 1e6 * 0.80 * 0.79
sum(increase(assistant_llm_tokens_total{direction="completion"}[5m])) / 1e6 * 3.20 * 0.79
```
**Use**: Cost rate over time for prompt vs completion tokens.

---

### LLM Tokens
**Panel type**: Stat

**Queries**:
```promql
sum(assistant_llm_tokens_total{direction="prompt"}) or vector(0)
sum(assistant_llm_tokens_total{direction="completion"}) or vector(0)
```
**Use**: Total tokens consumed by prompts and completions.

---

### Avg Tokens / Chat
**Panel type**: Stat

**Queries**:
```promql
(sum(assistant_llm_tokens_total{direction="prompt"}) or vector(0))
/ clamp_min(sum(monitoring_assistant_chats_started_total) or vector(1), 1)

(sum(assistant_llm_tokens_total{direction="completion"}) or vector(0))
/ clamp_min(sum(monitoring_assistant_chats_started_total) or vector(1), 1)
```
**Use**: Average token consumption per chat session.

---

### Host Logs (Promtail)
**Panel type**: Logs

**Query**:
```logql
{app="monitoring-assistant"}
```
**Use**: Application logs for debugging and deployment validation.

---

## Common Troubleshooting

### Requests per second drop to zero
- Check if the backend is running and reachable.
- Confirm the reverse proxy path and base path configuration.
- Verify Prometheus scrape targets for the assistant.

### High p95 latency
- Look for spikes in LLM requests or tool call latency.
- Inspect host logs for timeouts or upstream errors.
- Validate MCP server connectivity and Grafana datasource availability.

### Cost spikes
- Check prompt length and tool call volume.
- Review recent chat transcripts to see if large tool outputs are being included.

---

## Related Metrics
- `assistant_http_requests_total`
- `assistant_http_request_duration_seconds_bucket`
- `assistant_llm_tokens_total{direction="prompt"|"completion"}`
- `monitoring_assistant_chats_started_total`
