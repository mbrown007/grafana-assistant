# OSS Grafana Assistant Execution Plan

Last updated: February 8, 2026
Scope: Monitoring Assistant OSS roadmap based on Grafana Assistant interview insights and current repository state.

## How to use this document

- Use this file as the single execution tracker for the assistant roadmap.
- Each task has a unique ID so you can assign it in tickets.
- Mark task checkboxes as work moves forward.
- Keep "Evidence" links updated with PR/local refs, issue/ticket IDs, and test results.

Status legend:

- `[ ]` Not started
- `[~]` In progress
- `[x]` Done
- `[!]` Blocked

Owner legend:

- `JR`: Junior dev friendly
- `MID`: Mid-level dev
- `SR`: Senior dev (architecture, cross-cutting risk)

---

## Objectives

1. Keep the agent loop simple and controllable.
2. Reduce prompt and tool surface area to improve reliability.
3. Use context routing so the assistant decides when to use dashboard context, schema search, metadata search, and docs/vector retrieval.
4. Improve trust UX by showing the assistant's tool usage and evidence clearly.
5. Add repeatable evals (including LLM-as-judge) so prompt/model changes are safe.

---

## Current baseline (already present)

- [x] Custom iterative tool loop with max iterations (`internal/agent/manager.go`).
- [x] Tool call errors are passed back into model context (`internal/agent/manager.go`).
- [x] Dashboard-aware prompt context (`internal/agent/prompts.go` and `internal/context/enricher.go`).
- [x] Evidence UX for tool calls, KB search, and vector search (`frontend/src/components/ChatPanel.tsx`, `frontend/src/components/EvidenceModal.tsx`).
- [x] Hybrid token + vector KB context (`internal/agent/kb_context.go`).
- [x] User feedback capture and audit trail (`internal/api/feedback.go`, `internal/storage/sqlite.go`).

Main gaps to close:

- Tool overload and prompt bloat from sending many MCP tools at once.
- No explicit retrieval policy router (schema-first vs docs-first).
- No structured eval harness and no LLM judge flow yet.
- No stage gates for quality regression by model/prompt version.

---

## Phase summary

| Phase | Goal | Owner | ETA |
|---|---|---|---|
| Phase 0 | Planning and baseline metrics | SR + JR | 2-3 days |
| Phase 1 | Tool budget and prompt compaction | SR + MID | 4-6 days |
| Phase 2 | Retrieval router (schema/docs/metadata) | SR + MID | 5-8 days |
| Phase 3 | Composite investigation tool pattern | SR + MID | 5-7 days |
| Phase 4 | UX trust and explainability upgrades | MID + JR | 4-6 days |
| Phase 5 | Eval harness + LLM judge + CI gate | SR + MID + JR | 6-9 days |
| Phase 6 | Hardening and rollout controls | SR + MID | 4-6 days |
| Phase 7 | Cloud-parity feature expansion (Tempo/TraceQL deferred) | SR + MID + JR | 7-10 days |

---

## Tracking matrix

Update this table whenever an issue/local ticket is created, status changes, or a PR merges.

