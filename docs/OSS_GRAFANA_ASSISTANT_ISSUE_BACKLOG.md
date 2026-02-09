# OSS Grafana Assistant Issue Backlog

Last updated: February 8, 2026
Source roadmap: `docs/OSS_GRAFANA_ASSISTANT_EXECUTION_PLAN.md`

## Purpose

Use this file to track one local ticket per roadmap task (`P0-1` to `P7-5`) and optionally publish to GitHub later.

## Local Ticket Convention

- Local ticket ID format: `LOCAL-{TaskID}` (example: `LOCAL-P2-3`).
- Local assignee uses owner tags in roadmap (`JR`, `MID`, `SR`).
- If/when GitHub issues are created, keep the same Task ID title prefix and add the issue number back to the execution plan.

## Recommended Labels (for optional GitHub publish)

- `roadmap`
- `phase:0` through `phase:7`
- `owner:jr`, `owner:mid`, `owner:sr`
- `type:planning`
- `type:backend`
- `type:frontend`
- `type:testing`
- `type:docs`
- `type:ops`

## Suggested Milestones (for optional GitHub publish)

- `Phase 0 - Planning`
- `Phase 1 - Tool Budget`
- `Phase 2 - Retrieval Router`
- `Phase 3 - Composite Tooling`
- `Phase 4 - Trust UX`
- `Phase 5 - Eval + CI`
- `Phase 6 - Rollout Hardening`
- `Phase 7 - Cloud-Parity Expansion`

## Quick Creation Checklist

- Create/update local tickets using the drafts below (keep title prefix `[P#-#]`).
- Map each task to a local ticket ID (`LOCAL-P#-#`) in `docs/OSS_GRAFANA_ASSISTANT_EXECUTION_PLAN.md`.
- Keep the local board synced in `docs/ROADMAP_LOCAL_BOARD.md`.
- Keep assignee and status in sync with the roadmap tracking matrix.
- If publishing to GitHub later: create labels/milestones/issues and replace local IDs with issue numbers.

---

## Issue Index

