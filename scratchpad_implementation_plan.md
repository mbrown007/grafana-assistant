# Scratchpad Dashboard Feature – Implementation Plan

Date: 2026-02-01

## Goal
Create a temporary, per-session Grafana dashboard (“scratchpad”) in the **Assistant Scratchpads** folder. The assistant can update a panel with LLM-generated queries and return a link to the dashboard. Dashboards auto-expire via TTL cleanup. No changes to the external Grafana MCP server.

## Core Decisions
- Folder name: **Assistant Scratchpads**
- TTL: **7 days (configurable)**
- Isolation: **per session** (tags + session ID), no Grafana permission enforcement for now
- Integration: **backend-only**, no new MCP server

## High-Level Flow
1. User asks a question that can be answered with data.
2. Agent chooses **artifact vs scratchpad** based on complexity rules below.
3. If artifact: fetch data and render in-chat (charts/metrics/tables).
4. If scratchpad: call internal tool.
5. Backend finds or creates a scratchpad dashboard for the session.
6. Backend updates the top panel with the generated query + title.
7. Assistant replies with a link to `/grafana/d/<uid>` and a short confirmation.
8. Background job cleans up stale scratchpads via TTL.
9. If any Grafana API operation fails, assistant responds with a brief failure message and offers the in-chat artifact alternative when possible.

## Artifact vs Scratchpad Decision Rules
Default to **artifacts**. Use **scratchpad** only for complex, exploratory, or highly interactive use cases.

### Always use artifacts
- **Tables** (always): results can be rendered in-chat.
- **Single-metric charts** with a small number of series.
- **Short time windows** and quick spot checks.
- **Static summaries** or “reporting style” views.

### Prefer scratchpad
- **Multi-panel or multi-query** outputs that are too dense for a single artifact.
- **Long time ranges** requiring exploration or zooming (e.g., days/weeks).
- **High-cardinality breakdowns** where user will filter or slice further.
- **Complex overlays** (multiple queries, alert bands, thresholds, annotations).
- **User explicitly wants to “explore”, “drill down”, or “tweak”** the graph.

### Examples
**Artifact-worthy**
- “Show CPU usage for the last 30 minutes on host A.”
- “Top 5 services by error rate in the last hour (table).”
- “Compare request latency p50 vs p99 for api in the last 6 hours.”

**Scratchpad-worthy**
- “Show CPU usage for the last 6 hours, broken down by host.”
- “Overlay deploy annotations and alert thresholds on latency.”
- “Graph several related PromQL queries and let me explore the time range.”
- “Build a dashboard panel for this query so I can tweak it.”

### Panel Type Selection (Phase 2)
- Default to timeseries.
- Use stat for single-value summaries (e.g., “current”, “total” with instant queries).
- Use table only if explicitly requested or if the query is topk/bottomk style.
- The agent may pass `panelType` to the scratchpad tool; backend also applies lightweight heuristics.

### Messaging Guidance
If choosing artifact, reply with: “Here’s a quick chart/table in the chat.”
If choosing scratchpad, reply with: “This is better as an interactive Grafana panel; I’ve put it in your scratchpad.”

## Data Model + Tags
- Dashboard title: `Scratchpad — <user name> — <short session id>`
- Folder: `Assistant Scratchpads`
- Tags:
  - `monitoring-assistant-scratchpad`
  - `user-id:<grafana user id>`
  - `session-id:<session id>`
  - `assistant-last-used:<unix>`

## Components to Add / Update

### 1) Grafana API Client Extensions
**File:** `internal/grafana/client.go`

Add methods:
- `SearchDashboards(tags []string) ([]DashboardHit, error)`
- `CreateDashboard(dashboard map[string]any, folderUID string, overwrite bool) (*DashboardSaveResponse, error)`
- `UpdateDashboard(uid string, dashboard map[string]any, folderUID string, overwrite bool) (*DashboardSaveResponse, error)`
- `DeleteDashboard(uid string) error`
- `GetFolderByTitle(title string) (*Folder, error)`
- `CreateFolder(title string) (*Folder, error)`

Tests in `internal/grafana/client_test.go` with `httptest`.

### 2) Scratchpad Template
**File:** `internal/dashboard/templates/scratchpad.json`

- Minimal dashboard with 1–3 panels
- Panel 1 is a general timeseries with empty targets
- **Stable, hardcoded panel ID** (e.g., `id: 1`) used as the update target (do not rely on visual position)

Embed via `//go:embed` in the new manager.

### 3) Scratchpad Manager
**New package:** `internal/dashboard`

Responsibilities:
- Ensure folder exists
- Find dashboard by session tag
- Create dashboard from template if missing
- Update top panel with query/title/description
- Update `assistant-last-used:<unix>` tag on each use

Key functions:
- `GetOrCreateScratchpad(ctx, user, sessionID) (uid string, panelID int, url string, err error)`
- `UpdatePanel(ctx, uid string, panelID int, query, title, description string, datasource map[string]any, timeRange map[string]string) error`
- `TouchLastUsed(ctx, uid string, ts time.Time) error`

### 4) Internal Tool Wiring (Agent)
**Files:** `internal/agent/manager.go`, `internal/agent/prompts.go`

- Add an internal tool (non-MCP): `scratchpad__upsert_panel`
- Tool args: `{query, title, description, datasource?, timeRange?}`
- Returns: `{dashboardUid, panelId, url}`
- Add tool handling before MCP routing or via a local registry.
- Update system prompt to instruct the LLM to call this tool when a graph is needed or the query is complex.

### 5) Session Storage
**Files:** `internal/storage/storage.go`, `internal/storage/sqlite.go`

- Add `ScratchpadUID` to `Session`
- Add `scratchpad_uid` column (migrate if missing)
- Persist UID on first scratchpad creation

### 6) TTL Cleanup Job
**Files:** `internal/config/config.go`, `cmd/assistant/main.go`

- Add config fields:
  - `ScratchpadTTLDays` (default 7)
  - `ScratchpadFolder` (default "Assistant Scratchpads")
- Add background goroutine to:
  - Search dashboards with `monitoring-assistant-scratchpad`
  - Parse `assistant-last-used:<unix>` tag
  - Delete if older than TTL

## Backend Responses
Assistant response should include:
- Short confirmation: “I’ve updated your scratchpad.”
- Link: `/grafana/d/<uid>` (Grafana handles slug redirect)

Frontend already renders markdown links, so no UI changes are required.

## Testing Plan
- Unit tests for Grafana client methods
- Unit tests for scratchpad manager logic (mock Grafana client)
- Optional integration test against test Grafana (if available)

## Rollout Notes
- Requires service-account Grafana token with dashboard write permissions
- Scratchpad dashboards are temporary; TTL cleanup keeps Grafana tidy
- No changes required to external Grafana MCP server

## Implementation Status (2026-02-01)
- Core backend implementation completed.
- Tests added for Grafana client and scratchpad manager.
- `go test ./...` passes.
