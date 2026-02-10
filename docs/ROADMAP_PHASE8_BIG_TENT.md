# Phase 8: Big Tent Architecture Alignment

Last updated: February 9, 2026
Source: Grafana "Building AI Into Observability Workflows" (Big Tent sessions)
Scope: Align monitoring assistant architecture with Grafana's production agent patterns — modular system prompts, domain-enriched NL formatting, mock eval sandbox, and coordinator+specialist sub-agents.

---

## How to use this document

This roadmap is designed for **independent implementation**. Each task includes:
- Exact file paths and line numbers for where to make changes
- Input/output contracts (function signatures, struct shapes)
- Test requirements with example assertions
- Acceptance criteria that can be verified without context from other tasks

Implementers should read the referenced source files before starting. Tasks within a sub-phase can often be parallelized across developers.

Status legend: `[ ]` Not started · `[~]` In progress · `[x]` Done · `[!]` Blocked
Owner legend: `JR` Junior · `MID` Mid-level · `SR` Senior (architecture/cross-cutting)

---

## Background: Current State vs Big Tent Patterns

| Pattern | Current State | Gap |
|---|---|---|
| **MCP tool discovery** | ✅ 33 tools via stdio, allowlist/denylist filtering | None — fully aligned |
| **Intent routing** | ✅ Rule-based classifier with 5 intent classes (`internal/agent/intent.go`) | None — fast, no-LLM approach matches Grafana's |
| **NL tool result formatting** | ✅ Summary-first with token budgets (`internal/mcp/formatter.go`) | Minor — no domain-specific enrichment |
| **System prompt** | ❌ Single monolithic prompt for all intents (`internal/agent/prompts.go:16-219`) | Major — same ~4-8k char prompt regardless of intent |
| **Sub-agent isolation** | ❌ Single LLM stream, all context in one conversation | Major — no isolated sub-agent calls |
| **Mock eval environment** | ❌ Evals require live Grafana stack | Major — not CI-reproducible |

---

## Phase summary

| Sub-phase | Goal | Owner | Effort |
|---|---|---|---|
| P8-1 | Modular intent-specific system prompts | SR + MID | 3-4 days |
| P8-2 | Domain-enriched NL tool result formatting | MID + JR | 2-3 days |
| P8-3 | Mock eval sandbox for reproducible testing | SR + MID | 4-5 days |
| P8-4 | Coordinator + specialist sub-agent scaffold | SR | 5-7 days |

---

## Tracking matrix

| Task | Owner | Status | Issue/Ticket | PR | Notes |
|---|---|---|---|---|---|
| P8-1a | SR | `[x]` | LOCAL-P8-1a | — | Intent-specific prompt builder refactor |
| P8-1b | MID | `[x]` | LOCAL-P8-1b | local | Intent-filtered MCP tool selection before prompt/schema exposure |
| P8-1c | MID | `[x]` | LOCAL-P8-1c | local | Intent-labeled prompt/tool-count histograms emitted and documented |
| P8-1d | JR | `[x]` | LOCAL-P8-1d | local | Added full 5-intent section matrix + size regression tests for prompt builder |
| P8-2a | MID | `[x]` | LOCAL-P8-2a | local | Added alert domain summary path with severity/state counts, oldest firing duration, and common label grouping |
| P8-2b | MID | `[x]` | LOCAL-P8-2b | local | Added Prometheus domain summary path with series/metric/value/cardinality enrichment |
| P8-2c | JR | `[x]` | LOCAL-P8-2c | local | Added Loki domain summary path with log-line counts, level distribution, service grouping, and time span |
| P8-2d | JR | `[x]` | LOCAL-P8-2d | local | Added fixture-driven domain tests + fallback/empty-case coverage for formatter paths |
| P8-3a | SR | `[x]` | LOCAL-P8-3a | local | Mock MCP server with fixture replay |
| P8-3b | MID | `[x]` | LOCAL-P8-3b | local | Fixture recording middleware + capture script for live runs |
| P8-3c | MID | `[x]` | LOCAL-P8-3c | local | Added eval-baseline --mock + mock config + make eval-baseline-mock |
| P8-3d | JR | `[x]` | LOCAL-P8-3d | local | Added PR mock-eval CI job + scheduled/manual live-stack job split |
| P8-4a | SR | `[x]` | LOCAL-P8-4a | local | Sub-agent interface + coordinator decision/delegation scaffold behind feature flag |
| P8-4b | SR | `[x]` | LOCAL-P8-4b | local | Added focused dashboard specialist constructor + coordinator heuristic delegation |
| P8-4c | SR | `[ ]` | LOCAL-P8-4c | TBD | Investigation specialist sub-agent |
| P8-4d | MID | `[ ]` | LOCAL-P8-4d | TBD | Sub-agent observability and audit |
| P8-4e | JR | `[ ]` | LOCAL-P8-4e | TBD | Sub-agent integration tests |

---

## P8-1: Modular Intent-Specific System Prompts

Phase status: `[x]`

**Goal**: Replace the single monolithic system prompt with intent-specific variants that include only the sections relevant to each intent class. This reduces prompt tokens by 30-50% per request and improves LLM focus.

**Rationale** (from Big Tent talk): "A single massive prompt (~10k tokens) makes debugging impossible — fixing one flow often breaks ten others" [22:19]. Modular prompts keep each intent's context clean.

### Current architecture

`SystemPrompt()` in `internal/agent/prompts.go:16` builds one prompt for all requests containing:
- Role preamble (~200 chars)
- Model profile section (~100 chars, conditional)
- Capability description (~300 chars)
- Artifact system docs (~1500 chars) — **always included even for simple metric queries**
- Guidelines (~600 chars)
- Scratchpad tool docs (~200 chars) — **irrelevant for docs/help intent**
- Explore tool docs (~150 chars) — **irrelevant for incident summary intent**
- Composite investigation tool docs (~300 chars, conditional)
- Security rules (~300 chars)
- Tool policy (~200 chars)
- Dashboard context (variable, 0-2000 chars)
- Datasource list (~150 chars)
- Tool summary (~500 chars)