| ID | Title | Owner | Phase | Depends On |
|---|---|---|---|---|
| P0-1 | Create issue tracker board and map task IDs to issues | JR | 0 | none |
| P0-2 | Capture baseline eval set (40+ prompts) | MID | 0 | none |
| P0-3 | Add baseline metrics capture script | JR | 0 | P0-2 |
| P0-4 | Maintain roadmap changelog updates per merged task | JR | 0 | P0-1 |
| P1-1 | Constrain Grafana MCP tool categories by default | SR | 1 | P0-1 |
| P1-2 | Add per-server tool allowlist/denylist config | SR | 1 | P0-1 |
| P1-3 | Compact system prompt tool section | SR | 1 | P1-2 |
| P1-4 | Add prompt budget telemetry | MID | 1 | P1-3 |
| P1-5 | Add tests for tool filtering and prompt compaction | JR | 1 | P1-2, P1-3 |
| P2-1 | Implement rule-based request intent classifier | MID | 2 | P1-3 |
| P2-2 | Route live/query intents to schema search first | SR | 2 | P2-1 |
| P2-3 | Route dashboard lookup to metadata-first strategy | MID | 2 | P2-1 |
| P2-4 | Route docs/help intents to KB/vector on demand | MID | 2 | P2-1 |
| P2-5 | Add intent-routing tests with fixtures | JR | 2 | P2-1, P2-2, P2-3, P2-4 |
| P3-1 | Define composite tool contract `investigation__manage` | SR | 3 | P2-2 |
| P3-2 | Implement composite tool router to MCP/internal tools | SR | 3 | P3-1 |
| P3-3 | Add natural-language tool result normalization | MID | 3 | P3-2 |
| P3-4 | Preserve raw evidence while feeding summarized model context | MID | 3 | P3-3 |
| P3-5 | Add syntax-error-aware retry policy | SR | 3 | P3-2 |
| P3-6 | Add tests for composite actions and retries | JR | 3 | P3-2, P3-5 |
| P4-1 | Add response timeline view in chat UI | MID | 4 | P3-4 |
| P4-2 | Add "why this tool" reason text per invocation | MID | 4 | P3-2 |
| P4-3 | Add secret redaction before evidence streaming | SR | 4 | P3-4 |
| P4-4 | Add copy/export for evidence bundle | JR | 4 | P4-1 |
| P4-5 | Add frontend tests for timeline and evidence controls | JR | 4 | P4-1, P4-4 |
| P5-1 | Build eval runner for dataset prompts | MID | 5 | P0-2 |
| P5-2 | Define judge rubric and scoring schema | SR | 5 | P5-1 |
| P5-3 | Implement LLM-as-judge scoring pipeline | SR | 5 | P5-2 |
| P5-4 | Add deterministic non-LLM guard checks | MID | 5 | P5-1 |
| P5-5 | Add CI quality gate thresholds | SR | 5 | P5-3, P5-4 |
| P5-6 | Add eval case authoring guide for juniors | JR | 5 | P5-1 |
| P6-1 | Add feature flags for major capabilities | SR | 6 | P5-5 |
| P6-2 | Add model profile config for prompt variants | SR | 6 | P6-1 |
| P6-3 | Add token/cost budget guardrails | MID | 6 | P6-2 |
| P6-4 | Add incident-mode assistant runbook | JR | 6 | P6-1 |
| P6-5 | Run staged rollout and capture metrics | SR | 6 | P6-1, P6-2, P6-3 |
| P7-1 | Add Slack ChatOps bridge for assistant workflows | SR | 7 | P6-1 |
| P7-2 | Add @ context insertion UX in chat composer | MID | 7 | P6-1 |
| P7-3 | Add guarded dashboard create/edit action pipeline | SR | 7 | P6-1, P6-2 |
| P7-4 | Expand incident/on-call action coverage + deep links | MID | 7 | P6-1 |
| P7-5 | Introduce coordinator + specialist multi-agent scaffold | SR | 7 | P7-3 |

---

## Issue Drafts

Use the following template for each issue:

```md
## Context
(why this task matters)

## Scope
- ...

## Acceptance Criteria
- ...

## Verification
- tests / commands / manual checks

## Links
- Roadmap: docs/OSS_GRAFANA_ASSISTANT_EXECUTION_PLAN.md (Task ID: P#-#)
```

For local-only tracking, set `Local Ticket ID: LOCAL-P#-#` and maintain status in the roadmap tracking matrix.

### P0-1

- Title: `[P0-1] Create issue tracker board and map every roadmap task ID to an issue`
- Labels: `roadmap`, `phase:0`, `owner:jr`, `type:planning`
- Milestone: `Phase 0 - Planning`
- Assignee: `JR`
- Context: Ensure all roadmap work is tracked and assignable.
- Scope:
- Create a local project board/checklist for the roadmap.
- Create/link one local ticket for each `P*` task.
- Acceptance Criteria:
- Every `P*` task in roadmap has a local ticket ID and assignee.
- Backlinks exist from roadmap evidence fields.
- Verification:
- Local board/checklist exists in docs and is up to date.
- Checklist in roadmap updated.

### P0-2

- Title: `[P0-2] Build baseline eval dataset with at least 40 real prompts`
- Labels: `roadmap`, `phase:0`, `owner:mid`, `type:testing`
- Milestone: `Phase 0 - Planning`
- Assignee: `MID`
- Context: Establish a stable benchmark before architecture changes.
- Scope:
- Create `tests/evals/dataset.yaml`.
- Include expected-answer notes and expected tool usage hints.
- Acceptance Criteria:
- Dataset has >= 40 prompts from real workflows.
- Prompt categories include incident, dashboard interpretation, and query generation.
- Verification:
- Dataset committed and reviewed by at least one senior engineer.

### P0-3