| Task | Owner | Status | Issue/Ticket | PR | Notes |
|---|---|---|---|---|---|
| P0-1 | JR | `[x]` | LOCAL-P0-1 | TBD | Local ticket mapping completed (`LOCAL-P*`) and board added in `docs/ROADMAP_LOCAL_BOARD.md` |
| P0-2 | MID | `[x]` | LOCAL-P0-2 | TBD | Baseline dataset added at `tests/evals/dataset.yaml` |
| P0-3 | JR | `[x]` | LOCAL-P0-3 | TBD | Baseline runner added at `scripts/eval_baseline.sh` |
| P0-4 | JR | `[~]` | LOCAL-P0-4 | TBD | Changelog process active with local-first tracking |
| P1-1 | SR | `[x]` | LOCAL-P1-1 | local | Read-focused Grafana MCP defaults set in dev + stdio deploy configs (write/admin disabled) |
| P1-2 | SR | `[x]` | LOCAL-P1-2 | local | Per-server allowlist/denylist config wired with MCP filter wrapper + tests |
| P1-3 | SR | `[x]` | LOCAL-P1-3 | local | Replaced full tool dump with compact policy + ranked high-value tool summary |
| P1-4 | MID | `[x]` | LOCAL-P1-4 | local | Added prompt budget metrics for prompt chars + tool count per request |
| P1-5 | JR | `[x]` | LOCAL-P1-5 | local | Added tests for MCP allow/deny filtering and compact prompt tool summary behavior |
| P2-1 | MID | `[x]` | LOCAL-P2-1 | local | Deterministic request intent classifier with confidence/rationale and tests |
| P2-2 | SR | `[x]` | LOCAL-P2-2 | local | Schema-first routing injects datasource/metric/label context before LLM reasoning for live/query intents |
| P2-3 | MID | `[x]` | LOCAL-P2-3 | local | Dashboard lookup now routes metadata/tag search first with semantic fallback only on metadata miss |
| P2-4 | MID | `[x]` | LOCAL-P2-4 | local | KB/vector retrieval now gated by docs/help intent + relevance signals (not session-change heuristics) |
| P2-5 | JR | `[x]` | LOCAL-P2-5 | local | Fixture-driven routing tests added across all intent classes and retrieval paths |
| P3-1 | SR | `[x]` | LOCAL-P3-1 | local | Added `investigation__manage` internal tool contract schema, parser validation, and prompt/test coverage |
| P3-2 | SR | `[x]` | LOCAL-P3-2 | local | Implemented composite `investigation__manage` router with MCP/internal action handling and integration tests |
| P3-3 | MID | `[x]` | LOCAL-P3-3 | local | Added summary-first tool-result formatter (NL summary + machine details) with coverage |
| P3-4 | MID | `[x]` | LOCAL-P3-4 | local | Locked raw streamed tool payloads for evidence UI while model memory uses summary-shaped tool context |
| P3-5 | SR | `[x]` | LOCAL-P3-5 | local | Syntax-error-aware single guided retry with corrected query context + tests |
| P3-6 | JR | `[x]` | LOCAL-P3-6 | local | Expanded composite action + retry-path coverage across route and internal tool entrypoints |
| P4-1 | MID | `[x]` | LOCAL-P4-1 | local | Per-response timeline view added with ordered tool/retry/final-answer events |
| P4-2 | MID | `[x]` | LOCAL-P4-2 | local | Tool invocation reason text streamed and rendered in tool evidence + timeline |
| P4-3 | SR | `[x]` | LOCAL-P4-3 | local | Redaction layer masks secrets/tokens in streamed tool + evidence payloads |
| P4-4 | JR | `[x]` | LOCAL-P4-4 | local | One-click copy/export evidence bundle with schema + timestamp metadata |
| P4-5 | JR | `[x]` | LOCAL-P4-5 | local | Added timeline/evidence control tests including open-close-copy-export flows |
| P5-1 | MID | `[x]` | LOCAL-P5-1 | local | Eval runner captures per-case outputs, ordered tool traces, latency, and stream/tool errors |
| P5-2 | SR | `[x]` | LOCAL-P5-2 | local | Versioned judge rubric and scoring JSON schema documented under `docs/evals/` |
| P5-3 | SR | `[x]` | LOCAL-P5-3 | local | Added judge runner that scores baseline artifacts per-case with rubric and outputs aggregate metrics |
| P5-4 | MID | `[x]` | LOCAL-P5-4 | local | Added deterministic non-LLM hard guard checks with fail-on-violation runner + report |
| P5-5 | SR | `[x]` | LOCAL-P5-5 | local | Added CI quality-gate workflow and deterministic threshold runner with summary artifacts |
| P5-6 | JR | `[x]` | LOCAL-P5-6 | local | Added junior-friendly eval authoring guide with template, validation, and do/don't rules |
| P6-1 | SR | `[x]` | LOCAL-P6-1 | local | Added runtime feature flags for routing/composite/judge-gate/redaction with config+env overrides and manager/eval gate enforcement wiring |
| P6-2 | SR | `[x]` | LOCAL-P6-2 | local | Added model-profile resolver (explicit/default/family overrides) and prompt/tool prompt variant wiring by model family with tests/docs |
| P6-3 | MID | `[x]` | LOCAL-P6-3 | local | Added per-request token/tool/cost guardrails with degrade-on-budget behavior, runtime config/env tuning, and budget telemetry |
| P6-4 | JR | `[ ]` | LOCAL-P6-4 | TBD | Incident-mode runbook |
| P6-5 | SR | `[ ]` | LOCAL-P6-5 | TBD | Staged rollout + post-rollout report |
| P7-1 | SR | `[ ]` | LOCAL-P7-1 | TBD | Slack ChatOps bridge for assistant queries/investigations (excluding Tempo/TraceQL flows) |
| P7-2 | MID | `[x]` | LOCAL-P7-2 | local | Added `@` context picker with backend search endpoint, MCP-backed entity search, LLM injection, and frontend picker/chips UX |
| P7-3 | SR | `[ ]` | LOCAL-P7-3 | TBD | Add guarded dashboard create/edit action pipeline with patch-first updates |
| P7-4 | MID | `[ ]` | LOCAL-P7-4 | TBD | Expand incident/on-call action coverage and deep-link workflows |
| P7-5 | SR | `[ ]` | LOCAL-P7-5 | TBD | Introduce coordinator+specialist multi-agent execution scaffold |

---

## Phase 0: Planning and baseline metrics

Phase status: `[x]`

### Tasks

