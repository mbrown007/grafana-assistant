package storage

// Store defines the interface for persistent storage.
// SQLite implementation will be added in Phase 1.
type Store interface {
	Close() error
}