- Title: `[P0-3] Add baseline evaluation metrics capture script`
- Labels: `roadmap`, `phase:0`, `owner:jr`, `type:ops`
- Milestone: `Phase 0 - Planning`
- Assignee: `JR`
- Depends on: `P0-2`
- Context: Baseline metrics are needed for before/after comparisons.
- Scope:
- Add `scripts/eval_baseline.sh`.
- Update `docs/OBSERVABILITY.md` with usage instructions.
- Acceptance Criteria:
- Single command outputs success rate, tool error rate, and average latency.
- Script works on local dev setup.
- Verification:
- Attach sample script output from one run.

### P0-4

- Title: `[P0-4] Maintain roadmap changelog entries after each merged task`
- Labels: `roadmap`, `phase:0`, `owner:jr`, `type:docs`
- Milestone: `Phase 0 - Planning`
- Assignee: `JR`
- Depends on: `P0-1`
- Context: Keep roadmap state accurate for delegation and planning.
- Scope:
- Update roadmap changelog for every merged roadmap issue.
- Acceptance Criteria:
- Every completed roadmap issue has dated changelog entry with PR link.
- Verification:
- Random audit of 5 completed issues matches changelog entries.

### P1-1

- Title: `[P1-1] Constrain Grafana MCP default tool categories to reduce context load`
- Labels: `roadmap`, `phase:1`, `owner:sr`, `type:backend`
- Milestone: `Phase 1 - Tool Budget`
- Assignee: `SR`
- Depends on: `P0-1`
- Context: Too many tools inflate prompt/tool context and reduce reliability.
- Scope:
- Update defaults in dev/deploy startup paths to read-focused categories.
- Keep write/admin disabled by default.
- Acceptance Criteria:
- Default MCP startup exposes reduced tool set aligned with read workflows.
- No regression in core monitoring query flows.
- Verification:
- Tool discovery output before/after and integration smoke checks.

### P1-2

- Title: `[P1-2] Add per-server tool allowlist/denylist configuration`
- Labels: `roadmap`, `phase:1`, `owner:sr`, `type:backend`
- Milestone: `Phase 1 - Tool Budget`
- Assignee: `SR`
- Depends on: `P0-1`
- Context: Tool filtering must be configurable without code edits.
- Scope:
- Add config fields for allowlist/denylist by MCP server type.
- Apply filtering before tool registration to model.
- Acceptance Criteria:
- Config can restrict tools deterministically.
- Filter behavior is logged for auditability.
- Verification:
- Unit tests and sample config demonstrating allowlist + denylist.

### P1-3

- Title: `[P1-3] Compact system prompt and remove full tool catalog dump`
- Labels: `roadmap`, `phase:1`, `owner:sr`, `type:backend`
- Milestone: `Phase 1 - Tool Budget`
- Assignee: `SR`
- Depends on: `P1-2`
- Context: Prompt bloat harms model consistency and token efficiency.
- Scope:
- Replace long tool list with concise tool policy and compact high-value list.
- Acceptance Criteria:
- Prompt size is materially reduced.
- Data-related answers still trigger correct tool usage.
- Verification:
- Prompt size metrics before/after and regression test results.

### P1-4

- Title: `[P1-4] Add prompt budget telemetry metrics`
- Labels: `roadmap`, `phase:1`, `owner:mid`, `type:ops`
- Milestone: `Phase 1 - Tool Budget`
- Assignee: `MID`
- Depends on: `P1-3`
- Context: Need visibility into prompt size and tool-count pressure.
- Scope:
- Add metrics for prompt length and tool count per request.
- Acceptance Criteria:
- Metrics available in `/metrics` and dashboarded.
- Verification:
- Metric names documented and observed in local Prometheus.

### P1-5

- Title: `[P1-5] Add tests for tool filtering and prompt compaction`
- Labels: `roadmap`, `phase:1`, `owner:jr`, `type:testing`
- Milestone: `Phase 1 - Tool Budget`
- Assignee: `JR`
- Depends on: `P1-2`, `P1-3`
- Context: Guard against regressions in tool exposure and prompt shape.
- Scope:
- Add unit tests for filtered tool availability and prompt content.
- Acceptance Criteria:
- Tests fail if filtered tools leak into model context.
- Verification:
- `go test ./...` passes with new coverage in agent/config layers.