- [x] `P0-1` Create issue tracker board and map every task ID in this file to an issue.
Owner: `JR`
Done when: Every task in this document has an issue/ticket ID and assignee.
Evidence: Local issue map in tracking matrix (`LOCAL-P*`), board in `docs/ROADMAP_LOCAL_BOARD.md`, and issue drafts in `docs/OSS_GRAFANA_ASSISTANT_ISSUE_BACKLOG.md`.

- [x] `P0-2` Capture a baseline eval set (at least 40 real prompts from your incident and dashboard workflows).
Owner: `MID`
Done when: Dataset exists in `tests/evals/dataset.yaml` with expected answer notes and expected tool usage notes.
Evidence: `tests/evals/dataset.yaml`, `tests/evals/README.md`.

- [x] `P0-3` Add baseline metrics capture script for current branch.
Owner: `JR`
Files: `scripts/eval_baseline.sh` (new), `docs/OBSERVABILITY.md`
Done when: One command records success rate, tool error rate, and average response latency for the baseline set.
Evidence: `scripts/eval_baseline.sh`, `docs/OBSERVABILITY.md`.

- [~] `P0-4` Add "roadmap changelog" section updates to this file after each merged phase task.
Owner: `JR`
Done when: Each merged task appends date, PR/local ref, and result in the changelog section.
Evidence: Initial changelog entries added below.

---

## Phase 1: Tool budget and prompt compaction

Phase status: `[x]`

Goal: Reduce context pressure from large tool lists and long prompts while preserving capability.

### Tasks

- [x] `P1-1` Constrain Grafana MCP tool categories in dev and deploy startup commands.
Owner: `SR`
Files: `Makefile`, `deploy/config.stdio.example.yaml`, optional runtime docs.
Done when: Default startup enables only needed categories for read-focused operations and excludes write/admin by default.
Evidence: `Makefile`, `deploy/config.stdio.example.yaml`, `dev/lab/config.yaml`, `README.md`, `docs/DEPLOYMENT.md`, `deploy/assistant.env.example`.

- [x] `P1-2` Add configurable per-server tool allowlist/denylist in assistant config.
Owner: `SR`
Files: `internal/config/config.go`, MCP client wiring.
Done when: Config can explicitly filter discovered tools before they are passed to OpenAI.
Evidence: `internal/config/config.go`, `internal/mcp/filtered_client.go`, `internal/mcp/filtered_client_test.go`, `internal/config/config_test.go`, `cmd/assistant/main.go`, `config.example.yaml`, `deploy/config.stdio.example.yaml`.

- [x] `P1-3` Stop dumping full tool catalog into the system prompt.
Owner: `SR`
Files: `internal/agent/prompts.go`
Done when: Prompt includes only concise tool policy plus a compact list of currently enabled high-value tools.
Evidence: `internal/agent/prompts.go`, `internal/agent/prompts_test.go`.

- [x] `P1-4` Add prompt budget telemetry.
Owner: `MID`
Files: `internal/metrics/metrics.go`, `internal/agent/manager.go`
Done when: Metrics expose prompt char length and tool count per request.
Evidence: `internal/metrics/metrics.go`, `internal/metrics/metrics_test.go`, `internal/agent/manager.go`, `docs/OBSERVABILITY.md`.

- [x] `P1-5` Add tests for filtered tool sets and prompt compaction.
Owner: `JR`
Files: `internal/agent/prompts_test.go`, new tests for config/tool filtering.
Done when: Tests prove filtered tools are the only tools available to the model.
Evidence: `internal/mcp/filtered_client_test.go`, `internal/config/config_test.go`, `internal/agent/prompts_test.go`.

---

## Phase 2: Retrieval router (schema-first, metadata-first, docs-on-demand)

Phase status: `[x]`

Goal: Trigger the right retrieval path per question instead of broad retrieval every time.

### Tasks

- [x] `P2-1` Implement request intent classifier (rule-based first).
Owner: `MID`
Files: `internal/agent/` (new `intent.go`)
Done when: Requests are labeled into categories such as `live_data`, `query_help`, `dashboard_lookup`, `how_to_docs`, `incident_summary`.
Evidence: `internal/agent/intent.go`, `internal/agent/intent_test.go`, `internal/agent/manager.go`.

- [x] `P2-2` Route `query_help` and `live_data` to schema search first.
Owner: `SR`
Files: `internal/agent/manager.go`, MCP tool selection logic.
Done when: Model gets metric/label/datasource schema context before it attempts complex PromQL/LogQL generation.
Evidence: `internal/agent/schema_context.go`, `internal/agent/schema_context_test.go`, `internal/agent/manager.go`.

- [x] `P2-3` Route dashboard-find requests to metadata/tag search first, semantic fallback second.
Owner: `MID`
Done when: Dashboard lookup uses exact/metadata strategy first and only uses semantic retrieval when exact lookup fails.
Evidence: `internal/agent/dashboard_routing.go`, `internal/agent/dashboard_routing_test.go`, `internal/agent/manager.go`, `internal/agent/schema_context.go`.