**Total**: ~4000-8000 chars depending on dashboard context. Every request pays the full cost.

### Target architecture

```
SystemPrompt(intent, dashCtx, reqCtx, tools, flags, profile) string
  ├─ corePromptBlock()           // Always: role, security, model profile (~600 chars)
  ├─ intentPromptBlock(intent)   // Intent-specific capabilities and guidelines
  ├─ toolPolicyBlock(tools)      // Tool policy + summary (filtered by intent)
  ├─ dashboardContextBlock()     // Only when dashboard context exists
  └─ artifactBlock(intent)       // Only for intents that produce visualizations
```

### Tasks

---

#### `P8-1a` Refactor SystemPrompt into composable prompt builder

Owner: `SR`
Files to modify:
- `internal/agent/prompts.go` — refactor `SystemPrompt()` function
- `internal/agent/prompt_sections.go` — **new file** for section builders

**What to build:**

1. Create `internal/agent/prompt_sections.go` with these exported functions:

```go
// PromptContext holds all inputs needed to build a system prompt.
type PromptContext struct {
    Intent             IntentClass
    DashboardSummary   *appcontext.DashboardSummary
    DashboardContext    *api.DashboardContext
    Tools              []mcp.Tool
    CompositeToolMode  bool
    Profile            PromptProfile
}

// BuildSystemPrompt assembles an intent-aware system prompt from sections.
func BuildSystemPrompt(pc PromptContext) string

// Section builders (unexported, testable via BuildSystemPrompt).
func coreRoleBlock(profile PromptProfile) string
func securityRulesBlock() string
func artifactSystemBlock() string
func toolCapabilityBlock(hasTools bool) string
func toolPolicyBlock(tools []mcp.Tool) string
func compositeToolBlock() string
func scratchpadBlock() string
func exploreBlock() string
func guidelinesBlock(intent IntentClass) string
func dashboardContextBlock(dashCtx *appcontext.DashboardSummary, reqCtx *api.DashboardContext) string
func toolSummaryBlock(tools []mcp.Tool) string
```

2. Define intent → section mapping:

| Section | live_data | query_help | dashboard_lookup | how_to_docs | incident_summary |
|---|---|---|---|---|---|
| coreRoleBlock | ✅ | ✅ | ✅ | ✅ | ✅ |
| securityRulesBlock | ✅ | ✅ | ✅ | ✅ | ✅ |
| toolCapabilityBlock | ✅ | ✅ | ✅ | ❌ | ✅ |
| artifactSystemBlock | ✅ | ❌ | ❌ | ❌ | ✅ |
| guidelinesBlock | ✅ (data) | ✅ (query) | ✅ (search) | ✅ (docs) | ✅ (investigation) |
| compositeToolBlock | ❌ | ❌ | ❌ | ❌ | ✅ |
| scratchpadBlock | ✅ | ❌ | ❌ | ❌ | ✅ |
| exploreBlock | ✅ | ✅ | ❌ | ❌ | ❌ |
| dashboardContextBlock | ✅ | ✅ | ✅ | ❌ | ✅ |
| toolPolicyBlock | ✅ | ✅ | ✅ | ❌ | ✅ |
| toolSummaryBlock | ✅ | ✅ | ✅ | ❌ | ✅ |

3. Write intent-specific `guidelinesBlock()` variants:

- **live_data**: "Be concise. Fetch data with tools before answering. Present results as artifacts. Reference panel names and time ranges."
- **query_help**: "Focus on query construction. Explain PromQL/LogQL syntax. Show corrected queries. Use schema context to validate metric/label names."
- **dashboard_lookup**: "Help find dashboards by name, tag, or content. Use search tools. Return dashboard UIDs and folder paths. Don't fabricate dashboard names."
- **how_to_docs**: "Answer from documentation and knowledge base. Cite sources. Don't fabricate procedures. If docs don't cover the topic, say so."
- **incident_summary**: "Investigate systematically. Use composite investigation tool for multi-step flows. Summarize findings with evidence. Build timeline."

4. Update `SystemPrompt()` to delegate to `BuildSystemPrompt()`:

```go
// SystemPrompt wraps BuildSystemPrompt for backward compatibility.
func SystemPrompt(dashCtx *appcontext.DashboardSummary, reqCtx *api.DashboardContext, tools []mcp.Tool, compositeToolMode bool, profile PromptProfile) string {
    return BuildSystemPrompt(PromptContext{
        Intent:            IntentLiveData, // default for non-routed callers
        DashboardSummary:  dashCtx,
        DashboardContext:  reqCtx,
        Tools:             tools,
        CompositeToolMode: compositeToolMode,
        Profile:           profile,
    })
}
```

5. Update `HandleChat()` in `internal/agent/manager.go:212` to pass intent:

Change:
```go
systemPrompt := SystemPrompt(dashCtx, req.DashboardContext, m.tools, m.compositeToolModeEnabled(), m.promptProfile)
```
To:
```go
systemPrompt := BuildSystemPrompt(PromptContext{
    Intent:            intent.Label,
    DashboardSummary:  dashCtx,
    DashboardContext:  req.DashboardContext,
    Tools:             m.tools,
    CompositeToolMode: m.compositeToolModeEnabled(),
    Profile:           m.promptProfile,
})
```

**Done when**: `BuildSystemPrompt()` produces different prompts for each intent class. `how_to_docs` prompt should be ~40% smaller than `live_data` prompt (no artifact docs, no tool policy, no scratchpad/explore).

**Tests required**: See P8-1d.

---

#### `P8-1b` Intent-aware tool set selection

Owner: `MID`
Files to modify:
- `internal/agent/tools.go` — add intent-based tool filtering
- `internal/agent/manager.go:466-471` — wire intent filter before OpenAI tool conversion

**What to build:**

Currently `orderMCPToolsForIntent()` (in `internal/agent/tools.go`) reorders tools but sends ALL tools to the LLM regardless of intent. For `how_to_docs` intent, the model receives 33 MCP tools it will never use.

