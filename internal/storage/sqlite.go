package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite implements Store using a local SQLite database.
type SQLite struct {
	db       *sql.DB
	audit    *auditWriter
	auditMu  sync.Mutex
}

// NewSQLite opens (or creates) a SQLite database at path and runs migrations.
func NewSQLite(path string) (*SQLite, error) {
	return NewSQLiteWithAudit(path, "")
}

// NewSQLiteWithAudit opens (or creates) a SQLite database at path and optionally
// configures a JSONL audit log file.
func NewSQLiteWithAudit(path, auditLogPath string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &SQLite{db: db}
	if auditLogPath != "" {
		audit, err := openAuditWriter(auditLogPath)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("open audit log: %w", err)
		}
		s.audit = audit
	}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *SQLite) migrate() error {
	// Phase 1: create tables without columns that may need migration.
	const tables = `
	CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL,
		org_id     INTEGER NOT NULL DEFAULT 1,
		title      TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id, org_id);

	CREATE TABLE IF NOT EXISTS messages (
		id         TEXT PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		role       TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
		content    TEXT NOT NULL,
		created_at TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);

	CREATE TABLE IF NOT EXISTS audit_log (
		id           TEXT PRIMARY KEY,
		session_id   TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		user_id      INTEGER NOT NULL,
		org_id       INTEGER NOT NULL,
		dashboard_uid TEXT NOT NULL DEFAULT '',
		event_type   TEXT NOT NULL,
		tool_name    TEXT NOT NULL DEFAULT '',
		request      TEXT NOT NULL DEFAULT '',
		response     TEXT NOT NULL DEFAULT '',
		created_at   TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_log(session_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id, org_id, created_at);

	CREATE TABLE IF NOT EXISTS user_feedback (
		id          TEXT PRIMARY KEY,
		session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		message_id  TEXT NOT NULL,
		user_id     INTEGER NOT NULL,
		org_id      INTEGER NOT NULL,
		rating      INTEGER NOT NULL,
		comment     TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_feedback_session ON user_feedback(session_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_feedback_user ON user_feedback(user_id, org_id, created_at);

	PRAGMA foreign_keys = ON;
	`
	if _, err := s.db.Exec(tables); err != nil {
		return err
	}

	// Phase 2: ensure columns added in later phases exist.
	if err := s.ensureColumn("sessions", "dashboard_uid", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("sessions", "scratchpad_uid", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	// Phase 3: create indexes that depend on migrated columns.
	const indexes = `
	CREATE INDEX IF NOT EXISTS idx_sessions_dashboard ON sessions(user_id, org_id, dashboard_uid);
	CREATE INDEX IF NOT EXISTS idx_sessions_scratchpad ON sessions(user_id, org_id, scratchpad_uid);
	`
	if _, err := s.db.Exec(indexes); err != nil {
		return err
	}

	return nil
}

func (s *SQLite) ensureColumn(table, column, decl string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notNull   int
			defValue  sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + decl)
	return err
}

// CreateSession inserts a new session.
func (s *SQLite) CreateSession(ctx context.Context, sess *Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, org_id, dashboard_uid, scratchpad_uid, title, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.OrgID, sess.DashboardUID, sess.ScratchpadUID, sess.Title,
		sess.CreatedAt.Format(time.RFC3339),
		sess.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

// GetSession retrieves a session by ID.
func (s *SQLite) GetSession(ctx context.Context, id string) (*Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, org_id, dashboard_uid, scratchpad_uid, title, created_at, updated_at FROM sessions WHERE id = ?`, id)

	sess := &Session{}
	var createdAt, updatedAt string
	if err := row.Scan(&sess.ID, &sess.UserID, &sess.OrgID, &sess.DashboardUID, &sess.ScratchpadUID, &sess.Title, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	sess.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	sess.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return sess, nil
}

// ListSessions returns all sessions for a user/org, newest first.
func (s *SQLite) ListSessions(ctx context.Context, userID, orgID int64, dashboardUID string) ([]Session, error) {
	query := `SELECT id, user_id, org_id, dashboard_uid, scratchpad_uid, title, created_at, updated_at
		 FROM sessions WHERE user_id = ? AND org_id = ?`
	args := []any{userID, orgID}
	if dashboardUID != "" {
		query += ` AND dashboard_uid = ?`
		args = append(args, dashboardUID)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var createdAt, updatedAt string
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.OrgID, &sess.DashboardUID, &sess.ScratchpadUID, &sess.Title, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		sess.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		sess.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// UpdateSessionMeta updates title/dashboard UID and updated_at for a session.
func (s *SQLite) UpdateSessionMeta(ctx context.Context, id, title, dashboardUID string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET title = ?, dashboard_uid = ?, updated_at = ? WHERE id = ?`,
		title, dashboardUID, updatedAt.Format(time.RFC3339), id,
	)
	return err
}

// UpdateSessionScratchpad updates scratchpad UID and updated_at for a session.
func (s *SQLite) UpdateSessionScratchpad(ctx context.Context, id, scratchpadUID string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET scratchpad_uid = ?, updated_at = ? WHERE id = ?`,
		scratchpadUID, updatedAt.Format(time.RFC3339), id,
	)
	return err
}

// DeleteSession removes a session and its messages (cascade).
func (s *SQLite) DeleteSession(ctx context.Context, id string) error {
	// Ensure foreign keys are on for cascade delete.
	_, _ = s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// AddMessage inserts a chat message.
func (s *SQLite) AddMessage(ctx context.Context, msg *Message) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO messages (id, session_id, role, content, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, msg.Role, msg.Content,
		msg.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return err
	}

	// Touch session updated_at.
	_, err = s.db.ExecContext(ctx,
		`UPDATE sessions SET updated_at = ? WHERE id = ?`,
		msg.CreatedAt.Format(time.RFC3339), msg.SessionID,
	)
	return err
}

// AddFeedback inserts a user feedback entry.
func (s *SQLite) AddFeedback(ctx context.Context, feedback *Feedback) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_feedback (id, session_id, message_id, user_id, org_id, rating, comment, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		feedback.ID, feedback.SessionID, feedback.MessageID, feedback.UserID, feedback.OrgID,
		feedback.Rating, feedback.Comment, feedback.CreatedAt.Format(time.RFC3339),
	)
	return err
}

// AddAuditEntry inserts an audit log entry.
func (s *SQLite) AddAuditEntry(ctx context.Context, entry *AuditEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (id, session_id, user_id, org_id, dashboard_uid, event_type, tool_name, request, response, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.SessionID, entry.UserID, entry.OrgID, entry.DashboardUID, entry.EventType, entry.ToolName,
		entry.Request, entry.Response, entry.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return err
	}
	if s.audit != nil {
		if err := s.writeAuditEntry(entry); err != nil {
			slog.Error("failed to write audit log", "error", err)
		}
	}
	return nil
}

// GetMessages returns all messages for a session in chronological order.
func (s *SQLite) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, role, content, created_at
		 FROM messages WHERE session_id = ?
		 ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var createdAt string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &createdAt); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// PurgeOlderThan deletes sessions (and their messages) updated before the given time.
// Returns the number of sessions deleted.
func (s *SQLite) PurgeOlderThan(ctx context.Context, before time.Time) (int64, error) {
	_, _ = s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE updated_at < ?`,
		before.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Close closes the database connection.
func (s *SQLite) Close() error {
	if s.audit != nil {
		_ = s.audit.Close()
	}
	return s.db.Close()
}

type auditWriter struct {
	file    *os.File
	encoder *json.Encoder
}

func openAuditWriter(path string) (*auditWriter, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}
	return &auditWriter{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

func (w *auditWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (s *SQLite) writeAuditEntry(entry *AuditEntry) error {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	record := map[string]any{
		"id":            entry.ID,
		"session_id":    entry.SessionID,
		"user_id":       entry.UserID,
		"org_id":        entry.OrgID,
		"dashboard_uid": entry.DashboardUID,
		"event_type":    entry.EventType,
		"event":         entry.EventType,
		"tool_name":     entry.ToolName,
		"request":       entry.Request,
		"response":      entry.Response,
		"created_at":    entry.CreatedAt.Format(time.RFC3339),
	}
	return s.audit.encoder.Encode(record)
}
