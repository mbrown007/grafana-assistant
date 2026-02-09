# Baseline eval dataset

This folder contains baseline prompts for evaluating assistant behavior before and after roadmap changes.

## Files

- `dataset.yaml`: Baseline dataset (currently JSON-formatted content at a `.yaml` path for compatibility).
- `results/`: Generated evaluation outputs from `scripts/eval_baseline.sh`.
- `docs/EVAL_CASE_AUTHORING.md`: Step-by-step guide for adding new eval cases safely.

## Dataset shape

Top-level fields:

- `version`
- `created_at`
- `description`
- `defaults`
- `cases` (array)

Each case includes:

- `id`
- `category`
- `message`
- `use_default_dashboard_context`
- `expected`
  - `answer_notes`
  - `tool_usage_notes`
  - `expected_tools`

`expected.*` fields are reference notes for human review in `P5-1`; they are not yet used for automated pass/fail scoring.

## Run

From repo root:

```bash
scripts/eval_baseline.sh
```

Filter by category:

```bash
scripts/eval_baseline.sh --category incident_response --limit 5
```

Authoring new cases:

```text
docs/EVAL_CASE_AUTHORING.md
```

## Result artifact schema

Each run writes a JSON artifact to `tests/evals/results/` with:

- Run metadata: `generated_at`, `dataset`, `api_url`, `selection`
- Aggregate metrics: `totals.success_rate_pct`, `totals.tool_error_rate_pct`, `totals.avg_latency_ms`
- Aggregate tool summaries: `tool_calls_by_name`, `tool_errors_by_name`
- Per-case response fields under `cases[]`: `assistant_text`, `latency_ms`, `http_status`, `error`
- Per-case stream diagnostics under `cases[]`: `sse_event_count`, `stream_errors`
- Per-case tool trace under `cases[]`: `tool_trace` (ordered call/result events with args/results and error flags)

Example artifact:

- `tests/evals/results/sample-baseline-run.json`

The sample artifact is a schema example generated from a no-backend run and intentionally shows `curl_request_failed` with `0%` success.
