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
| assistant_prompt_chars | Histogram |  | Character length of the built system prompt per chat request. |
| assistant_prompt_chars_by_intent | HistogramVec | intent | Character length of the built system prompt, split by intent class. |
| assistant_prompt_tool_count | Histogram |  | Number of tools exposed to the model per chat request. |
| assistant_prompt_tool_count_by_intent | HistogramVec | intent | Number of tools exposed to the model, split by intent class. |
| assistant_prompt_tool_count_filtered | Histogram |  | Number of MCP tools selected after intent-based filtering per chat request. |
| assistant_request_budget_trips_total | Counter | reason | Number of times a request hit a budget guardrail. |
| assistant_request_budget_estimated_cost_usd | Histogram |  | Estimated in-request LLM cost (USD) observed during tool-loop iterations. |

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

- `event` (intent_classification, schema_routing, dashboard_lookup_routing, kb_routing, context_injection, user_message, tool_call, assistant_response, request_budget_trim, request_budget_triggered, error)
- `session_id`
- `user_id`
- `org_id`
- `dashboard_uid` (when available)
- `tool_name` / `params` (tool calls)
- `intent_label` and `intent_confidence` (intent classification events)
- `inject_kb_context`, `kb_route_reason`, `kb_relevance_score`, and `kb_signals` (`kb_routing` events)
- `has_schema_context`, `has_dashboard_lookup_context`, `dashboard_lookup_used_semantic_fallback`, `has_kb_context`, and `has_dashboard_context` (context injection events)

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

## Baseline eval metrics script

Use the baseline eval script to capture a quick quality snapshot from `tests/evals/dataset.yaml`.

Script:

```bash
scripts/eval_baseline.sh
# or
make eval-baseline
# or (mock replay mode, no Docker)
make eval-baseline-mock
```

Outputs:

- Success rate (`success_rate_pct`)
- Tool error rate (`tool_error_rate_pct`)
- Average latency (`avg_latency_ms`)
- Per-case stream diagnostics (`sse_event_count`, `stream_errors`)
- Per-case ordered tool trace (`tool_trace` call/result events)
- Aggregate tool usage/error breakdown (`tool_calls_by_name`, `tool_errors_by_name`)

Notes:

- `tests/evals/dataset.yaml` is currently JSON-formatted; runner also supports true YAML via `yq` conversion when needed.
- `expected.*` fields in dataset cases are reference notes only in `P5-1` and are not yet scored automatically.
- For adding new eval cases, use `docs/EVAL_CASE_AUTHORING.md`.

Result artifact is written to:

```text
tests/evals/results/baseline-<timestamp>.json
```

Optional filters:

```bash
scripts/eval_baseline.sh --category incident_response --limit 10
```

Auth notes:

- `/api/chat` is authenticated in normal deployments.
- Export a valid session cookie before running:

```bash
export ASSISTANT_COOKIE='grafana_session=...'
```

Mock mode notes:

- `scripts/eval_baseline.sh --mock` (or `make eval-baseline-mock`) uses `tests/evals/config.mock.yaml`.
- Mock mode enables `eval_bypass_auth` for local eval-only runs and starts a local assistant automatically when needed.

Optional context defaults:

```bash
export ASSISTANT_DASHBOARD_UID='your_dashboard_uid'
export ASSISTANT_DASHBOARD_NAME='Monitoring Assistant'
export ASSISTANT_TIME_FROM='now-6h'
export ASSISTANT_TIME_TO='now'
```

## Judge scoring pipeline

Use the LLM judge runner to score a baseline run artifact with the rubric in `docs/evals/`.

```bash
scripts/eval_judge.sh -run-artifact tests/evals/results/baseline-<timestamp>.json
# or
make eval-judge RUN_ARTIFACT=tests/evals/results/baseline-<timestamp>.json
```

Judge output artifact:

```text
tests/evals/results/judge-<timestamp>.json
```

The report includes per-case dimension scores/rationales and aggregate pass rate + average weighted score metrics.

## Deterministic guard checks

Use deterministic hard checks (`P5-4`) on the baseline artifact before CI threshold gating:

```bash
scripts/eval_guard.sh -run-artifact tests/evals/results/baseline-<timestamp>.json
# or
make eval-guard RUN_ARTIFACT=tests/evals/results/baseline-<timestamp>.json
```

Guard report artifact:

```text
tests/evals/results/guard-<timestamp>.json
```

By default the command exits non-zero if any case violates guard rules. Use `--allow-failures` for exploratory runs that should still write a report.

## Eval quality gate thresholds

Use the quality gate runner (`P5-5`) to enforce judge + guard thresholds:

```bash
scripts/eval_quality_gate.sh \
  --judge-report tests/evals/results/judge-<timestamp>.json \
  --guard-report tests/evals/results/guard-<timestamp>.json

# or
make eval-quality-gate \
  JUDGE_REPORT=tests/evals/results/judge-<timestamp>.json \
  GUARD_REPORT=tests/evals/results/guard-<timestamp>.json
```

Default policy:

- judge `pass_rate >= 0.85`
- judge hallucination hard failures `<= 0`
- judge error cases `<= 0`
- guard failed cases `<= 0`
- guard violations total `<= 0`

The command exits non-zero when any threshold or consistency check fails.

Feature flag note:

- `ASSISTANT_FEATURE_JUDGE_GATE_MODE=false` (or `--judge-gate-mode=false`) keeps report generation but disables enforcement failure, useful for staged rollout.

CI workflow:

- `.github/workflows/eval-quality-gate.yml` runs baseline -> judge -> guard -> quality gate on pull requests (non-fork PRs), logs into local CI Grafana to obtain a real `grafana_session` cookie for `/api/chat` auth, and uploads the full eval artifact bundle.