- [x] `P2-4` Route docs/help requests to KB/vector retrieval only when intent requires docs.
Owner: `MID`
Files: `internal/agent/kb_context.go`, `internal/agent/manager.go`
Done when: KB injection is no longer tied only to new session or dashboard change.
Evidence: `internal/agent/kb_routing.go`, `internal/agent/kb_routing_test.go`, `internal/agent/manager.go`, `internal/agent/schema_context.go`.

- [x] `P2-5` Add intent-routing tests with fixtures.
Owner: `JR`
Files: new tests under `internal/agent/`
Done when: Tests cover all intent classes and verify selected retrieval path.
Evidence: `internal/agent/intent_routing_fixtures_test.go`, `internal/agent/testdata/intent_routing_fixtures.json`, `internal/agent/intent_test.go`, `internal/agent/schema_context_test.go`, `internal/agent/dashboard_routing_test.go`, `internal/agent/kb_routing_test.go`.

---

## Phase 3: Composite investigation tool pattern (tool overloading)

Phase status: `[x]`

Goal: Replace many low-level tool selection decisions with a smaller high-level action API where appropriate.

### Tasks

- [x] `P3-1` Define composite tool contract `investigation__manage`.
Owner: `SR`
Files: `internal/agent/tools.go` plus implementation file.
Actions: `plan`, `fetch_metrics`, `fetch_logs`, `summarize`, `next_step`.
Done when: A single tool schema with `action` and typed payload is available to the model.
Evidence: `internal/agent/investigation_contract.go`, `internal/agent/tools.go`, `internal/agent/tools_test.go`, `internal/agent/prompts.go`, `internal/agent/prompts_test.go`.

- [x] `P3-2` Implement composite tool router to existing MCP/internal tools.
Owner: `SR`
Done when: Composite actions call underlying MCP/internal tools and normalize outputs.
Evidence: `internal/agent/investigation_router.go`, `internal/agent/manager.go`, `internal/agent/investigation_router_test.go`, `internal/agent/prompts.go`, `internal/agent/prompts_test.go`.

- [x] `P3-3` Add natural-language result normalization for tool outputs.
Owner: `MID`
Files: `internal/mcp/formatter.go` or new formatter module.
Done when: Tool results include concise NL summary plus machine details, improving model comprehension.
Evidence: `internal/mcp/formatter.go`, `internal/mcp/formatter_test.go`, `internal/agent/manager.go`.

- [x] `P3-4` Keep raw payload available in evidence UI while feeding summarized output to model.
Owner: `MID`
Files: `internal/agent/manager.go`, frontend evidence components.
Done when: UI transparency is preserved and model context stays compact.
Evidence: `internal/agent/manager.go`, `internal/agent/tool_result_path_test.go`, `internal/api/types_test.go`, `frontend/src/components/ChatPanel.tsx`, `frontend/src/components/EvidenceModal.tsx`.

- [x] `P3-5` Add failure-retry policy for query syntax errors.
Owner: `SR`
Done when: On known syntax errors, agent gets one guided retry with corrected query context.
Evidence: `internal/agent/investigation_router.go`, `internal/agent/investigation_router_test.go`.

- [x] `P3-6` Add tests for composite actions and retries.
Owner: `JR`
Done when: Tests validate action routing, retries, and fallback behavior.
Evidence: `internal/agent/investigation_router_test.go`.

---

## Phase 4: UX trust and explainability upgrades

Phase status: `[~]`

Goal: Make assistant workflow visible and audit-friendly during incidents.

### Tasks

- [x] `P4-1` Add response timeline view per assistant message.
Owner: `MID`
Files: `frontend/src/components/ChatPanel.tsx`, `frontend/src/components/EvidenceModal.tsx`
Done when: User can see ordered steps (tool call, result, retry, final answer).
Evidence: `frontend/src/types.ts`, `frontend/src/components/ChatPanel.tsx`, `frontend/src/components/EvidenceModal.tsx`, `frontend/src/components/__tests__/ChatPanel.test.tsx`.

- [x] `P4-2` Add "why this tool" reason text for each tool invocation.
Owner: `MID`
Done when: Model/tool layer includes short reason string for each call.
Evidence: `internal/agent/manager.go`, `internal/agent/tool_reason_test.go`, `internal/api/types.go`, `internal/api/types_test.go`, `frontend/src/types.ts`, `frontend/src/components/ChatPanel.tsx`, `frontend/src/components/EvidenceModal.tsx`, `frontend/src/components/__tests__/ChatPanel.test.tsx`.

- [x] `P4-3` Add secret redaction in evidence payload before frontend stream.
Owner: `SR`
Files: API streaming and tool result shaping path.
Done when: Tokens/credentials/secrets are masked consistently.
Evidence: `internal/agent/redaction.go`, `internal/agent/redaction_test.go`, `internal/agent/manager.go`, `internal/agent/tool_result_path_test.go`, `docs/RESPONSE_EVIDENCE_UI.md`.