### P2-1

- Title: `[P2-1] Implement rule-based request intent classifier`
- Labels: `roadmap`, `phase:2`, `owner:mid`, `type:backend`
- Milestone: `Phase 2 - Retrieval Router`
- Assignee: `MID`
- Depends on: `P1-3`
- Context: Retrieval should be intent-driven, not always-on.
- Scope:
- Add intent classifier with categories: `live_data`, `query_help`, `dashboard_lookup`, `how_to_docs`, `incident_summary`.
- Acceptance Criteria:
- Classifier returns deterministic labels with confidence/rationale fields.
- Verification:
- Unit tests with representative prompt fixtures.

### P2-2

- Title: `[P2-2] Route live-data and query-help intents to schema search first`
- Labels: `roadmap`, `phase:2`, `owner:sr`, `type:backend`
- Milestone: `Phase 2 - Retrieval Router`
- Assignee: `SR`
- Depends on: `P2-1`
- Context: For OSS observability, schema context beats generic docs in many cases.
- Scope:
- Prioritize metric/label/datasource discovery before composing advanced queries.
- Acceptance Criteria:
- Query quality improves on eval set.
- Reduced invalid-query failures in tool calls.
- Verification:
- Eval deltas and tool error rate comparisons.

### P2-3

- Title: `[P2-3] Route dashboard lookup to metadata-first, semantic fallback strategy`
- Labels: `roadmap`, `phase:2`, `owner:mid`, `type:backend`
- Milestone: `Phase 2 - Retrieval Router`
- Assignee: `MID`
- Depends on: `P2-1`
- Context: Dashboard retrieval is often exact-name/tag/UID driven.
- Scope:
- Implement metadata-first matching with semantic fallback.
- Acceptance Criteria:
- Exact lookup requests succeed without unnecessary semantic retrieval.
- Verification:
- Integration tests for UID/title/tag lookup paths.

### P2-4

- Title: `[P2-4] Trigger KB/vector retrieval only for docs-help intents`
- Labels: `roadmap`, `phase:2`, `owner:mid`, `type:backend`
- Milestone: `Phase 2 - Retrieval Router`
- Assignee: `MID`
- Depends on: `P2-1`
- Context: Current KB injection timing is coarse; retrieval should be intentional.
- Scope:
- Decouple KB injection from session-change-only heuristic.
- Gate by intent and relevance signals.
- Acceptance Criteria:
- KB/vector usage aligns with intent classes.
- Lower average prompt size where docs are not needed.
- Verification:
- Trace logs showing retrieval decisions by intent.

### P2-5

- Title: `[P2-5] Add intent-routing test suite with fixtures`
- Labels: `roadmap`, `phase:2`, `owner:jr`, `type:testing`
- Milestone: `Phase 2 - Retrieval Router`
- Assignee: `JR`
- Depends on: `P2-1`, `P2-2`, `P2-3`, `P2-4`
- Context: Routing policy needs regression safety.
- Scope:
- Add tests for each intent class and selected retrieval path.
- Acceptance Criteria:
- Tests validate routing decisions and fallback behavior.
- Verification:
- CI test run includes new routing suite.

### P3-1

- Title: `[P3-1] Define composite investigation tool contract investigation__manage`
- Labels: `roadmap`, `phase:3`, `owner:sr`, `type:backend`
- Milestone: `Phase 3 - Composite Tooling`
- Assignee: `SR`
- Depends on: `P2-2`
- Context: Reduce model burden from many low-level tool choices.
- Scope:
- Design schema with `action` plus typed payload (`plan`, `fetch_metrics`, `fetch_logs`, `summarize`, `next_step`).
- Acceptance Criteria:
- Contract documented and registered as an internal tool.
- Verification:
- Contract tests and prompt/tool docs updated.

### P3-2

