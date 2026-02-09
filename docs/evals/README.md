# Eval Judge Rubric and Scoring Schema

This folder defines the `P5-2` rubric/schema contract, `P5-3` judge runner contract, `P5-4` deterministic guard-check contract, and `P5-5` CI quality-gate contract. For `P5-6` eval authoring guidance, see `docs/EVAL_CASE_AUTHORING.md`.

## Files

- `judge-rubric.v1.json`: Versioned scoring rubric with dimension weights, anchors, and pass rules.
- `judge-score.schema.json`: JSON Schema for judge output artifacts.
- `judge-score.sample.json`: Example score artifact that validates against the schema.

## Rubric dimensions

The rubric uses five dimensions from the roadmap:

- `factuality`
- `answer_completeness`
- `tool_usage_correctness`
- `hallucination_risk`
- `actionability`

Each dimension is scored on an integer scale `0..4` where `4` is best.

## Weighting and score math

Weights are defined in `judge-rubric.v1.json` and sum to `1.0`:

- factuality: `0.30`
- answer_completeness: `0.25`
- tool_usage_correctness: `0.20`
- hallucination_risk: `0.15`
- actionability: `0.10`

Score math:

1. Normalize each dimension as `dimension_score / 4`.
2. Compute weighted score: `sum(weight * normalized_dimension_score)`.
3. Weighted score range is `0..1`.

## Pass/fail policy

A case is `pass` only if all conditions are met:

- Weighted score is `>= 0.75`.
- Required minimum dimensions are met: factuality `>= 2`, answer_completeness `>= 2`, hallucination_risk `>= 2`.
- No hard-fail dimension is scored at `<= 1`: factuality, hallucination_risk.

If any hard-fail condition triggers, verdict must be `fail` regardless of weighted score.

## Judge output contract

`judge-score.schema.json` requires:

- Top-level run metadata (`schema_version`, `rubric_id`, `generated_at`, `judge_model`, `dataset`, `run_artifact`)
- Per-case scores with `dimension_scores`, `weighted_score`, `verdict`, `hard_failures`, and per-dimension `rationales`
- Aggregate block with counts (`case_count`, `pass_count`, `fail_count`), rates (`pass_rate`, `average_weighted_score`), average dimension scores, and hard-failure counts

## Validation command

Use `jq` for JSON syntax checks:

```bash
jq -e '.' docs/evals/judge-rubric.v1.json >/dev/null
jq -e '.' docs/evals/judge-score.schema.json >/dev/null
jq -e '.' docs/evals/judge-score.sample.json >/dev/null
```

Schema validation can be done with any Draft 2020-12 compatible validator in `P5-3`.

## Judge runner (`P5-3`)

Score a baseline run artifact with the LLM judge:

```bash
scripts/eval_judge.sh \
  -run-artifact tests/evals/results/baseline-<timestamp>.json \
  -rubric docs/evals/judge-rubric.v1.json

# make shortcut (override artifact path as needed)
make eval-judge RUN_ARTIFACT=tests/evals/results/baseline-<timestamp>.json
```

Environment:

- `ASSISTANT_OPENAI_API_KEY` (or `OPENAI_API_KEY`) is required.
- Optional model override: `ASSISTANT_EVAL_JUDGE_MODEL` (defaults to `gpt-4o-mini`).
- Optional API base URL override: `ASSISTANT_OPENAI_BASE_URL`.

Useful flags:

- `--max-completion-tokens` to tune judge response budget (default `1200`).
- `--fail-fast` to abort on first case-level judge error. By default, case-level judge errors are recorded and run continues.

Output:

- Default report path: `tests/evals/results/judge-<timestamp>.json`
- Includes per-case verdict/scores/rationales and aggregate pass rate + average score metrics.

## Deterministic guard checks (`P5-4`)

Run deterministic, non-LLM hard checks against a baseline run artifact:

```bash
scripts/eval_guard.sh -run-artifact tests/evals/results/baseline-<timestamp>.json
# or
make eval-guard RUN_ARTIFACT=tests/evals/results/baseline-<timestamp>.json
```

Default behavior:

- Writes `tests/evals/results/guard-<timestamp>.json`
- Exits non-zero if any case violates a hard guard rule

Useful flags:

- `--category <name>`: evaluate one category only
- `--limit <n>`: evaluate first N selected cases
- `--allow-failures`: keep exit code `0` while still writing violations (for inspection runs)

Current hard guard rules:

- `no_fabricated_metric_values_without_data`: metric-like claims without live tool results on live-data-expected cases
- `tool_errors_must_be_acknowledged`: assistant must acknowledge degraded evidence/error state when tool errors occurred
- `required_evidence_signal_for_docs_cases`: docs/runbook cases must show KB tool usage or source/citation language

## CI quality gate (`P5-5`)

Evaluate judge + guard reports against deterministic thresholds:

```bash
scripts/eval_quality_gate.sh \
  --judge-report tests/evals/results/judge-<timestamp>.json \
  --guard-report tests/evals/results/guard-<timestamp>.json

# or
make eval-quality-gate \
  JUDGE_REPORT=tests/evals/results/judge-<timestamp>.json \
  GUARD_REPORT=tests/evals/results/guard-<timestamp>.json
```

Default thresholds:

- `min_pass_rate = 0.85`
- `max_hallucination_hard_failures = 0`
- `max_judge_error_cases = 0`
- `max_guard_failed_cases = 0`
- `max_guard_violations_total = 0`

Rollout flag:

- `ASSISTANT_FEATURE_JUDGE_GATE_MODE=true|false` (or `--judge-gate-mode`) controls whether failing checks are enforcement-blocking.
- When disabled, the command still writes the quality gate report/metrics but does not fail the run.

Additional consistency checks (enabled by default):

- judge/guard schema versions must match expected versions
- judge/guard `dataset` fields must match
- judge/guard `run_artifact` fields must match
- judge/guard case counts must match

Output:

- JSON summary report: `tests/evals/results/quality-gate-<timestamp>.json`
- Optional markdown summary via `--summary-md` (used by CI job summaries)

## Mock MCP replay server (`P8-3a`)

For deterministic evals without a live Grafana stack, build and run the mock MCP fixture replay server:

```bash
make mock-mcp-build
./bin/mock-mcp -fixtures tests/evals/fixtures
```

Run the baseline dataset end-to-end in mock mode:

```bash
make eval-baseline-mock
```

CI workflow split (`P8-3d`):
- PRs run the mock-based eval quality gate job (no Docker).
- Scheduled/manual runs execute the live-stack Docker eval job.

Fixtures are stored under `tests/evals/fixtures/` and define:

- `tool_name`: MCP tool identifier (for example `grafana__query_prometheus`)
- `match`: regex patterns evaluated against tool arguments
- `response`: JSON payload returned for matching calls

If no fixture matches a call, the mock server returns a successful generic "no data" payload instead of an MCP error.

Record fixtures from live stack:

```bash
scripts/record_eval_fixtures.sh
```

Before recording, run the assistant with fixture recording enabled:
- `eval_fixture_record_dir: tests/evals/fixtures` in config, or
- `ASSISTANT_EVAL_FIXTURE_RECORD_DIR=tests/evals/fixtures`