- [x] `P4-4` Add copy/export support for evidence bundle for incident postmortem.
Owner: `JR`
Done when: One-click copy of message content plus tool/evidence payload.
Evidence: `frontend/src/types.ts`, `frontend/src/components/ChatPanel.tsx`, `frontend/src/components/__tests__/ChatPanel.test.tsx`.

- [x] `P4-5` Add frontend tests for timeline and evidence controls.
Owner: `JR`
Files: `frontend/src/components/__tests__/`
Done when: Tests pass for new trust UX components.
Evidence: `frontend/src/components/__tests__/ChatPanel.test.tsx`.

---

## Phase 5: Eval harness, LLM judge, and CI quality gate

Phase status: `[ ]`

Goal: Prevent regressions across prompt/model/tool changes and automate quality checks.

### Tasks

- [x] `P5-1` Build eval runner for dataset prompts.
Owner: `MID`
Files: `cmd/` or `scripts/` eval runner.
Done when: Runner executes prompt set and captures outputs, tool traces, latency, and errors.
Evidence:
- `scripts/eval_baseline.sh`
- `tests/evals/README.md`
- `tests/evals/results/sample-baseline-run.json`

- [x] `P5-2` Add judge rubric and scoring schema.
Owner: `SR`
Rubric: factuality, question answered, tool usage correctness, hallucination risk, actionability.
Done when: JSON rubric and scoring documentation exists under `docs/evals/`.
Evidence:
- `docs/evals/judge-rubric.v1.json`
- `docs/evals/judge-score.schema.json`
- `docs/evals/README.md`
- `docs/evals/judge-score.sample.json`

- [x] `P5-3` Implement LLM-as-judge scoring pipeline.
Owner: `SR`
Done when: Runner produces per-case score and aggregate metrics.
Evidence:
- `cmd/eval-judge/main.go`
- `internal/evals/scoring.go`
- `scripts/eval_judge.sh`
- `docs/evals/README.md`

- [x] `P5-4` Add deterministic guard checks (non-LLM) for hard constraints.
Owner: `MID`
Checks: no fabricated metric values, tool error handling, required citation/evidence presence when needed.
Done when: Hard checks fail build when violated.
Evidence:
- `cmd/eval-guard/main.go`
- `internal/evals/guards.go`
- `internal/evals/guards_test.go`
- `scripts/eval_guard.sh`
- `Makefile`
- `docs/evals/README.md`

- [x] `P5-5` Add CI gate with thresholds.
Owner: `SR`
Threshold examples: pass rate >= 85%, hallucination violations = 0 on guarded checks.
Done when: CI blocks merges below threshold and posts summary artifact.
Evidence:
- `.github/workflows/eval-quality-gate.yml`
- `cmd/eval-quality-gate/main.go`
- `cmd/eval-quality-gate/main_test.go`
- `internal/evals/quality_gate.go`
- `internal/evals/quality_gate_test.go`
- `scripts/eval_quality_gate.sh`
- `Makefile`
- `docs/evals/README.md`

- [x] `P5-6` Add junior-friendly eval case authoring guide.
Owner: `JR`
Files: `docs/EVAL_CASE_AUTHORING.md`
Done when: Juniors can add evals without touching runner code.
Evidence:
- `docs/EVAL_CASE_AUTHORING.md`
- `tests/evals/README.md`
- `docs/evals/README.md`

---

## Phase 6: Hardening and controlled rollout

Phase status: `[~]`

Goal: Safely ship improvements with rollback and visibility.

### Tasks

- [x] `P6-1` Add feature flags for each major capability.
Owner: `SR`
Flags: routing mode, composite tool mode, judge gate mode, evidence redaction mode.
Done when: Features can be toggled without code rollback.
Evidence:
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/agent/manager.go`
- `internal/agent/prompts.go`
- `internal/agent/tools.go`
- `cmd/assistant/main.go`
- `cmd/eval-quality-gate/main.go`
- `cmd/eval-quality-gate/main_test.go`
- `config.example.yaml`
- `wiki/Configuration.md`
- `docs/OBSERVABILITY.md`
- `docs/evals/README.md`
- `.github/workflows/eval-quality-gate.yml`

- [x] `P6-2` Add model profile config for prompt variants per model.
Owner: `SR`
Files: config and prompt builder path.
Done when: Different system/tool prompts can be selected by model family.
Evidence:
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/agent/prompt_profile.go`
- `internal/agent/prompt_profile_test.go`
- `internal/agent/prompts.go`
- `internal/agent/prompts_test.go`
- `internal/agent/tools.go`
- `internal/agent/tools_test.go`
- `internal/agent/manager.go`
- `cmd/assistant/main.go`
- `config.example.yaml`
- `wiki/Configuration.md`
- `deploy/assistant.env.example`
- `deploy/config.stdio.example.yaml`
- `dev/lab/config.yaml`
- `docs/DEPLOYMENT.md`