1. Add `filterToolsForIntent()` function in `internal/agent/tools.go`:

```go
// filterToolsForIntent returns only the tools relevant to the given intent.
// This reduces the tool schema tokens sent to the LLM.
func filterToolsForIntent(tools []mcp.Tool, intent IntentClass) []mcp.Tool {
    switch intent {
    case IntentHowToDocs:
        // Docs intent only needs KB tools (if any). No Grafana/Prometheus/Alertmanager tools.
        return filterByPrefix(tools, "kb__")
    case IntentDashboardLookup:
        // Dashboard lookup needs search + dashboard tools only.
        return filterByShortNames(tools,
            "search_dashboards", "get_dashboard_by_uid", "get_dashboard_summary",
            "get_dashboard_panel_queries", "list_datasources",
        )
    case IntentQueryHelp:
        // Query help needs schema + query tools.
        return filterByShortNames(tools,
            "query_prometheus", "query_loki_logs",
            "list_prometheus_metric_names", "list_prometheus_label_names",
            "list_prometheus_label_values", "list_datasources",
            "search_dashboards", "get_dashboard_panel_queries",
        )
    default:
        // live_data, incident_summary: return all tools
        return tools
    }
}
```

2. Update `manager.go:466-471`:

```go
orderedMCPTools := m.tools
if m.routingModeEnabled() {
    orderedMCPTools = filterToolsForIntent(m.tools, intent.Label)
    orderedMCPTools = orderMCPToolsForIntent(orderedMCPTools, intent.Label)
}
```

3. Add metric for filtered tool count vs total count:

```go
metrics.PromptToolCountFiltered.Observe(float64(len(orderedMCPTools)))
```

Add `PromptToolCountFiltered` histogram in `internal/metrics/metrics.go`.

**Done when**: `how_to_docs` intent sends 0-3 tools to the LLM (only KB tools). `dashboard_lookup` sends 5-6 tools. Existing `live_data` and `incident_summary` behavior unchanged.

**Tests required**:
- `filterToolsForIntent(allTools, IntentHowToDocs)` returns only `kb__*` tools
- `filterToolsForIntent(allTools, IntentDashboardLookup)` returns only search/dashboard tools
- `filterToolsForIntent(allTools, IntentLiveData)` returns all tools unchanged

---

#### `P8-1c` Prompt token budget metrics by intent

Owner: `MID`
Files to modify:
- `internal/metrics/metrics.go` — add intent-labeled prompt size metrics
- `internal/agent/manager.go` — emit metrics with intent label

**What to build:**

1. Add to `internal/metrics/metrics.go`:

```go
var PromptCharsByIntent = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "assistant_prompt_chars_by_intent",
        Help:    "System prompt character count by intent class",
        Buckets: []float64{500, 1000, 2000, 3000, 4000, 6000, 8000, 10000},
    },
    []string{"intent"},
)

var PromptToolCountByIntent = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "assistant_prompt_tool_count_by_intent",
        Help:    "Number of tools exposed to LLM by intent class",
        Buckets: []float64{0, 2, 5, 10, 15, 20, 30, 40},
    },
    []string{"intent"},
)
```

2. Update `manager.go` (~line 472) to emit intent-labeled metrics:

```go
metrics.PromptCharsByIntent.WithLabelValues(string(intent.Label)).Observe(float64(len(systemPrompt)))
metrics.PromptToolCountByIntent.WithLabelValues(string(intent.Label)).Observe(float64(len(openaiTools)))
```

3. Update `docs/OBSERVABILITY.md` with new metric descriptions.

**Done when**: Grafana dashboard can show prompt size breakdown by intent class. `how_to_docs` should show consistently smaller prompts than `incident_summary`.

---

#### `P8-1d` Prompt builder tests and intent coverage

Owner: `JR`
Files to modify:
- `internal/agent/prompt_sections_test.go` — **new file**
- `internal/agent/prompts_test.go` — update existing tests

**Tests to write:**

1. **Section inclusion by intent** — for each of the 5 intents, verify which sections are present/absent:

```go
func TestBuildSystemPrompt_HowToDocsOmitsArtifacts(t *testing.T) {
    prompt := BuildSystemPrompt(PromptContext{Intent: IntentHowToDocs, Tools: sampleTools})
    if strings.Contains(prompt, "## Artifact System") {
        t.Fatal("how_to_docs should not include artifact docs")
    }
    if strings.Contains(prompt, "## Scratchpad Tool") {
        t.Fatal("how_to_docs should not include scratchpad docs")
    }
    if strings.Contains(prompt, "## Tool Policy") {
        t.Fatal("how_to_docs should not include tool policy")
    }
}

func TestBuildSystemPrompt_LiveDataIncludesArtifacts(t *testing.T) {
    prompt := BuildSystemPrompt(PromptContext{Intent: IntentLiveData, Tools: sampleTools})
    if !strings.Contains(prompt, "## Artifact System") {
        t.Fatal("live_data should include artifact docs")
    }
}
```

2. **Prompt size reduction** — verify `how_to_docs` is at least 30% smaller than `live_data`:

```go
func TestBuildSystemPrompt_HowToDocsSmallerThanLiveData(t *testing.T) {
    live := BuildSystemPrompt(PromptContext{Intent: IntentLiveData, Tools: sampleTools})
    docs := BuildSystemPrompt(PromptContext{Intent: IntentHowToDocs})
    ratio := float64(len(docs)) / float64(len(live))
    if ratio > 0.70 {
        t.Fatalf("how_to_docs prompt should be ≤70%% of live_data; got %.0f%% (%d vs %d chars)", ratio*100, len(docs), len(live))
    }
}
```

3. **Backward compatibility** — verify `SystemPrompt()` (old signature) still works and produces a valid prompt.

4. **Intent-specific guidelines** — verify each intent gets its own guidelines text:

```go
func TestBuildSystemPrompt_IntentGuidelines(t *testing.T) {
    cases := map[IntentClass]string{
        IntentLiveData:        "Fetch data with tools",
        IntentQueryHelp:       "query construction",
        IntentDashboardLookup: "find dashboards",
        IntentHowToDocs:       "documentation",
        IntentIncidentSummary: "Investigate systematically",
    }
    for intent, expected := range cases {
        prompt := BuildSystemPrompt(PromptContext{Intent: intent, Tools: sampleTools})
        if !strings.Contains(prompt, expected) {
            t.Errorf("intent %s: expected guideline containing %q", intent, expected)
        }
    }
}
```

**Done when**: All 5 intent paths have test coverage. Prompt size assertions prevent regressions.

---

## P8-2: Domain-Enriched NL Tool Result Formatting

Phase status: `[x]`

**Goal**: Enhance the existing NL formatter with domain-specific intelligence so the LLM receives semantically rich summaries instead of generic "Tool returned object (status=ok, alerts=5)".

**Rationale** (from Big Tent talk): "LLMs actually parse natural language descriptions of data more accurately than they parse complex, nested JSON objects" [20:28]. Domain enrichment turns `alerts=5` into `5 alerts: 3 critical (firing >1h), 2 warning (new)`.

### Current architecture

`FormatToolResult()` in `internal/mcp/formatter.go:19` produces generic summaries:
- `summarizeObject()` extracts `status`, `action`, `tool`, `message`, and collection counts
- `summarizeArray()` counts items and reports type distribution
- No awareness of what kind of data is being summarized (alerts vs metrics vs dashboards)

### Target architecture

```
FormatToolResult(result any) string                    // existing entry point
  ├─ normalizeToolResultValue()                         // existing
  ├─ tryDomainSummary(normalized) (string, bool)        // NEW: domain-specific
  │   ├─ summarizeAlertResults(obj) string              // alert severity, duration
  │   ├─ summarizePrometheusResults(obj) string         // metric names, value ranges
  │   ├─ summarizeLokiResults(obj) string               // log levels, error counts
  │   └─ summarizeDashboardResults(obj) string          // panel counts, datasources
  └─ buildToolResultSummary(normalized) string          // existing generic fallback
```

### Tasks

---

#### `P8-2a` Alert domain enrichment

Owner: `MID`
Files to modify:
- `internal/mcp/formatter.go` — add alert-specific summarizer
- `internal/mcp/domain_formatters.go` — **new file** for domain logic

**What to build:**

1. Create `internal/mcp/domain_formatters.go`:

```go
package mcp

// tryDomainSummary attempts to produce a domain-enriched summary.
// Returns ("", false) if the data doesn't match a known domain pattern.
func tryDomainSummary(value any) (string, bool)
```

2. Alert detection pattern — look for known alert response shapes from Grafana MCP and Alertmanager MCP:

```go
func isAlertResult(obj map[string]any) bool {
    // Grafana MCP: {"alerts": [...]} or items with "state"/"severity"/"labels"
    // Alertmanager MCP: {"status": "success", "data": {"alerts": [...]}}
    _, hasAlerts := obj["alerts"]
    _, hasState := obj["state"]
    return hasAlerts || hasState
}

func summarizeAlertResults(obj map[string]any) string {
    // Extract alerts array (try obj["alerts"], obj["data"]["alerts"])
    // Count by state: firing, pending, resolved
    // Count by severity: critical, warning, info
    // Report duration for firing alerts (if "startsAt" present)
    // Example output:
    //   "5 alerts: 2 critical (firing, oldest 3h12m), 2 warning (firing), 1 info (resolved).
    //    Labels: job=api-server (3), job=worker (2). Datasource: alertmanager."
}
```

3. Wire into `FormatToolResult()` — insert domain check before generic fallback:

```go
func FormatToolResult(result any) string {
    if result == nil {
        return "Summary: No result returned."
    }
    normalized := normalizeToolResultValue(result)

    // Try domain-specific summary first.
    if domainSummary, ok := tryDomainSummary(normalized); ok {
        details := formatToolResultDetails(normalized)
        if details == "" {
            return "Summary: " + domainSummary
        }
        return "Summary: " + domainSummary + "\nMachine details:\n" + details
    }

    // Existing generic path.
    summary := buildToolResultSummary(normalized)
    // ...
}
```

**Done when**: Alert tool results produce enriched summaries with severity counts, firing durations, and common label groups. Generic results unchanged.

---

#### `P8-2b` Prometheus query result enrichment

Owner: `MID`
Files to modify: `internal/mcp/domain_formatters.go`

**What to build:**

Prometheus query results from `grafana__query_prometheus` typically return:

```json
{
  "status": "success",
  "data": {
    "resultType": "matrix"|"vector"|"scalar",
    "result": [
      {"metric": {"__name__": "up", "job": "api"}, "values": [[1234, "1"], ...]}
    ]
  }
}
```

1. Detection:

```go
func isPrometheusResult(obj map[string]any) bool {
    data, ok := obj["data"].(map[string]any)
    if !ok { return false }
    _, hasResultType := data["resultType"]
    return hasResultType
}
```

2. Enrichment:

```go
func summarizePrometheusResults(obj map[string]any) string {
    // Extract result type (matrix/vector/scalar)
    // Count number of series
    // Report metric names (unique __name__ values)
    // Report value ranges (min/max/latest) for numeric results
    // Report label cardinality (unique label key counts)
    // Example output:
    //   "Prometheus matrix query returned 3 series for metric 'node_cpu_seconds_total'.
    //    Labels: instance (3 unique), mode (8 unique). Latest values: 0.02-0.98.
    //    Time range: 1h (240 samples per series)."
}
```

**Done when**: Prometheus query results show metric names, series counts, value ranges, and label cardinality. LLM can reason about the data shape without parsing JSON.

---

#### `P8-2c` Loki/log result enrichment

Owner: `JR`
Files to modify: `internal/mcp/domain_formatters.go`

**What to build:**

Loki results from `grafana__query_loki_logs` return log streams:

```json
{
  "status": "success",
  "data": {
    "resultType": "streams",
    "result": [
      {"stream": {"app": "api", "level": "error"}, "values": [["1234", "error message"]]}
    ]
  }
}
```

1. Enrichment:

```go
func summarizeLokiResults(obj map[string]any) string {
    // Count total log lines across all streams
    // Count streams
    // Extract log levels (if "level" label present): error, warn, info, debug
    // Report app/service labels
    // Show first/last timestamp range
    // Example output:
    //   "Loki query returned 47 log lines across 3 streams.
    //    Log levels: error (12), warn (8), info (27). Apps: api-server, worker.
    //    Time span: 14:02:11 — 14:47:33 (45m)."
}
```

**Done when**: Loki results show log level distribution, stream counts, and time span. Follow the same pattern as P8-2b.

---

#### `P8-2d` Domain formatter tests

Owner: `JR`
Files to modify:
- `internal/mcp/domain_formatters_test.go` — **new file**
- `internal/mcp/formatter_test.go` — add integration tests

**Tests to write:**

1. **Alert enrichment**: Feed sample Grafana alert JSON → verify severity counts in output
2. **Alert enrichment (Alertmanager)**: Feed sample Alertmanager JSON → same verification
3. **Prometheus matrix**: Feed sample matrix result → verify series count, metric name, value range
4. **Prometheus vector**: Feed sample instant query result → verify
5. **Loki streams**: Feed sample log result → verify line count, level distribution
6. **Non-domain data falls through**: Feed generic JSON → verify generic summary (unchanged behavior)
7. **Empty/nil cases**: Verify no panics, graceful fallback

Each test should use realistic JSON fixtures. Create `internal/mcp/testdata/` directory with sample responses recorded from the dev environment.

**Done when**: 100% coverage of domain detection and enrichment paths. Regression tests confirm generic path is unchanged.

---

## P8-3: Mock Eval Sandbox for Reproducible Testing

Phase status: `[x]`

**Goal**: Create a mock MCP server that replays recorded tool responses so evals can run without a live Grafana/Prometheus/Alertmanager stack. This makes evals CI-reproducible and deterministic.

**Rationale** (from Big Tent talk): "Control the environment, not the LLM. Mock the environment (the Grafana API responses). This creates a controlled sandbox where you can run reproducible end-to-end tests" [21:02].

### Current architecture

- Eval runner (`scripts/eval_baseline.sh`) requires a running assistant + live Docker stack
- CI workflow (`.github/workflows/eval-quality-gate.yml`) starts full Docker stack before running evals
- Each eval case hits real MCP tools → non-deterministic results (metrics change over time)
- Cannot run evals on a laptop without Docker

### Target architecture

```
                                 ┌─────────────────────┐
                                 │  Mock MCP Server     │
  eval_baseline.sh ──────────►   │  (stdio transport)   │
       │                         │                      │
       ▼                         │  Reads fixtures from │
  assistant binary               │  tests/evals/        │
  (config: mock MCP)             │  fixtures/           │
                                 └─────────────────────┘

  Fixture format:
  tests/evals/fixtures/
    grafana__query_prometheus/
      case_01_cpu_usage.json     ← recorded response
      case_02_memory.json
    grafana__search_dashboards/
      case_01_node_exporter.json
    alertmanager__list_alerts/
      case_01_firing.json
```

### Tasks

---

#### `P8-3a` Mock MCP server with fixture replay

Owner: `SR`
Files to create:
- `cmd/mock-mcp/main.go` — **new** mock MCP server binary
- `internal/mockserver/server.go` — **new** mock server logic
- `internal/mockserver/matcher.go` — **new** request→fixture matching

**What to build:**

1. A stdio-transport MCP server that:
   - Reads fixture files from a configurable directory
   - Responds to `tools/list` with tool definitions extracted from fixture filenames
   - Responds to `tools/call` by matching the tool name + arguments to a fixture
   - Returns the fixture content as the tool result

2. Fixture file format (JSON):

```json
{
  "tool_name": "grafana__query_prometheus",
  "match": {
    "datasourceUid": "prometheus",
    "expr": ".*node_cpu.*"
  },
  "response": {
    "status": "success",
    "data": {
      "resultType": "matrix",
      "result": [...]
    }
  }
}
```

3. Matching strategy:
   - Exact match on `tool_name`
   - Regex match on argument values (allows one fixture to cover multiple queries)
   - If no match, return a generic "no data" response (not an error — simulates empty result)
   - Fixtures are loaded at startup, not per-request

4. Tool discovery:
   - Scan fixture directory for unique tool names
   - Generate minimal tool schemas from fixture filenames
   - Alternative: accept a `tools.json` file that defines the full tool list

5. Build as `cmd/mock-mcp/main.go`:

```go
func main() {
    fixtureDir := flag.String("fixtures", "tests/evals/fixtures", "fixture directory")
    flag.Parse()
    server := mockserver.New(*fixtureDir)
    // Run as stdio MCP server (same protocol as real servers)
    server.ServeStdio()
}
```

6. Add to Makefile:

```makefile
mock-mcp-build:
	go build -o bin/mock-mcp ./cmd/mock-mcp
```

**Done when**: `bin/mock-mcp -fixtures tests/evals/fixtures` responds to tool calls with fixture data over stdio. Can be used as a drop-in replacement for real MCP servers in config.yaml.

---

#### `P8-3b` Fixture recording from live environment

Owner: `MID`
Files to create:
- `scripts/record_eval_fixtures.sh` — **new** recording script
- `internal/mockserver/recorder.go` — **new** optional recording middleware

**What to build:**

1. Add a recording mode to the agent that captures tool call/response pairs:

```go
// recorder.go wraps an MCP client and records all tool calls.
type RecordingClient struct {
    inner     mcp.Client
    outputDir string
    mu        sync.Mutex
    counter   int
}

func (r *RecordingClient) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
    result, err := r.inner.CallTool(ctx, name, args)
    if err == nil {
        r.saveFixture(name, args, result)
    }
    return result, err
}
```

