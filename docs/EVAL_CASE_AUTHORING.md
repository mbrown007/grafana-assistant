# Eval Case Authoring Guide

This guide shows how to add new eval cases in `tests/evals/dataset.yaml` without changing runner code.

Target audience: junior engineers adding prompts for baseline/judge/guard evaluation.

## Quick Start

1. Pick one category:
- `dashboard_summary`
- `anomaly_detection`
- `query_help`
- `incident_response`
- `docs_runbook`

2. Add one new case object to `tests/evals/dataset.yaml` under `cases`.

3. Use the next ID in that category prefix:
- `DSH-###`, `ANO-###`, `QRY-###`, `INC-###`, `DOC-###`

4. Run validation commands:

```bash
# JSON syntax + required top-level shape
jq -e '.cases and (.cases | type == "array")' tests/evals/dataset.yaml >/dev/null

# Duplicate ID check (should print [])
jq -c '.cases | group_by(.id) | map(select(length > 1) | .[0].id)' tests/evals/dataset.yaml

# Required per-case fields check (should print true)
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

5. Run a small local eval smoke test:

```bash
scripts/eval_baseline.sh --limit 1
```

## Dataset Contract

Top-level keys in `tests/evals/dataset.yaml`:

- `version`
- `created_at`
- `description`
- `defaults`
- `cases` (array)

Each case must include:

- `id`: unique case ID (example: `QRY-009`)
- `category`: one of the 5 supported categories
- `message`: user prompt text to evaluate
- `use_default_dashboard_context`: `true` or `false`
- `expected`:
  - `answer_notes`: expected answer quality notes
  - `tool_usage_notes`: expected tool behavior notes
  - `expected_tools`: preferred/expected tools (can be empty list)

## Case Template

Copy this and edit values:

```json
{
  "id": "QRY-009",
  "category": "query_help",
  "message": "Write a PromQL query for p95 request latency by service over the last 30 minutes.",
  "use_default_dashboard_context": true,
  "expected": {
    "answer_notes": [
      "Returns syntactically valid PromQL",
      "Uses histogram quantile or equivalent correct approach",
      "Explains key parts of the query"
    ],
    "tool_usage_notes": [
      "Should use schema/label discovery when useful",
      "Should avoid fabricating metric names"
    ],
    "expected_tools": [
      "grafana__list_prometheus_metric_names",
      "grafana__list_prometheus_label_names"
    ]
  }
}
```

## Context Rules

- `use_default_dashboard_context: true`:
  - Runner injects default dashboard context from dataset defaults (or env overrides).
- `use_default_dashboard_context: false`:
  - No default dashboard context is injected.
  - Use this for docs/runbook cases that should not depend on dashboard state.

`dashboard_context` per-case override is supported by runner, but avoid introducing it unless you need a specific test scenario.

## Writing Good Cases

Do:

- Write prompts based on real workflows (on-call, query authoring, dashboard triage).
- Keep one core intent per case.
- Make `answer_notes` concrete and observable.
- Add `expected_tools` only when tool usage materially matters.
- Keep category ID ordering tidy (append next number).

Don’t:

- Don’t require private/internal environment knowledge in `message`.
- Don’t include expected numeric values unless deterministic in your environment.
- Don’t make `answer_notes` vague (for example, “good answer”).
- Don’t add new top-level fields or runner-only metadata.
- Don’t edit runner code (`scripts/eval_baseline.sh`, `cmd/eval-judge`, `cmd/eval-guard`) for normal case additions.

## Recommended Authoring Workflow

1. Add one case only.
2. Validate JSON + required fields + duplicate IDs (commands above).
3. Run a small baseline smoke:

```bash
scripts/eval_baseline.sh --category query_help --limit 1
```

4. If you already have judge/guard artifacts, optionally check quality gate:

```bash
scripts/eval_quality_gate.sh \
  --judge-report tests/evals/results/judge-<timestamp>.json \
  --guard-report tests/evals/results/guard-<timestamp>.json \
  --allow-failures
```

5. In your PR/commit notes, state:
- Which case ID you added
- Why this scenario matters
- Baseline command used for quick verification

## FAQ

Q: Is `expected.*` used for strict pass/fail in baseline runner?

A: No. In baseline (`P5-1`) it is reference metadata. It is still important because judge/scoring review context uses it downstream.

Q: Can I add YAML features (comments, anchors) to `dataset.yaml`?

A: Avoid it. The baseline runner validates/parses with `jq`, and `jq` only parses JSON. The file is currently JSON-formatted content at a `.yaml` path for compatibility.
