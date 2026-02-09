# Eval Case Authoring

Use this page to add new evaluation prompts to `tests/evals/dataset.yaml` without changing any runner code.

## What You Edit

- `tests/evals/dataset.yaml` only.

Do not edit `scripts/eval_baseline.sh`, `cmd/eval-judge`, or `cmd/eval-guard` for normal case additions.

## Required Case Shape

Each case must include:

- `id`
- `category`
- `message`
- `use_default_dashboard_context`
- `expected.answer_notes`
- `expected.tool_usage_notes`
- `expected.expected_tools`

Supported categories:

- `dashboard_summary`
- `anomaly_detection`
- `query_help`
- `incident_response`
- `docs_runbook`

ID prefixes:

- `DSH-###`, `ANO-###`, `QRY-###`, `INC-###`, `DOC-###`

## Copy Template

```json
{
  "id": "QRY-009",
  "category": "query_help",
  "message": "Write a PromQL query for p95 request latency by service over the last 30 minutes.",
  "use_default_dashboard_context": true,
  "expected": {
    "answer_notes": [
      "Returns syntactically valid PromQL",
      "Uses histogram quantile (or equivalent correct approach)",
      "Explains key parts of the query"
    ],
    "tool_usage_notes": [
      "Should use schema/label discovery when useful",
      "Should not fabricate metric names"
    ],
    "expected_tools": [
      "grafana__list_prometheus_metric_names",
      "grafana__list_prometheus_label_names"
    ]
  }
}
```

## Validate Before Committing

```bash
jq -e '.cases and (.cases | type == "array")' tests/evals/dataset.yaml >/dev/null
jq -c '.cases | group_by(.id) | map(select(length > 1) | .[0].id)' tests/evals/dataset.yaml
jq -e '
  .cases
  | all(
      has("id")
      and has("category")
      and has("message")
      and has("use_default_dashboard_context")
      and has("expected")
      and (.expected | has("answer_notes"))
      and (.expected | has("tool_usage_notes"))
      and (.expected | has("expected_tools"))
    )
' tests/evals/dataset.yaml >/dev/null
```

Expected validation results:

- Duplicate ID command returns `[]`.
- Other commands exit successfully.

## Quick Smoke Run

```bash
scripts/eval_baseline.sh --limit 1
```

Category-specific check:

```bash
scripts/eval_baseline.sh --category query_help --limit 1
```

## Do / Don’t

Do:

- Add one case at a time.
- Keep one primary intent per case.
- Use concrete, observable `answer_notes`.
- Add `expected_tools` only when tool behavior is important.

Don’t:

- Don’t require private environment-only details in the prompt.
- Don’t add vague notes like “good answer”.
- Don’t add custom top-level fields unless runner contract changes are approved.

## Full Source Doc

Primary engineering doc:

- `docs/EVAL_CASE_AUTHORING.md`
