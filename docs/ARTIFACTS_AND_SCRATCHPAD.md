# In-Chat Artifacts and Grafana Scratchpads

The Monitoring Assistant uses two primary mechanisms for visualizing data and responding to user queries: **in-chat artifacts** and **Grafana scratchpad dashboards**. This document explains what they are, how they work, and when to use each.

## Overview

- **Artifacts** are for quick, static visualizations rendered directly in the chat panel. They are best for simple, self-contained answers like small tables or single-metric charts.
- **Scratchpads** are temporary, interactive Grafana dashboards created per-session. They are ideal for complex queries, exploration, and situations where the user will want to drill down or modify the visualization.

The assistant will automatically choose the best mechanism based on the complexity of the user's request.

## In-Chat Artifacts

Artifacts are rich, structured data objects that the LLM can generate to be rendered as visualizations within the chat UI.

### How they work

The LLM includes a special JSON object inside a fenced code block in its response:

````markdown
```artifact
{
  "type": "table",
  "title": "Top 5 Services by Error Rate",
  "columns": [
    {"key": "service", "label": "Service"},
    {"key": "error_rate", "label": "Error Rate"}
  ],
  "rows": [
    {"service": "checkout-api", "error_rate": "5.2%"},
    {"service": "payment-gateway", "error_rate": "3.1%"}
  ]
}
```
````

The frontend parses this `artifact` block, removes it from the markdown, and renders the JSON as an interactive component.

### Supported Artifact Types

#### 1. `table`
Renders a simple data table.
- `title`: The title of the table.
- `columns`: An array of objects with `key` and `label` properties.
- `rows`: An array of data objects.

#### 2. `chart`
Renders a single chart.
- `title`: The title of the chart.
- `chartType`: The type of chart to render. Can be `bar`, `line`, `pie`, or `area`.
- `data`: An array of data objects. For most charts, each object should have a `name` property for the x-axis label and one or more numeric properties for the y-axis values.

#### 3. `metric-cards`
Displays a grid of key metrics.
- `title`: A title for the group of cards.
- `metrics`: An array of metric objects, each with:
  - `label`: The metric's name.
  - `value`: The metric's value.
  - `change` (optional): A percentage change to display.
  - `icon` (optional): An icon name (e.g., `users`, `server`, `trending_up`).

#### 4. `report`
A multi-section artifact that can combine other types.
- `title`: The main title of the report.
- `sections`: An array of section objects. Each section can have a `type` of `text`, `metrics`, `chart`, or `table`, along with the corresponding properties.

### When to Use Artifacts
- **Tables:** Always use for tabular data.
- **Single-metric charts** with a small number of series.
- **Short time windows** and quick spot checks.
- **Static summaries** or “reporting style” views.

## Grafana Scratchpad Dashboards

The scratchpad is a temporary, interactive Grafana dashboard created for each user session, designed for more complex data exploration.

### How it works

1.  **First Use:** When a user's query requires a complex graph, the assistant checks if a scratchpad dashboard exists for the current session (by searching for a dashboard with the tag `session-id:<session_id>`).
2.  **Creation:** If none exists, it creates a new dashboard in the "Assistant Scratchpads" folder using a predefined template. This new dashboard is tagged with the user's session ID.
3.  **Panel Update:** The assistant then updates the first panel on that dashboard with the LLM-generated query, title, and description.
4.  **Response:** The assistant replies with a link to this dashboard, e.g., "This is better as an interactive Grafana panel; I’ve put it in your [scratchpad](/grafana/d/<uid>)".
5.  **Cleanup:** These temporary dashboards are automatically deleted after a configurable period of inactivity (default: 7 days) by a background job.

### When to Use a Scratchpad

- **Multi-panel or multi-query** outputs that are too dense for a single artifact.
- **Long time ranges** requiring exploration or zooming (e.g., days/weeks).
- **High-cardinality breakdowns** where the user will likely want to filter or slice the data further.
- **Complex overlays** (e.g., combining metrics with alert thresholds or deploy annotations).
- When the **user explicitly asks to “explore” or “drill down”** into the data.
