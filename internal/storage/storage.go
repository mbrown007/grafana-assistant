package storage

import (
	"context"
	"time"
)

// User represents a resolved Grafana user.
type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
	OrgID int64  `json:"orgId"`
}

// Session represents a chat session.
type Session struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	OrgID     int64     `json:"org_id"`
	DashboardUID string `json:"dashboard_uid,omitempty"`
	ScratchpadUID string `json:"scratchpad_uid,omitempty"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a single chat message.
type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Feedback represents user feedback on an assistant message.
type Feedback struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	MessageID string    `json:"message_id"`
	UserID    int64     `json:"user_id"`
	OrgID     int64     `json:"org_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditEntry represents an audit log entry for chat activity.
type AuditEntry struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	UserID      int64     `json:"user_id"`
	OrgID       int64     `json:"org_id"`
	DashboardUID string   `json:"dashboard_uid,omitempty"`
	EventType   string    `json:"event_type"` // user_message, tool_call, assistant_response
	ToolName    string    `json:"tool_name,omitempty"`
	Request     string    `json:"request,omitempty"`
	Response    string    `json:"response,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store defines the interface for persistent storage.
type Store interface {
	// Sessions
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessions(ctx context.Context, userID, orgID int64, dashboardUID string) ([]Session, error)
	UpdateSessionMeta(ctx context.Context, id, title, dashboardUID string, updatedAt time.Time) error
	UpdateSessionScratchpad(ctx context.Context, id, scratchpadUID string, updatedAt time.Time) error
	DeleteSession(ctx context.Context, id string) error

	// Messages
	AddMessage(ctx context.Context, msg *Message) error
	GetMessages(ctx context.Context, sessionID string) ([]Message, error)

	// Feedback
	AddFeedback(ctx context.Context, feedback *Feedback) error

	// Audit log
	AddAuditEntry(ctx context.Context, entry *AuditEntry) error

	// Maintenance
	PurgeOlderThan(ctx context.Context, before time.Time) (int64, error)

	Close() error
}