- [x] `P6-3` Add token/cost budget guardrails per request.
Owner: `MID`
Done when: Request aborts gracefully or degrades strategy when budget is exceeded.
Evidence:
- `internal/agent/request_budget.go`
- `internal/agent/request_budget_test.go`
- `internal/agent/manager.go`
- `internal/llm/openai.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/metrics/metrics.go`
- `internal/metrics/metrics_test.go`
- `cmd/assistant/main.go`
- `config.example.yaml`
- `deploy/assistant.env.example`
- `deploy/config.stdio.example.yaml`
- `dev/lab/config.yaml`
- `docs/OBSERVABILITY.md`
- `docs/DEPLOYMENT.md`
- `wiki/Configuration.md`
- `wiki/Deployment.md`

- [ ] `P6-4` Add runbook for incident mode operations.
Owner: `JR`
Files: `docs/RUNBOOK_ASSISTANT_INCIDENT_MODE.md`
Done when: On-call team has step-by-step fallback and verification workflow.
Evidence:

- [ ] `P6-5` Perform staged rollout and collect post-rollout metrics.
Owner: `SR`
Done when: Baseline vs rollout metrics documented and accepted.
Evidence:

---

## Phase 7: Cloud-parity feature expansion (Tempo/TraceQL deferred)

Phase status: `[~]`

Goal: Close high-value Grafana Cloud assistant capability gaps that apply to our stack today, while explicitly deferring Tempo/TraceQL-specific work.

### Tasks

- [ ] `P7-1` Add Slack ChatOps bridge for assistant query/investigation workflows.
Owner: `SR`
Done when: Slack users can ask assistant questions and receive evidence-linked responses backed by existing MCP/Grafana flows.
Evidence:

- [x] `P7-2` Add `@` context insertion UX in chat composer for datasources/metrics/dashboards.
Owner: `MID`
Done when: Users can inject relevant context entities without manual ID lookup/copy-paste.
Evidence:
- `internal/api/types.go` (ContextEntity/ContextSearchResponse types, ChatRequest extended)
- `internal/api/context_search.go` (HTTP handler)
- `internal/api/context_search_test.go` (6 handler tests)
- `internal/agent/context_search.go` (MCP-backed search + buildSelectedContextBlock)
- `internal/agent/context_search_test.go` (13 unit tests)
- `internal/agent/manager.go` ([Selected Context] injection + audit)
- `cmd/assistant/main.go` (route registration)
- `frontend/src/types.ts` (frontend entity types, ChatRequest extended)
- `frontend/src/services/api.ts` (contextSearchApi)
- `frontend/src/hooks/useContextSearch.ts` (debounced search hook)
- `frontend/src/components/ContextPicker.tsx` (popover with tabs/keyboard/search)
- `frontend/src/components/ContextChips.tsx` (removable entity pills)
- `frontend/src/components/ChatPanel.tsx` ("@" detection, picker/chips integration)
- `frontend/src/components/__tests__/ContextPicker.test.tsx` (10 tests)
- `frontend/src/components/__tests__/ContextChips.test.tsx` (4 tests)
- `frontend/src/components/__tests__/ChatPanel.test.tsx` (updated mock)

- [ ] `P7-3` Add guarded dashboard create/edit action pipeline (patch-first).
Owner: `SR`
Done when: Assistant can safely create/update dashboards with explicit guardrails and audit visibility.
Evidence:

- [ ] `P7-4` Expand incident/on-call action coverage and deep-link workflows.
Owner: `MID`
Done when: Assistant can reliably surface incidents, ownership, and on-call routing actions with direct Grafana links.
Evidence:

- [ ] `P7-5` Introduce coordinator + specialist multi-agent scaffold.
Owner: `SR`
Done when: Complex workflows can be delegated to specialized sub-agents with deterministic handoff boundaries and tests.
Evidence:

Deferred note:

- Tempo/TraceQL-specific assistant expansion is intentionally deferred until stack adoption requires it.

---

## Junior-dev task queue (safe delegation first)

- [x] `JR-1` `P0-1` task board + issue linking.
- [~] `JR-2` `P0-4` roadmap changelog maintenance.
- [x] `JR-3` `P1-5` tool filtering and prompt tests.
- [x] `JR-4` `P2-5` intent routing tests and fixtures.
- [x] `JR-5` `P3-6` composite tool action tests.
- [x] `JR-6` `P4-5` frontend timeline/evidence tests.
- [x] `JR-7` `P5-6` eval case authoring guide.
- [ ] `JR-8` `P6-4` incident runbook doc.

---

## Senior-only tasks (architecture and risk)

