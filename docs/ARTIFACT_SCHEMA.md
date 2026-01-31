# Artifact and Streaming Schema

This document defines the canonical JSON contracts between the Go backend and the React frontend. These schemas are derived from the proven implementation in `grafana-chat-plugin` and must be treated as the single source of truth for both sides.

---

## Artifact Schema

Artifacts are rich data visualizations embedded in assistant responses. The LLM returns them as fenced `\`\`\`artifact ... \`\`\`` blocks within its text response. The frontend parses these blocks and renders the appropriate component.

### ArtifactData

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `"report" \| "chart" \| "table" \| "metric-cards" \| "raw"` | Yes | Determines which renderer to use |
| `title` | `string` | No | Display title |
| `subtitle` | `string` | No | Secondary title (reports) |
| `description` | `string` | No | Additional context |
| `chartType` | `"bar" \| "line" \| "pie" \| "area"` | When `type: "chart"` | Chart variant |
| `data` | `array<object>` | When `type: "chart"` | Chart data points (see Chart Data below) |
| `metrics` | `array<MetricCard>` | When `type: "metric-cards"` | Metric cards to display |
| `columns` | `array<TableColumn>` | When `type: "table"` | Table column definitions |
| `rows` | `array<object>` | When `type: "table"` | Table row data |
| `sections` | `array<ReportSection>` | When `type: "report"` | Composable report sections |

### MetricCard

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `label` | `string` | Yes | Metric name |
| `value` | `string \| number` | Yes | Display value |
| `change` | `number` | No | Percentage change (positive = up, negative = down) |
| `changeLabel` | `string` | No | Label for the change (e.g., "vs last week") |
| `icon` | `string` | No | Icon key: `users`, `activity`, `alert`, `success`, `clock`, `server`, `phone`, `message`, `trending_up`, `trending_down` |
| `color` | `"blue" \| "green" \| "red" \| "amber" \| "purple"` | No | Card accent color (default: `blue`) |

### TableColumn

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `key` | `string` | Yes | Maps to row object keys |
| `label` | `string` | Yes | Column header text |
| `align` | `"left" \| "center" \| "right"` | No | Text alignment (default: `left`) |

### ReportSection

Reports are composed of ordered sections. Each section has its own type and relevant fields.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `"header" \| "summary" \| "metrics" \| "chart" \| "table" \| "text"` | Yes | Section renderer |
| `title` | `string` | No | Section heading |
| `content` | `string` | No | Text content (for `header`, `summary`, `text`) |
| `data` | `array<object>` | No | Chart data (for `chart` sections) |
| `chartType` | `"bar" \| "line" \| "pie" \| "area"` | No | Chart variant (for `chart` sections) |
| `metrics` | `array<MetricCard>` | No | Metrics (for `metrics` sections) |
| `columns` | `array<TableColumn>` | No | Columns (for `table` sections) |
| `rows` | `array<object>` | No | Rows (for `table` sections) |

### Chart Data Format

Chart data is an array of objects. Each object must have a `name` (or `label`) key for the x-axis, and one or more numeric keys for the data series:

```json
[
  { "name": "Mon", "requests": 120, "errors": 3 },
  { "name": "Tue", "requests": 150, "errors": 5 },
  { "name": "Wed", "requests": 130, "errors": 2 }
]
```

For pie charts, use `name` and a single numeric key (typically `value`):

```json
[
  { "name": "200 OK", "value": 8500 },
  { "name": "404 Not Found", "value": 230 },
  { "name": "500 Error", "value": 45 }
]
```

---

## Artifact Examples

### Chart

```json
{
  "type": "chart",
  "title": "Request Rate by Day",
  "chartType": "line",
  "data": [
    { "name": "Mon", "requests": 1200, "errors": 15 },
    { "name": "Tue", "requests": 1350, "errors": 22 },
    { "name": "Wed", "requests": 1100, "errors": 8 }
  ]
}
```

### Table

