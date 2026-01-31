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

// Store defines the interface for persistent storage.
type Store interface {
	// Sessions
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessions(ctx context.Context, userID, orgID int64) ([]Session, error)
	DeleteSession(ctx context.Context, id string) error

	// Messages
	AddMessage(ctx context.Context, msg *Message) error
	GetMessages(ctx context.Context, sessionID string) ([]Message, error)

	// Maintenance
	PurgeOlderThan(ctx context.Context, before time.Time) (int64, error)

	Close() error
}
