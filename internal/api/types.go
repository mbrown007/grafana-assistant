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
	SubAgent  string                 `json:"subagent,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	ToolID    string                 `json:"tool_id,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Result    interface{}            `json:"result,omitempty"`
	Evidence  *EvidencePayload       `json:"evidence,omitempty"`
}

type EvidencePayload struct {
	KBSearch     *KBSearchEvidence     `json:"kb_search,omitempty"`
	VectorSearch *VectorSearchEvidence `json:"vector_search,omitempty"`
}

type EvidenceResult struct {
	ID      string  `json:"id,omitempty"`
	Path    string  `json:"path,omitempty"`
	Title   string  `json:"title,omitempty"`
	Excerpt string  `json:"excerpt,omitempty"`
	Score   float64 `json:"score,omitempty"`
	Source  string  `json:"source,omitempty"`
}

type KBSearchEvidence struct {
	Query   string           `json:"query"`
	Results []EvidenceResult `json:"results"`
}

type VectorSearchEvidence struct {
	Query   string           `json:"query"`
	Results []EvidenceResult `json:"results"`
}

type ChatRequest struct {
	Message          string            `json:"message"`
	SessionID        string            `json:"session_id,omitempty"`
	DashboardContext *DashboardContext `json:"dashboard_context,omitempty"`
	SelectedContext  []ContextEntity   `json:"selected_context,omitempty"`
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

// ContextEntityType defines the category of a user-selected context entity.
type ContextEntityType string

const (
	ContextEntityDatasource ContextEntityType = "datasource"
	ContextEntityDashboard  ContextEntityType = "dashboard"
	ContextEntityMetric     ContextEntityType = "metric"
	ContextEntityLabel      ContextEntityType = "label"
)

// ContextEntity is a single entity the user selected via the "@" context picker.
type ContextEntity struct {
	Type        ContextEntityType `json:"type"`
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ContextSearchResponse is returned by GET /api/context/search.
type ContextSearchResponse struct {
	Entities []ContextEntity `json:"entities"`
}

type ChatResponse struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id"`
}

type HistorySession struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	DashboardUID string    `json:"dashboard_uid,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type HistoryMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type HistoryDetail struct {
	Session  HistorySession   `json:"session"`
	Messages []HistoryMessage `json:"messages"`
}

type FeedbackRequest struct {
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment,omitempty"`
}