```json
{
  "type": "table",
  "title": "Top Alerting Rules",
  "columns": [
    { "key": "rule", "label": "Rule Name" },
    { "key": "fires", "label": "Fires (24h)", "align": "right" },
    { "key": "severity", "label": "Severity", "align": "center" }
  ],
  "rows": [
    { "rule": "HighCPU", "fires": 12, "severity": "warning" },
    { "rule": "DiskFull", "fires": 3, "severity": "critical" }
  ]
}
```

### Metric Cards

```json
{
  "type": "metric-cards",
  "title": "System Overview",
  "metrics": [
    { "label": "Active Alerts", "value": 7, "change": 40, "changeLabel": "vs yesterday", "icon": "alert", "color": "red" },
    { "label": "Uptime", "value": "99.97%", "icon": "activity", "color": "green" },
    { "label": "Avg Response", "value": "142ms", "change": -12, "changeLabel": "vs last week", "icon": "clock", "color": "blue" }
  ]
}
```

### Report (composite)

```json
{
  "type": "report",
  "title": "Daily Monitoring Summary",
  "subtitle": "2025-01-30",
  "sections": [
    { "type": "summary", "title": "Executive Summary", "content": "All systems healthy. 3 alerts resolved." },
    { "type": "metrics", "metrics": [
      { "label": "Alerts", "value": 3, "icon": "alert", "color": "amber" },
      { "label": "Services Up", "value": "12/12", "icon": "server", "color": "green" }
    ]},
    { "type": "chart", "title": "Error Rate", "chartType": "area", "data": [
      { "name": "00:00", "errors": 2 }, { "name": "06:00", "errors": 0 }, { "name": "12:00", "errors": 5 }
    ]},
    { "type": "text", "title": "Notes", "content": "Scheduled maintenance window at 02:00 UTC caused brief spike." }
  ]
}
```

---

## SSE Streaming Protocol

Chat responses are streamed via Server-Sent Events on `POST /api/chat`. Each SSE `data:` line contains a JSON `StreamChunk`.

### StreamChunk

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `"start" \| "token" \| "tool" \| "error" \| "complete" \| "done"` | Yes | Chunk type |
| `message` | `string` | No | Token text (for `token`) or full response (for `complete`) or error text (for `error`) |
| `tool` | `string` | No | Tool name (for `tool` chunks) |
| `arguments` | `object` | No | Tool call arguments (for `tool` chunks) |
| `result` | `any` | No | Tool execution result (for `tool` chunks) |

### Chunk Lifecycle

A complete streamed response follows this sequence:

```
data: {"type":"start"}

data: {"type":"token","message":"Let me "}
data: {"type":"token","message":"check the "}
data: {"type":"token","message":"alerts..."}

data: {"type":"tool","tool":"alertmanager__list_alerts","arguments":{"state":"active"}}
data: {"type":"tool","tool":"alertmanager__list_alerts","result":[...]}

data: {"type":"token","message":"There are "}
data: {"type":"token","message":"3 active alerts."}

data: {"type":"complete","message":"Let me check the alerts... There are 3 active alerts."}
data: {"type":"done"}
```

On error at any point:
```
data: {"type":"error","message":"LLM request failed: rate limit exceeded"}
```

### Frontend Handling

1. On `start`: clear streaming state, show typing indicator.
2. On `token`: append `message` to the current response buffer, render incrementally.
3. On `tool`: display tool call in UI (name + arguments), then update with result when the second `tool` chunk arrives.
4. On `complete`: replace streamed buffer with final `message` (canonical response).
5. On `done`: finalize message, parse `\`\`\`artifact\`\`\`` blocks, render artifacts.
6. On `error`: display error to user, stop streaming.

---

## Chat API Types

### ChatRequest (POST /api/chat)