2. Config flag to enable recording:

```yaml
# In config.yaml (dev only):
eval_fixture_record_dir: "tests/evals/fixtures"
```

3. Recording script:

```bash
#!/bin/bash
# Run the eval dataset against a live environment and record all tool responses.
# Usage: ./scripts/record_eval_fixtures.sh
export ASSISTANT_EVAL_FIXTURE_RECORD_DIR=tests/evals/fixtures
./scripts/eval_baseline.sh
echo "Fixtures recorded to tests/evals/fixtures/"
```

4. Fixture file naming convention:

```
tests/evals/fixtures/
  {tool_name}/
    {case_id}_{hash}.json
```

Where `{hash}` is a short hash of the arguments for deduplication.

**Done when**: Running the eval dataset against a live environment produces a fixture directory that can be replayed by the mock server.

---

#### `P8-3c` Eval runner mock-mode integration

Owner: `MID`
Files to modify:
- `scripts/eval_baseline.sh` — add `--mock` flag
- `tests/evals/config.mock.yaml` — **new** config for mock mode

**What to build:**

1. Create `tests/evals/config.mock.yaml`:

```yaml
listen_addr: ":18081"
grafana_url: "http://localhost:13000"  # not used in mock mode but required
db_path: ":memory:"
audit_log_path: "/dev/null"
openai_model: "gpt-4o"

mcp_servers:
  - type: "grafana"
    transport: "stdio"
    command: "./bin/mock-mcp"
    args: ["-fixtures", "tests/evals/fixtures/grafana"]

  - type: "alertmanager"
    transport: "stdio"
    command: "./bin/mock-mcp"
    args: ["-fixtures", "tests/evals/fixtures/alertmanager"]

  - type: "kb"
    transport: "stdio"
    command: "./bin/mcp-kb"
    args: ["-transport", "stdio"]
    env:
      KB_PATH: "KB"
```

2. Update `scripts/eval_baseline.sh` to accept `--mock`:

```bash
if [ "$1" = "--mock" ]; then
    CONFIG="tests/evals/config.mock.yaml"
    # Build mock-mcp if needed
    make mock-mcp-build 2>/dev/null
    # Don't require Docker
    SKIP_DOCKER=true
fi
```

3. Add Makefile target:

```makefile
eval-baseline-mock: mock-mcp-build build  ## Run evals against recorded fixtures (no Docker needed)
	./scripts/eval_baseline.sh --mock
```

**Done when**: `make eval-baseline-mock` runs the full eval dataset using recorded fixtures. No Docker required. Results should be deterministic across runs (same fixtures → same tool results → LLM may vary but tool context is stable).

---

#### `P8-3d` CI workflow for mock-based evals

Owner: `JR`
Files to modify:
- `.github/workflows/eval-quality-gate.yml` — add mock-based job

**What to build:**

1. Add a new job to the existing workflow that runs mock-based evals:

```yaml
eval-mock:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      # No Docker setup needed!
      - name: Build binaries
        run: make build mock-mcp-build
      - name: Run mock-based eval
        env:
          ASSISTANT_OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: make eval-baseline-mock
      - name: Run judge
        run: make eval-judge RUN_ARTIFACT=tests/evals/results/latest-run.json
      - name: Run guard
        run: make eval-guard RUN_ARTIFACT=tests/evals/results/latest-run.json
      - name: Quality gate
        run: make eval-quality-gate ...
```

2. This job should run **without Docker** — much faster CI (~2min vs ~5min).

3. Keep the existing Docker-based eval job as a separate, less-frequent check (e.g., nightly or on release branches).

**Done when**: CI runs mock-based evals on every PR without requiring Docker. Quality gate still enforces thresholds.

---

## P8-4: Coordinator + Specialist Sub-Agent Scaffold

Phase status: `[~]`

**Goal**: Implement the coordinator pattern where complex requests are delegated to specialized sub-agents with isolated LLM calls and focused prompts. This is the most architecturally significant change.

**Rationale** (from Big Tent talk): "A Coordinator Agent acts as a router. It delegates specific tasks to Expert sub-agents. Keeps conversation history clean. The Support Agent's technical doc searches don't clutter the Coordinator's context window" [23:14].

### Current architecture

`HandleChat()` in `manager.go:186` runs everything in a single LLM stream:
1. Intent classification (rule-based, no LLM) ← good
2. Context injection (schema, dashboard, KB) ← good
3. Single system prompt with all capabilities ← problem
4. Single tool loop handling all tool calls ← problem
5. All tool results feed back into same conversation ← problem

### Target architecture

```
HandleChat()
  ├─ ClassifyIntent()                    // existing, no change
  ├─ Coordinator decides:
  │   ├─ Simple query (confidence > 0.9) → Direct LLM call (existing path)
  │   └─ Complex/multi-domain query → Delegate to sub-agent(s)
  │
  ├─ SubAgent: DashboardSpecialist
  │   ├─ Isolated LLM call with compact prompt
  │   ├─ Only dashboard/search tools exposed
  │   ├─ Returns structured result (UID, panels, summary)
  │   └─ Context does NOT leak back to coordinator
  │
  ├─ SubAgent: InvestigationSpecialist
  │   ├─ Isolated LLM call with investigation prompt
  │   ├─ All query/alert tools exposed
  │   ├─ Uses composite investigation tool
  │   └─ Returns findings summary
  │
  └─ Coordinator synthesizes sub-agent results into final response
```

### Important design constraints

1. **Sub-agents are NOT separate processes** — they are separate LLM call chains within the same Go process using the same `llm.Client`.
2. **Sub-agents share the MCP tool pool** — they call the same MCP clients but with filtered tool sets.
3. **Sub-agents have isolated memory** — each gets a fresh `Memory` with only their system prompt and the user query. They don't see the coordinator's conversation history.
4. **The coordinator is the existing HandleChat path** — not a new LLM call. It routes based on intent classification (rule-based) and synthesizes results.
5. **Feature flag**: `sub_agent_mode` in `FeatureFlags`. When disabled, falls back to current single-stream behavior.