- [x] `SR-1` `P1-2` config-driven tool filtering design and implementation.
- [x] `SR-2` `P2-2` schema-first routing and tool strategy.
- [x] `SR-3` `P3-1` and `P3-2` composite tool contract and execution path.
- [x] `SR-4` `P5-3` LLM-as-judge pipeline design.
- [x] `SR-5` `P5-5` CI quality thresholds and merge gate.
- [x] `SR-6` `P6-1` feature flag rollout and fallback model.
- [ ] `SR-7` `P7-3`/`P7-5` dashboard action architecture + multi-agent scaffold.

---

## Definition of done per phase

- [ ] `DoD-1` Code merged with tests passing.
- [ ] `DoD-2` Observability updated for new behavior.
- [ ] `DoD-3` User-facing docs updated if behavior changes.
- [ ] `DoD-4` Eval set rerun and result posted.
- [ ] `DoD-5` This roadmap file updated with PR/local refs.

---

## Changelog

Use this format:

- `YYYY-MM-DD` Task ID(s): short summary. PR: `#123` (or `local`). Owner: `name`.

Entries:

- `2026-02-07` Task `P0-2`: Added baseline eval dataset with 40 prompts and expected answer/tool usage notes. PR: `TBD`. Owner: `codex`.
- `2026-02-07` Task `P0-3`: Added baseline eval runner script and observability documentation for baseline metrics capture. PR: `TBD`. Owner: `codex`.
- `2026-02-07` Task `P0-4` (in progress): Started changelog tracking process with initial entries. PR: `TBD`. Owner: `codex`.
- `2026-02-08` Task `P0-1`: Completed local ticket mapping for all roadmap tasks using `LOCAL-P*` IDs in the tracking matrix. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P0-4` (in progress): Continued changelog maintenance in local-first mode (no GitHub issue dependency). PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P1-1`: Constrained default Grafana MCP categories for dev and stdio deploy flows to read-focused tools with write/admin disabled by default. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P1-2`: Added per-server MCP tool allowlist/denylist configuration and enforcement before model tool exposure, with filter logging and unit tests. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P1-3`: Replaced full prompt tool catalog dump with compact tool policy and a ranked high-value tool summary to reduce prompt bloat. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P1-4`: Added prompt budget telemetry histograms for system prompt length and LLM-exposed tool count per chat request. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P1-5`: Added tests for MCP tool filtering and prompt compaction to prevent filtered tool leakage and full catalog regressions. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P2-1`: Added deterministic rule-based request intent classification with confidence/rationale fields and unit tests; classifier now logged/audited per request. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P2-2`: Added schema-first retrieval routing for `live_data`/`query_help` with datasource + metric/label context prefetch and schema-tool prioritization in MCP tool ordering, plus unit tests. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P2-3`: Added metadata-first dashboard lookup routing with deterministic query candidates (UID/title/tag), semantic fallback only on metadata miss, and intent-aware tool ordering/tests. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P2-4`: Added docs/help-only KB/vector routing with intent + relevance signal gating, explicit `kb_routing` decision logs, and docs-intent tool prioritization tests. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P2-5`: Added fixture-driven intent-routing tests covering all intent classes and verifying selected retrieval path + tool-priority behavior. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-1`: Added composite internal tool contract `investigation__manage` with typed action payload schema, parser validation, and prompt/contract tests. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-2`: Implemented composite action router for `investigation__manage` with deterministic MCP/internal execution (`plan`, `fetch_metrics`, `fetch_logs`, `summarize`, `next_step`), structured retryable error feedback, and integration-style tests per action route. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-2` (review hardening): Added cached tool->client MCP routing to avoid per-attempt re-discovery, strict investigation list payload parsing, deep-clone safety for retry args, structured parse-failure responses, explicit fallback-path tests, and configurable investigation tool timeout wiring. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-3`: Added summary-first tool result formatting for model context with concise NL summaries + machine details (with truncation guard), including composite-status and JSON-text parsing tests. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-4`: Confirmed and locked dual-path tool result handling so raw payloads continue streaming to evidence UI while model memory receives wrapped summary-formatted tool context, with regression tests on shaping and streamed result object shape. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-3`/`P3-4` (review hardening): Reduced formatter machine-detail budget, added multi-collection and heterogeneous-array summary coverage, made truncation rune-safe, and added nil-path shaping test coverage. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-5`: Added syntax-error-aware fetch retry policy for composite investigation actions, including conservative query correction hints, one-pass retry safety, and retry/no-retry test coverage. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-6`: Expanded composite investigation test coverage with action-level input-error matrix, end-to-end `handleInternalTool` routing checks for internal actions, structured parse/payload failure checks, and syntax-retry integration assertions. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-5`/`P3-6` (review hardening): Aligned formatter machine-detail truncation budget to 1200 chars for tighter model context control and added explicit formatter budget-limit test coverage. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P3-5`/`P3-6` (review hardening follow-up): Removed stale retry-attempt error labeling, added isolated syntax helper unit tests, documented shared timeout semantics for initial+retry investigation fetch attempts, and clarified delimiter-balancer string-context limitation. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P4-1`: Added expandable per-response timeline UI with ordered stream events (start, tool call, retry, tool result, final answer, error) and timeline modal rendering/tests. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P4-2`: Added per-tool "why this tool" reason text by deriving rationale in backend tool-call streaming and rendering reason details in frontend tool evidence/timeline views, with unit and integration test coverage. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P4-3`: Added stream-side secret redaction for tool arguments/results and KB/vector evidence payloads with key-aware + pattern-aware masking rules, documentation of policy, and security-focused unit tests. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P4-4`: Added one-click evidence bundle copy/export actions for assistant responses with a stable JSON schema (`evidence_bundle.v1`), export timestamp metadata, and frontend test coverage for copy/export behavior. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P4-5`: Expanded frontend trust-UX tests to cover timeline and evidence modal controls (open/close flows across tool/KB/vector/timeline) plus copy/export bundle status behavior. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P4-3`/`P4-5` (review hardening): Optimized redaction hot path to avoid JSON round-trip for native map/list payloads, made tool-reason truncation rune-safe, documented aggressive key matching and raw-audit intent, and deep-cloned nested evidence bundle payloads with added nested-structure test assertions. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-1`: Extended eval runner artifacts to include ordered per-case tool traces plus SSE diagnostics, and documented output schema/metrics for repeatable local eval runs. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-1` (review hardening): Added YAML parsing fallback via `yq`, tightened tool-result error classification to reduce false positives, made SSE line extraction binary-safe, added portable epoch-ms fallback for non-GNU `date`, and clarified dataset/sample artifact expectations in docs. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-2`: Added versioned LLM-judge rubric JSON, judge score report schema, and scoring documentation/sample artifact under `docs/evals/` for stable transparent scoring in `P5-3`. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-3`: Added LLM-as-judge runner (`cmd/eval-judge`) with per-case structured scoring, deterministic rubric pass/fail calculation, aggregate metrics output, and local run wiring/docs (`scripts/eval_judge.sh`, `make eval-judge`). PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-2`/`P5-3` (review hardening): Corrected sample weighted-score math, added required-minimum fail-path scoring test coverage, made judge completion token budget configurable, switched judge runner to continue-on-case-error by default with `--fail-fast` override, and tightened score schema traceability requirements for `judge_model`/`run_artifact`. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-4`: Added deterministic guard runner (`cmd/eval-guard`) with hard checks for fabricated metric claims, tool-error acknowledgment, and docs evidence signals; wired local execution via `scripts/eval_guard.sh` and `make eval-guard`, with guard docs and unit coverage. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-5`: Added deterministic quality-gate runner (`cmd/eval-quality-gate`) with configurable thresholds, consistency checks, and markdown/json summaries; wired CI workflow (`.github/workflows/eval-quality-gate.yml`) to run baseline->judge->guard->gate and upload eval artifact bundles while failing merges on threshold violations. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-6`: Added junior-focused eval case authoring guide with dataset contract, copy-ready case template, validation commands, and do/don't guidance so new cases can be added without runner code changes; linked guide from eval docs. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P5-4`/`P5-5`/`P5-6` (review hardening): Documented guard scope limitations for fabricated-metric detection, switched CI eval auth to create a real Grafana session cookie instead of a placeholder cookie, added clearer CI startup diagnostics (`config.yaml` precheck + assistant log tail on health timeout), centralized eval report schema version constants, and clarified JSON-only dataset parsing rationale in eval authoring docs. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P6-1`: Added runtime feature flags for routing/composite-tool/judge-gate/evidence-redaction with config/env overrides, manager + prompt/tool gating, eval quality-gate enforcement toggle, and coverage/docs updates. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P6-2`: Added model prompt profile configuration with explicit/default/family override resolution, model-family selection in startup, and profile-driven system/internal-tool prompt variants (`balanced`/`compact`/`strict`) with test coverage and config/deploy/wiki updates. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P6-3`: Added per-request token/tool/cost budget guardrails with prompt fitting, completion/tool/cost trip handling, degrade-to-final-answer fallback messaging, configurable budget env/yaml controls, and new budget telemetry metrics/docs/wiki updates. PR: `local`. Owner: `codex`.
- `2026-02-08` Phase planning `P7-*`: Added Phase 7 cloud-parity expansion tasks (Slack bridge, context insertion UX, dashboard actions, incident/on-call coverage, multi-agent scaffold) and explicitly deferred Tempo/TraceQL work. PR: `local`. Owner: `codex`.
- `2026-02-08` Task `P7-2`: Added "@" context picker with backend `GET /api/context/search` endpoint backed by MCP tool dispatch (datasources, dashboards, metrics, labels), `[Selected Context]` block injection into user messages with audit trail, and frontend popover picker with category tabs, keyboard navigation, debounced search, removable entity chips, and ChatPanel "@" trigger integration; 33 new tests across Go and frontend suites. PR: `local`. Owner: `codex`.
