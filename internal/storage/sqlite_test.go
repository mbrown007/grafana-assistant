package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *SQLite {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSessionCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	sess := &Session{
		ID:        "sess-1",
		UserID:    42,
		OrgID:     1,
		DashboardUID: "dash-1",
		Title:     "Test Session",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := db.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got == nil {
		t.Fatal("GetSession returned nil")
	}
	if got.Title != "Test Session" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Session")
	}
	if got.UserID != 42 {
		t.Errorf("UserID = %d, want 42", got.UserID)
	}
	if got.DashboardUID != "dash-1" {
		t.Errorf("DashboardUID = %q, want %q", got.DashboardUID, "dash-1")
	}

	// List
	sessions, err := db.ListSessions(ctx, 42, 1, "")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("ListSessions returned %d sessions, want 1", len(sessions))
	}

	// List with dashboard filter
	filtered, err := db.ListSessions(ctx, 42, 1, "dash-1")
	if err != nil {
		t.Fatalf("ListSessions filter: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("ListSessions filter returned %d sessions, want 1", len(filtered))
	}

	filtered, err = db.ListSessions(ctx, 42, 1, "other")
	if err != nil {
		t.Fatalf("ListSessions filter (other): %v", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("ListSessions filter (other) returned %d sessions, want 0", len(filtered))
	}

	// Not found
	got, err = db.GetSession(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSession nonexistent: %v", err)
	}
	if got != nil {
		t.Error("GetSession should return nil for nonexistent session")
	}

	if err := db.UpdateSessionMeta(ctx, "sess-1", "Updated Title", "dash-2", now.Add(time.Minute)); err != nil {
		t.Fatalf("UpdateSessionMeta: %v", err)
	}
	got, err = db.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession after update: %v", err)
	}
	if got.Title != "Updated Title" || got.DashboardUID != "dash-2" {
		t.Errorf("UpdateSessionMeta mismatch: %+v", got)
	}
}

func TestMessageCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	sess := &Session{
		ID: "sess-1", UserID: 1, OrgID: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	msg1 := &Message{
		ID: "msg-1", SessionID: "sess-1", Role: "user",
		Content: "Hello", CreatedAt: now,
	}
	msg2 := &Message{
		ID: "msg-2", SessionID: "sess-1", Role: "assistant",
		Content: "Hi there", CreatedAt: now.Add(time.Second),
	}

	if err := db.AddMessage(ctx, msg1); err != nil {
		t.Fatalf("AddMessage 1: %v", err)
	}
	if err := db.AddMessage(ctx, msg2); err != nil {
		t.Fatalf("AddMessage 2: %v", err)
	}

	msgs, err := db.GetMessages(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("GetMessages returned %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Errorf("messages not in chronological order: %v", msgs)
	}
}

func TestDeleteSessionCascade(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	sess := &Session{
		ID: "sess-1", UserID: 1, OrgID: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	db.CreateSession(ctx, sess)
	db.AddMessage(ctx, &Message{
		ID: "msg-1", SessionID: "sess-1", Role: "user",
		Content: "test", CreatedAt: now,
	})

	if err := db.DeleteSession(ctx, "sess-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	got, _ := db.GetSession(ctx, "sess-1")
	if got != nil {
		t.Error("session should be deleted")
	}

	msgs, _ := db.GetMessages(ctx, "sess-1")
	if len(msgs) != 0 {
		t.Errorf("messages should be cascade deleted, got %d", len(msgs))
	}
}

func TestPurgeOlderThan(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	recent := time.Now().Truncate(time.Second)

	db.CreateSession(ctx, &Session{
		ID: "old-sess", UserID: 1, OrgID: 1,
		CreatedAt: old, UpdatedAt: old,
	})
	db.CreateSession(ctx, &Session{
		ID: "new-sess", UserID: 1, OrgID: 1,
		CreatedAt: recent, UpdatedAt: recent,
	})

	cutoff := time.Now().Add(-24 * time.Hour)
	deleted, err := db.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	sessions, _ := db.ListSessions(ctx, 1, 1, "")
	if len(sessions) != 1 || sessions[0].ID != "new-sess" {
		t.Errorf("expected only new-sess to remain, got %v", sessions)
	}
}

func TestAuditLogInsert(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	sess := &Session{
		ID: "sess-1", UserID: 1, OrgID: 1,
		DashboardUID: "dash-1",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	entry := &AuditEntry{
		ID:           "audit-1",
		SessionID:    "sess-1",
		UserID:       1,
		OrgID:        1,
		DashboardUID: "dash-1",
		EventType:    "assistant_response",
		Response:     "hello",
		CreatedAt:    now,
	}
	if err := db.AddAuditEntry(ctx, entry); err != nil {
		t.Fatalf("AddAuditEntry: %v", err)
	}

	var count int
	row := db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log WHERE session_id = ?`, "sess-1")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan audit count: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit count = %d, want 1", count)
	}
}

func TestFeedbackInsert(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	sess := &Session{
		ID: "sess-1", UserID: 1, OrgID: 1,
		DashboardUID: "dash-1",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	entry := &Feedback{
		ID:        "feedback-1",
		SessionID: "sess-1",
		MessageID: "sess-1-123-assistant",
		UserID:    1,
		OrgID:     1,
		Rating:    4,
		Comment:   "Helpful",
		CreatedAt: now,
	}
	if err := db.AddFeedback(ctx, entry); err != nil {
		t.Fatalf("AddFeedback: %v", err)
	}

	var count int
	row := db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_feedback WHERE session_id = ?`, "sess-1")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan feedback count: %v", err)
	}
	if count != 1 {
		t.Fatalf("feedback count = %d, want 1", count)
	}
}