- Title: `[P3-2] Implement composite investigation router to MCP/internal tooling`
- Labels: `roadmap`, `phase:3`, `owner:sr`, `type:backend`
- Milestone: `Phase 3 - Composite Tooling`
- Assignee: `SR`
- Depends on: `P3-1`
- Context: Composite actions need deterministic backend execution.
- Scope:
- Map composite actions to existing MCP and internal tools.
- Acceptance Criteria:
- All defined actions execute successfully under normal conditions.
- Errors return usable feedback for model self-correction.
- Verification:
- Integration tests for each action route.

### P3-3

- Title: `[P3-3] Add natural-language summary layer for tool results`
- Labels: `roadmap`, `phase:3`, `owner:mid`, `type:backend`
- Milestone: `Phase 3 - Composite Tooling`
- Assignee: `MID`
- Depends on: `P3-2`
- Context: Models often reason better from concise NL summaries than raw JSON alone.
- Scope:
- Add NL summary + compact machine details formatting path.
- Acceptance Criteria:
- Tool context presented to model is shorter and clearer.
- Verification:
- Snapshot tests for formatter outputs.

### P3-4

- Title: `[P3-4] Keep raw evidence in UI while using summarized context for model`
- Labels: `roadmap`, `phase:3`, `owner:mid`, `type:backend`, `type:frontend`
- Milestone: `Phase 3 - Composite Tooling`
- Assignee: `MID`
- Depends on: `P3-3`
- Context: Preserve trust/transparency while optimizing model context.
- Scope:
- Stream raw payload to evidence UI.
- Feed summarized tool content into model memory.
- Acceptance Criteria:
- UI still shows complete tool result payloads.
- Model receives summary-first tool context.
- Verification:
- Manual UI check + unit tests on streamed payload shape.

### P3-5

- Title: `[P3-5] Add retry policy for syntax-related query failures`
- Labels: `roadmap`, `phase:3`, `owner:sr`, `type:backend`
- Milestone: `Phase 3 - Composite Tooling`
- Assignee: `SR`
- Depends on: `P3-2`
- Context: Query syntax failures should self-heal with one guided retry.
- Scope:
- Detect known syntax errors.
- Add single retry with contextual correction hint.
- Acceptance Criteria:
- Syntax error cases retry once and avoid infinite loops.
- Verification:
- Tests for retry and no-retry branches.

### P3-6

- Title: `[P3-6] Add test coverage for composite actions and retry behavior`
- Labels: `roadmap`, `phase:3`, `owner:jr`, `type:testing`
- Milestone: `Phase 3 - Composite Tooling`
- Assignee: `JR`
- Depends on: `P3-2`, `P3-5`
- Context: Composite layer becomes core behavior and needs broad tests.
- Scope:
- Add unit/integration tests for action routing and retry flow.
- Acceptance Criteria:
- Coverage exists for happy-path and failure-path action execution.
- Verification:
- CI shows passing composite test suite.

### P4-1

- Title: `[P4-1] Add per-response timeline view in chat UX`
- Labels: `roadmap`, `phase:4`, `owner:mid`, `type:frontend`
- Milestone: `Phase 4 - Trust UX`
- Assignee: `MID`
- Depends on: `P3-4`
- Context: Incident users need clear sequence of assistant actions.
- Scope:
- Show ordered timeline of reasoning/tool events/result steps.
- Acceptance Criteria:
- Each assistant response can expand to a full event timeline.
- Verification:
- Manual UX walkthrough and screenshot evidence.

### P4-2

- Title: `[P4-2] Add why-this-tool reason text for each tool invocation`
- Labels: `roadmap`, `phase:4`, `owner:mid`, `type:backend`, `type:frontend`
- Milestone: `Phase 4 - Trust UX`
- Assignee: `MID`
- Depends on: `P3-2`
- Context: Users should understand the intent behind each tool call.
- Scope:
- Include concise reason string with tool call events.
- Acceptance Criteria:
- Reason text appears in timeline/evidence for each call.
- Verification:
- API chunk examples and UI validation.

### P4-3