### Tasks

---

#### `P8-4a` Sub-agent interface and coordinator loop

Owner: `SR`
Files to create:
- `internal/agent/subagent.go` — **new** sub-agent interface and executor
- `internal/agent/coordinator.go` — **new** coordination logic

Files to modify:
- `internal/agent/manager.go` — wire coordinator into HandleChat
- `internal/config/config.go` — add `sub_agent_mode` feature flag

**What to build:**

1. Sub-agent interface:

```go
// SubAgentResult is the structured output of a sub-agent execution.
type SubAgentResult struct {
    Summary      string            // NL summary for the coordinator to use
    ToolsUsed    []string          // Tool names called by this sub-agent
    TokensUsed   int               // Prompt + completion tokens consumed
    Artifacts    []any             // Any artifacts produced (charts, tables)
    Metadata     map[string]string // Optional structured metadata
    Error        string            // Non-empty if sub-agent failed
}

// SubAgent executes an isolated LLM call chain for a specific domain.
type SubAgent struct {
    Name          string
    SystemPrompt  string
    Tools         []mcp.Tool        // Filtered tool set for this specialist
    MaxIterations int               // Tool loop limit (typically 3-5)
}

// Execute runs the sub-agent with an isolated LLM conversation.
// The streamFn receives tool call events for UI transparency.
func (sa *SubAgent) Execute(ctx context.Context, llmClient *llm.Client, mcpClients map[string]mcp.Client,
    userMessage string, streamFn func(api.StreamChunk)) SubAgentResult
```

2. Sub-agent executor — similar to the existing tool loop in `manager.go:475-750` but:
   - Creates fresh `Memory` with sub-agent's system prompt
   - Only exposes sub-agent's tool set
   - Runs for at most `MaxIterations` iterations
   - Collects the final response as `Summary`
   - Streams tool call events to the UI with a `sub_agent` prefix for identification

3. Coordinator logic:

```go
// CoordinatorDecision determines whether to use sub-agents or direct path.
type CoordinatorDecision struct {
    UseDirect   bool          // Use existing single-stream path
    SubAgents   []string      // Sub-agent names to invoke
    Reason      string        // Audit trail
}

func (m *Manager) coordinatorDecide(intent IntentResult, message string) CoordinatorDecision {
    // Simple, high-confidence intents → direct path
    if intent.Confidence >= 0.90 && intent.Label == IntentHowToDocs {
        return CoordinatorDecision{UseDirect: true, Reason: "high-confidence docs intent"}
    }
    if intent.Confidence >= 0.90 && intent.Label == IntentDashboardLookup {
        return CoordinatorDecision{
            SubAgents: []string{"dashboard"},
            Reason: "high-confidence dashboard lookup → delegate to specialist",
        }
    }
    if intent.Label == IntentIncidentSummary {
        return CoordinatorDecision{
            SubAgents: []string{"investigation"},
            Reason: "incident summary → delegate to investigation specialist",
        }
    }
    // Default: direct path (existing behavior)
    return CoordinatorDecision{UseDirect: true, Reason: "default direct path"}
}
```

4. Wire into `HandleChat()` — after intent classification (line 192), before system prompt (line 212):

```go
if m.subAgentModeEnabled() {
    decision := m.coordinatorDecide(intent, cleanMessage)
    if !decision.UseDirect {
        m.handleChatViaSubAgents(ctx, user, req, sess, intent, decision, streamFn)
        return
    }
}
// ... existing direct path continues
```

5. Feature flag in config:

```go
// In FeatureFlags struct:
SubAgentMode bool `yaml:"sub_agent_mode"`

// Env override:
// ASSISTANT_FEATURE_SUB_AGENT_MODE=true
```

**Done when**: Sub-agent interface exists and coordinator can decide between direct path and sub-agent delegation. Feature flag controls activation. Existing behavior unchanged when flag is off.

---

#### `P8-4b` Dashboard specialist sub-agent

Owner: `SR`
Files to create:
- `internal/agent/subagent_dashboard.go` — **new**

**What to build:**

1. Dashboard specialist with focused prompt:

```go
func (m *Manager) newDashboardSubAgent() *SubAgent {
    tools := filterToolsForIntent(m.tools, IntentDashboardLookup)
    return &SubAgent{
        Name: "dashboard",
        SystemPrompt: `You are a dashboard search specialist. Your ONLY job is to find the right Grafana dashboard.
Use search_dashboards to find dashboards by name, tag, or content.
Use get_dashboard_summary to verify a dashboard matches the user's need.
Return the dashboard UID, title, folder, and a one-sentence description of what it shows.
If you cannot find a matching dashboard, say so clearly.
Do NOT answer the user's question — only find the dashboard.`,
        Tools:         tools,
        MaxIterations: 3,
    }
}
```

2. The coordinator calls this sub-agent, then uses the result to:
   - Inject the found dashboard UID/context into the main conversation
   - Proceed with the direct path using the enriched context
   - OR return the dashboard info directly if that's all the user wanted

3. Response format:

```go
// Dashboard sub-agent Summary example:
// "Found dashboard 'Node Exporter Full' (uid=abc123, folder=Infrastructure).
//  Shows CPU, memory, disk, and network metrics for Linux hosts.
//  17 panels, primary datasource: prometheus."
```

**Done when**: Dashboard lookup requests are handled by an isolated LLM call with a ~200 char prompt + 5-6 tools instead of the full ~6000 char prompt + 33 tools.

---

#### `P8-4c` Investigation specialist sub-agent

Owner: `SR`
Files to create:
- `internal/agent/subagent_investigation.go` — **new**

**What to build:**

1. Investigation specialist:

```go
func (m *Manager) newInvestigationSubAgent(dashCtx *appcontext.DashboardSummary, schemaCtx string) *SubAgent {
    return &SubAgent{
        Name: "investigation",
        SystemPrompt: buildInvestigationPrompt(dashCtx, schemaCtx),
        Tools:         m.tools, // All tools available for investigation
        MaxIterations: 5,
    }
}