```json
{
  "message": "What alerts are firing?",
  "session_id": "user-42-org-1",
  "dashboard_context": {
    "uid": "abc123",
    "name": "Production Overview",
    "folder": "Operations",
    "tags": ["production", "sre"],
    "time_range": { "from": "now-1h", "to": "now" },
    "variables": { "server": "prod-01", "region": "eu-west-1" }
  }
}
```

### ChatResponse (non-streaming fallback)

```json
{
  "response": "There are 3 active alerts...",
  "session_id": "user-42-org-1"
}
```

### Message (UI state)

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique message ID |
| `role` | `"user" \| "assistant"` | Sender |
| `content` | `string` | Message text (may contain artifact blocks) |
| `timestamp` | `ISO 8601 string` | When the message was created |
| `toolCalls` | `array<ToolCall>` | Tool calls made during this response |
| `isStreaming` | `boolean` | Whether this message is still being streamed |

### ToolCall

| Field | Type | Description |
|-------|------|-------------|
| `tool` | `string` | Tool name (e.g., `alertmanager__list_alerts`) |
| `arguments` | `object` | Arguments passed to the tool |
| `output` | `string` | Formatted tool result |

---

## Go Type Mapping

These are the corresponding Go structs for the backend. Located in `internal/api/types.go`:

```go
type ArtifactData struct {
    Type        string          `json:"type"`
    Title       string          `json:"title,omitempty"`
    Subtitle    string          `json:"subtitle,omitempty"`
    Description string          `json:"description,omitempty"`
    ChartType   string          `json:"chartType,omitempty"`
    Data        json.RawMessage `json:"data,omitempty"`
    Metrics     []MetricCard    `json:"metrics,omitempty"`
    Columns     []TableColumn   `json:"columns,omitempty"`
    Rows        json.RawMessage `json:"rows,omitempty"`
    Sections    []ReportSection `json:"sections,omitempty"`
}

type MetricCard struct {
    Label       string      `json:"label"`
    Value       interface{} `json:"value"`
    Change      *float64    `json:"change,omitempty"`
    ChangeLabel string      `json:"changeLabel,omitempty"`
    Icon        string      `json:"icon,omitempty"`
    Color       string      `json:"color,omitempty"`
}

type TableColumn struct {
    Key   string `json:"key"`
    Label string `json:"label"`
    Align string `json:"align,omitempty"`
}

type ReportSection struct {
    Type      string          `json:"type"`
    Title     string          `json:"title,omitempty"`
    Content   string          `json:"content,omitempty"`
    Data      json.RawMessage `json:"data,omitempty"`
    ChartType string          `json:"chartType,omitempty"`
    Metrics   []MetricCard    `json:"metrics,omitempty"`
    Columns   []TableColumn   `json:"columns,omitempty"`
    Rows      json.RawMessage `json:"rows,omitempty"`
}

type StreamChunk struct {
    Type      string                 `json:"type"`
    Message   string                 `json:"message,omitempty"`
    Tool      string                 `json:"tool,omitempty"`
    Arguments map[string]interface{} `json:"arguments,omitempty"`
    Result    interface{}            `json:"result,omitempty"`
}

type ChatRequest struct {
    Message          string            `json:"message"`
    SessionID        string            `json:"session_id,omitempty"`
    DashboardContext *DashboardContext `json:"dashboard_context,omitempty"`
}

type DashboardContext struct {
    UID       string            `json:"uid"`
    Name      string            `json:"name"`
    Folder    string            `json:"folder,omitempty"`
    Tags      []string          `json:"tags,omitempty"`
    TimeRange map[string]string `json:"time_range,omitempty"`
    Variables map[string]string `json:"variables,omitempty"`
}
```

---

## Origin

These schemas are derived from:
- `grafana-chat-plugin/src/types.ts` — TypeScript interfaces
- `grafana-chat-plugin/src/components/Artifact.tsx` — `ArtifactData` and sub-types
- `grafana-chat-plugin/pkg/plugin/types.go` — Go request/response types
- `grafana-chat-plugin/pkg/plugin/streaming.go` — SSE chunk handling