- Title: `[P4-3] Implement secret redaction for streamed evidence payloads`
- Labels: `roadmap`, `phase:4`, `owner:sr`, `type:backend`, `type:security`
- Milestone: `Phase 4 - Trust UX`
- Assignee: `SR`
- Depends on: `P3-4`
- Context: Evidence payloads must not leak secrets or sensitive tokens.
- Scope:
- Redact credentials/tokens/secrets in tool outputs before streaming.
- Acceptance Criteria:
- Redaction policy documented and tested against known patterns.
- Verification:
- Security-focused unit tests and sample redacted payload logs.

### P4-4

- Title: `[P4-4] Add copy/export functionality for full evidence bundle`
- Labels: `roadmap`, `phase:4`, `owner:jr`, `type:frontend`
- Milestone: `Phase 4 - Trust UX`
- Assignee: `JR`
- Depends on: `P4-1`
- Context: Incident postmortems benefit from easy evidence export.
- Scope:
- Add one-click export/copy for message + tool + evidence data.
- Acceptance Criteria:
- Export includes consistent schema and timestamp metadata.
- Verification:
- Manual test with sample exported payload.

### P4-5

- Title: `[P4-5] Add frontend tests for timeline and evidence controls`
- Labels: `roadmap`, `phase:4`, `owner:jr`, `type:testing`, `type:frontend`
- Milestone: `Phase 4 - Trust UX`
- Assignee: `JR`
- Depends on: `P4-1`, `P4-4`
- Context: New UX behavior needs regression protection.
- Scope:
- Add component tests for timeline render and evidence actions.
- Acceptance Criteria:
- Tests cover open/close/copy/export behavior.
- Verification:
- `npm --prefix frontend run test` includes new passing cases.

### P5-1

- Title: `[P5-1] Build evaluation runner for dataset prompts`
- Labels: `roadmap`, `phase:5`, `owner:mid`, `type:testing`, `type:backend`
- Milestone: `Phase 5 - Eval + CI`
- Assignee: `MID`
- Depends on: `P0-2`
- Context: Need repeatable eval execution to compare changes.
- Scope:
- Build runner that captures outputs, tool traces, latency, and errors.
- Acceptance Criteria:
- Runner executes dataset end-to-end and writes machine-readable results.
- Verification:
- Sample run artifact committed or attached.

### P5-2

- Title: `[P5-2] Define LLM-judge rubric and scoring schema`
- Labels: `roadmap`, `phase:5`, `owner:sr`, `type:testing`, `type:docs`
- Milestone: `Phase 5 - Eval + CI`
- Assignee: `SR`
- Depends on: `P5-1`
- Context: Judge scoring must be transparent and stable.
- Scope:
- Define rubric: factuality, answer completeness, tool correctness, hallucination risk, actionability.
- Acceptance Criteria:
- Rubric and JSON schema documented under `docs/evals/`.
- Verification:
- Peer review sign-off on rubric clarity.

### P5-3

- Title: `[P5-3] Implement LLM-as-judge scoring pipeline`
- Labels: `roadmap`, `phase:5`, `owner:sr`, `type:testing`, `type:backend`
- Milestone: `Phase 5 - Eval + CI`
- Assignee: `SR`
- Depends on: `P5-2`
- Context: Automated quality scoring is needed for safe iteration.
- Scope:
- Integrate judge model into eval runner.
- Produce per-case and aggregate scores.
- Acceptance Criteria:
- Pipeline outputs stable score report for full dataset.
- Verification:
- Run report artifact includes judge reasoning and scores.

### P5-4

- Title: `[P5-4] Add deterministic non-LLM guard checks`
- Labels: `roadmap`, `phase:5`, `owner:mid`, `type:testing`
- Milestone: `Phase 5 - Eval + CI`
- Assignee: `MID`
- Depends on: `P5-1`
- Context: Some constraints should be hard-failed without judge variance.
- Scope:
- Add checks for no fabricated values, correct tool-error handling, required evidence signals.
- Acceptance Criteria:
- Guard checks run with eval pipeline and emit pass/fail.
- Verification:
- Demonstrate failing case and passing case outputs.

