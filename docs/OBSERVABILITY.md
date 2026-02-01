# Observability (Prometheus + Loki + Grafana)

This guide documents metrics, logging, and a ready-to-import Grafana dashboard for the monitoring assistant.

## Prometheus metrics

The application exposes Prometheus metrics at `/metrics`.

### Metrics emitted

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| monitoring_assistant_chats_started_total | Counter |  | Total number of new chat sessions initiated. |
| monitoring_assistant_tool_calls_total | Counter | tool_name | Total number of tool calls. |
| monitoring_assistant_llm_tokens_total | Counter | model, token_type (prompt/completion) | Total number of tokens processed by the LLM. |
| monitoring_assistant_scratchpads_created_total | Counter |  | Total number of scratchpad dashboards created. |
| monitoring_assistant_errors_total | Counter | source (llm, grafana_api, tool_call) | Total number of errors, categorized by source. |

### Prometheus scrape config snippet

Add the assistant as a scrape target (replace host/port if different):

```yaml
scrape_configs:
  - job_name: monitoring-assistant
    metrics_path: /metrics
    static_configs:
      - targets:
          - monitoring-assistant:8080
```

## Loki logging

The application uses structured JSON logs via `slog`. Each log event is tagged with:

- `event` (user_message, tool_call, assistant_response, error)
- `session_id`
- `user_id`
- `org_id`
- `dashboard_uid` (when available)
- `tool_name` / `params` (tool calls)

This lets you reconstruct session flows in Grafana with a single session ID filter.

### Loki query example

Use the following query in a Logs panel to view a full chat transcript:

```
{app="monitoring-assistant"} | json | session_id="$session_id"
```

### Promtail (or Grafana Agent) config snippet

Example for Promtail (adapt the paths and labels as needed):

```yaml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: monitoring-assistant
    static_configs:
      - targets:
          - localhost
        labels:
          app: monitoring-assistant
          __path__: /var/log/monitoring-assistant/*.log
```

## Grafana dashboard

Import the dashboard JSON below (Grafana: **Dashboards → New → Import**). It uses Prometheus for metrics and Loki for logs.

### Variables

- `session_id`: populated from Loki log labels via the query below.
- `user_id`: populated from Loki log labels via the query below.

### Dashboard JSON

The dashboard JSON is stored in `docs/agent_monitoring.json`.

### Notes

- Replace data source UIDs (`PROMETHEUS`, `LOKI`) with your actual data source UIDs.
- If your Loki logs do not promote JSON keys into labels, keep the `| json` stage in queries (as shown).
- The Recent Scratchpads panel uses tool call logs. If you want dashboard URLs, add `url` to the tool response log payload and include it in the table transformation.
