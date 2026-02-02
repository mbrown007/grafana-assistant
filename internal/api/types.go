package api

import (
	"encoding/json"
	"time"
)

// ArtifactData represents a rich data visualization embedded in assistant responses.
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
	SessionID string                 `json:"session_id,omitempty"`
	Tool      string                 `json:"tool,omitempty"`
	ToolID    string                 `json:"tool_id,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Result    interface{}            `json:"result,omitempty"`
}

type ChatRequest struct {
	Message          string            `json:"message"`
	SessionID        string            `json:"session_id,omitempty"`
	DashboardContext *DashboardContext `json:"dashboard_context,omitempty"`
}

type DashboardContext struct {
	UID       string            `json:"uid,omitempty"`
	Name      string            `json:"name,omitempty"`
	Folder    string            `json:"folder,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	TimeRange map[string]string `json:"time_range,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Explore   *ExploreContext   `json:"explore,omitempty"`
}

type ExploreContext struct {
	Datasource string   `json:"datasource,omitempty"`
	Queries    []string `json:"queries,omitempty"`
}

type ChatResponse struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id"`
}

type HistorySession struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	DashboardUID string   `json:"dashboard_uid,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type HistoryMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type HistoryDetail struct {
	Session  HistorySession  `json:"session"`
	Messages []HistoryMessage `json:"messages"`
}