### P5-5

- Title: `[P5-5] Add CI quality gate thresholds for eval pipeline`
- Labels: `roadmap`, `phase:5`, `owner:sr`, `type:ops`, `type:testing`
- Milestone: `Phase 5 - Eval + CI`
- Assignee: `SR`
- Depends on: `P5-3`, `P5-4`
- Context: Prevent regressions from merging.
- Scope:
- Add CI workflow step with configurable thresholds.
- Acceptance Criteria:
- CI fails when thresholds are not met.
- CI publishes eval summary artifact.
- Verification:
- Workflow run showing pass and intentional fail scenarios.

### P5-6

- Title: `[P5-6] Write junior-friendly eval case authoring guide`
- Labels: `roadmap`, `phase:5`, `owner:jr`, `type:docs`
- Milestone: `Phase 5 - Eval + CI`
- Assignee: `JR`
- Depends on: `P5-1`
- Context: Juniors should add eval cases safely without runner changes.
- Scope:
- Add `docs/EVAL_CASE_AUTHORING.md`.
- Include examples, schema, and do/don't guidance.
- Acceptance Criteria:
- A junior dev can add at least one valid case following the doc.
- Verification:
- Link PR that adds a new eval case via the guide.

### P6-1

- Title: `[P6-1] Add feature flags for major assistant capabilities`
- Labels: `roadmap`, `phase:6`, `owner:sr`, `type:backend`, `type:ops`
- Milestone: `Phase 6 - Rollout Hardening`
- Assignee: `SR`
- Depends on: `P5-5`
- Context: Rollout and rollback require runtime controls.
- Scope:
- Add flags for routing mode, composite tooling mode, judge gate mode, redaction mode.
- Acceptance Criteria:
- Flags can be toggled without redeploying code changes.
- Verification:
- Config examples and runtime behavior checks.

### P6-2

- Title: `[P6-2] Add model profile configuration for prompt variants`
- Labels: `roadmap`, `phase:6`, `owner:sr`, `type:backend`
- Milestone: `Phase 6 - Rollout Hardening`
- Assignee: `SR`
- Depends on: `P6-1`
- Context: Prompt behavior differs by model; profiles enable safer tuning.
- Scope:
- Support per-model prompt/tool profile selection.
- Acceptance Criteria:
- Config can select profiles by model family.
- Verification:
- Profile switch test and documentation update.

### P6-3

- Title: `[P6-3] Add request-level token and cost budget guardrails`
- Labels: `roadmap`, `phase:6`, `owner:mid`, `type:backend`, `type:ops`
- Milestone: `Phase 6 - Rollout Hardening`
- Assignee: `MID`
- Depends on: `P6-2`
- Context: Production usage needs bounded cost and latency.
- Scope:
- Add configurable prompt/tool iteration budgets with graceful fallback.
- Acceptance Criteria:
- Over-budget requests degrade predictably and log reason.
- Verification:
- Tests for budget breach behavior.

### P6-4

- Title: `[P6-4] Write assistant incident-mode operations runbook`
- Labels: `roadmap`, `phase:6`, `owner:jr`, `type:docs`, `type:ops`
- Milestone: `Phase 6 - Rollout Hardening`
- Assignee: `JR`
- Depends on: `P6-1`
- Context: On-call team needs clear operational fallback procedures.
- Scope:
- Add `docs/RUNBOOK_ASSISTANT_INCIDENT_MODE.md`.
- Include fallback modes, feature-flag overrides, and verification steps.
- Acceptance Criteria:
- Runbook is usable by on-call without code-owner assistance.
- Verification:
- Tabletop dry-run sign-off by one on-call engineer.

### P6-5

- Title: `[P6-5] Execute staged rollout and document post-rollout metrics`
- Labels: `roadmap`, `phase:6`, `owner:sr`, `type:ops`
- Milestone: `Phase 6 - Rollout Hardening`
- Assignee: `SR`
- Depends on: `P6-1`, `P6-2`, `P6-3`
- Context: Final rollout should be measured and reversible.
- Scope:
- Run staged rollout with checkpoints.
- Compare baseline vs post-rollout quality and reliability metrics.
- Acceptance Criteria:
- Rollout report published with pass/fail recommendation.
- Verification:
- Report includes metrics, incidents, mitigations, and final decision.