func buildInvestigationPrompt(dashCtx *appcontext.DashboardSummary, schemaCtx string) string {
    // Focused investigation prompt:
    // - Role: incident investigator
    // - Available context: dashboard panels, schema (metrics/labels)
    // - Instructions: fetch metrics, check alerts, query logs, build timeline
    // - Output format: structured findings with evidence
    // - NO artifact docs, NO scratchpad docs, NO explore docs
}
```

2. The coordinator streams the investigation sub-agent's tool calls to the UI (for transparency).

3. After the sub-agent returns, the coordinator either:
   - Returns the investigation summary directly (simple case)
   - Uses the findings as context for a final synthesis LLM call (complex case)

**Done when**: Incident summary requests are handled by a focused investigation sub-agent. The sub-agent's tool calls are visible in the UI. The coordinator synthesizes the result.

---

#### `P8-4d` Sub-agent observability and audit

Owner: `MID`
Files to modify:
- `internal/metrics/metrics.go` — add sub-agent metrics
- `internal/agent/subagent.go` — emit metrics and audit entries
- `docs/OBSERVABILITY.md` — document new metrics

**What to build:**

1. Metrics:

```go
var SubAgentInvocationsTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "assistant_subagent_invocations_total",
        Help: "Total sub-agent invocations by agent name",
    },
    []string{"agent_name"},
)

var SubAgentDurationSeconds = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "assistant_subagent_duration_seconds",
        Help:    "Sub-agent execution duration",
        Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30},
    },
    []string{"agent_name"},
)

var SubAgentTokensUsed = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "assistant_subagent_tokens_used",
        Help:    "Tokens consumed by sub-agent calls",
        Buckets: []float64{100, 500, 1000, 2000, 5000, 10000},
    },
    []string{"agent_name"},
)

var SubAgentErrorsTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "assistant_subagent_errors_total",
        Help: "Sub-agent errors by agent name",
    },
    []string{"agent_name"},
)
```

2. Audit entries — each sub-agent execution logs:
   - `event_type: "subagent_invocation"`
   - Agent name, user message, tools used, tokens consumed, duration
   - Whether the sub-agent succeeded or failed

3. Stream events — sub-agent tool calls appear in the UI stream with prefix:

```json
{"type": "tool_call", "subagent": "dashboard", "tool_name": "search_dashboards", ...}
```

**Done when**: Grafana dashboard shows sub-agent invocation rates, durations, token costs, and error rates. Audit trail includes sub-agent details.

---

#### `P8-4e` Sub-agent integration tests

Owner: `JR`
Files to create:
- `internal/agent/subagent_test.go` — **new**
- `internal/agent/coordinator_test.go` — **new**

**Tests to write:**

1. **Coordinator decision tests**:
   - High-confidence `dashboard_lookup` → delegates to dashboard sub-agent
   - `incident_summary` → delegates to investigation sub-agent
   - `how_to_docs` → direct path (no sub-agent)
   - `live_data` → direct path (default)
   - Low-confidence intent → direct path

2. **Sub-agent isolation tests**:
   - Sub-agent memory does not contain coordinator's conversation history
   - Sub-agent tool set is filtered (dashboard agent doesn't see alert tools)
   - Sub-agent respects MaxIterations limit

3. **Feature flag tests**:
   - `sub_agent_mode: false` → all requests use direct path
   - `sub_agent_mode: true` → coordinator decision is respected

4. **Mock LLM tests** — use mock `llm.Client` that returns canned responses to verify:
   - Sub-agent executes tool calls and collects results
   - SubAgentResult contains summary, tools used, tokens
   - Error handling when sub-agent LLM call fails

**Done when**: Coordinator routing logic has full test coverage. Sub-agent isolation is verified. Feature flag toggle works.

---

## Verification checklist

After all P8 tasks are complete:

1. `make test` — all Go tests pass
2. `make eval-baseline-mock` — mock-based evals run without Docker
3. Compare prompt sizes: `how_to_docs` should be ≤60% of `live_data` prompt
4. Compare tool counts: `how_to_docs` should expose 0-3 tools, `dashboard_lookup` 5-6
5. Domain formatter produces enriched alert/metric/log summaries
6. Sub-agent mode (when enabled) delegates dashboard lookups and incident investigations
7. Grafana shows intent-specific prompt size distributions and sub-agent metrics
8. All existing behavior preserved when feature flags are off

---

## Implementation order recommendation

```
Week 1:  P8-1a + P8-1b (parallel)  →  P8-1c + P8-1d (parallel)
Week 2:  P8-2a + P8-2b (parallel)  →  P8-2c + P8-2d (parallel)
Week 2:  P8-3a (can overlap with P8-2)
Week 3:  P8-3b + P8-3c (parallel)  →  P8-3d
Week 3-4: P8-4a → P8-4b + P8-4c (parallel) → P8-4d + P8-4e (parallel)
```

P8-1 should be done first — it's the foundation that P8-4 builds on (modular prompts become sub-agent prompts).
P8-2 and P8-3 are independent of each other and can run in parallel.
P8-4 depends on P8-1 (modular prompts) and benefits from P8-2 (better tool results for sub-agents).

---

## Changelog

Use this format:
- `YYYY-MM-DD` Task ID(s): short summary. PR: `#123` (or `local`). Owner: `name`.

Entries:

- `2026-02-09` Phase planning `P8-*`: Created Phase 8 Big Tent architecture alignment roadmap with 17 tasks across 4 sub-phases. PR: `local`. Owner: `marc`.
- `2026-02-10` `P8-4a`: Added sub-agent interface/executor, coordinator decision loop, and `sub_agent_mode` feature flag wiring with tests. PR: `local`. Owner: `marc`.
- `2026-02-10` `P8-4b`: Added dashboard specialist sub-agent module with focused prompt/tool set and coordinator delegation heuristics + tests. PR: `local`. Owner: `marc`.