### P7-1

- Title: `[P7-1] Add Slack ChatOps bridge for assistant workflows`
- Labels: `roadmap`, `phase:7`, `owner:sr`, `type:backend`, `type:ops`
- Milestone: `Phase 7 - Cloud-Parity Expansion`
- Assignee: `SR`
- Depends on: `P6-1`
- Context: Extend assistant usage into team incident collaboration channels.
- Scope:
- Add Slack entrypoint that forwards prompts to assistant with user/org context mapping.
- Include response threading and evidence/deep-link formatting for Grafana actions.
- Acceptance Criteria:
- Slack users can query assistant and receive context-aware responses with usable links.
- Verification:
- End-to-end Slack sandbox demo with at least one investigation flow.

### P7-2

- Title: `[P7-2] Add @ context insertion UX in chat composer`
- Labels: `roadmap`, `phase:7`, `owner:mid`, `type:frontend`, `type:backend`
- Milestone: `Phase 7 - Cloud-Parity Expansion`
- Assignee: `MID`
- Depends on: `P6-1`
- Context: Reduce prompt friction for datasource/metric/dashboard context injection.
- Scope:
- Add composer UX for `@` context picks (datasources, metrics, dashboards).
- Pass selected context in API request payload for routing/prompt use.
- Acceptance Criteria:
- Users can inject context entities without manually typing IDs/names.
- Verification:
- UI test coverage + backend request-shape validation.

### P7-3

- Title: `[P7-3] Add guarded dashboard create/edit action pipeline`
- Labels: `roadmap`, `phase:7`, `owner:sr`, `type:backend`, `type:ops`
- Milestone: `Phase 7 - Cloud-Parity Expansion`
- Assignee: `SR`
- Depends on: `P6-1`, `P6-2`
- Context: Safe dashboard authoring/editing requires explicit guardrails and auditability.
- Scope:
- Add assistant action path for create/update dashboard operations with patch-first strategy.
- Enforce confirmation/guard checks and audit log entries for all writes.
- Acceptance Criteria:
- Assistant can create/update dashboards safely with deterministic safeguards.
- Verification:
- Integration tests for allowed/blocked write scenarios and audit traces.

### P7-4

- Title: `[P7-4] Expand incident/on-call action coverage and deep-link workflows`
- Labels: `roadmap`, `phase:7`, `owner:mid`, `type:backend`
- Milestone: `Phase 7 - Cloud-Parity Expansion`
- Assignee: `MID`
- Depends on: `P6-1`
- Context: Incident response flows should be first-class in assistant operations.
- Scope:
- Add/expand tools and orchestration for incident listing, ownership lookups, and on-call routing.
- Improve action response formatting with direct Grafana deep links.
- Acceptance Criteria:
- Assistant can answer incident/on-call tasks with reliable actionability.
- Verification:
- Scenario tests covering incident triage and on-call lookup flows.

### P7-5

- Title: `[P7-5] Introduce coordinator + specialist multi-agent scaffold`
- Labels: `roadmap`, `phase:7`, `owner:sr`, `type:backend`
- Milestone: `Phase 7 - Cloud-Parity Expansion`
- Assignee: `SR`
- Depends on: `P7-3`
- Context: Complex tasks benefit from modular specialist execution with deterministic handoffs.
- Scope:
- Add coordinator pathway that can delegate to specialist handlers (query/dashboard/docs/incident).
- Keep strict boundaries and fallback behavior with centralized traceability.
- Acceptance Criteria:
- Multi-step workflows can delegate safely without regressing reliability.
- Verification:
- Integration tests with at least two delegated workflow scenarios.

Deferred note:

- Tempo/TraceQL-specific expansion remains out of Phase 7 scope until stack requirements change.
