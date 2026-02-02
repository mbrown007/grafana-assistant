package kb

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	_ "modernc.org/sqlite"
)

// VectorSection represents a KB section with its embedding.
type VectorSection struct {
	ID          string
	Title       string
	Content     string
	Path        string
	Embedding   []float32
	ContentHash string
}

// VectorIndex wraps a SQLite database for storing and searching vector embeddings.
type VectorIndex struct {
	db *sql.DB
}

// VectorSearchResult pairs a section with its cosine similarity score.
type VectorSearchResult struct {
	Section VectorSection
	Score   float64
}

const createVectorTable = `CREATE TABLE IF NOT EXISTS kb_vectors (
	id           TEXT PRIMARY KEY,
	title        TEXT NOT NULL,
	content      TEXT NOT NULL,
	path         TEXT NOT NULL,
	embedding    BLOB NOT NULL,
	content_hash TEXT NOT NULL
)`

// OpenVectorIndex opens (or creates) a SQLite vector index at dbPath.
func OpenVectorIndex(dbPath string) (*VectorIndex, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open vector db: %w", err)
	}
	if _, err := db.Exec(createVectorTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create vector table: %w", err)
	}
	return &VectorIndex{db: db}, nil
}

// UpsertSection inserts or replaces a section in the vector index.
func (vi *VectorIndex) UpsertSection(sec VectorSection) error {
	blob := encodeEmbedding(sec.Embedding)
	_, err := vi.db.Exec(
		`INSERT OR REPLACE INTO kb_vectors (id, title, content, path, embedding, content_hash) VALUES (?, ?, ?, ?, ?, ?)`,
		sec.ID, sec.Title, sec.Content, sec.Path, blob, sec.ContentHash,
	)
	return err
}

// DeleteStale removes sections whose IDs are not in currentIDs. Returns the number deleted.
func (vi *VectorIndex) DeleteStale(currentIDs map[string]struct{}) (int, error) {
	rows, err := vi.db.Query(`SELECT id FROM kb_vectors`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toDelete []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		if _, ok := currentIDs[id]; !ok {
			toDelete = append(toDelete, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, id := range toDelete {
		if _, err := vi.db.Exec(`DELETE FROM kb_vectors WHERE id = ?`, id); err != nil {
			return 0, err
		}
	}
	return len(toDelete), nil
}

// Search performs brute-force cosine similarity search over all stored embeddings.
func (vi *VectorIndex) Search(queryEmbedding []float32, limit int) []VectorSearchResult {
	rows, err := vi.db.Query(`SELECT id, title, content, path, embedding, content_hash FROM kb_vectors`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var sec VectorSection
		var blob []byte
		if err := rows.Scan(&sec.ID, &sec.Title, &sec.Content, &sec.Path, &blob, &sec.ContentHash); err != nil {
			continue
		}
		sec.Embedding = decodeEmbedding(blob)
		score := cosineSimilarity(queryEmbedding, sec.Embedding)
		results = append(results, VectorSearchResult{Section: sec, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// ContentHashForSection returns the content hash for a given section ID, or "" if not found.
func (vi *VectorIndex) ContentHashForSection(id string) string {
	var hash string
	_ = vi.db.QueryRow(`SELECT content_hash FROM kb_vectors WHERE id = ?`, id).Scan(&hash)
	return hash
}

// Count returns the number of sections in the vector index.
func (vi *VectorIndex) Count() int {
	var n int
	_ = vi.db.QueryRow(`SELECT COUNT(*) FROM kb_vectors`).Scan(&n)
	return n
}

// Close closes the underlying database.
func (vi *VectorIndex) Close() error {
	return vi.db.Close()
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// encodeEmbedding serializes a float32 slice to a little-endian byte blob.
func encodeEmbedding(emb []float32) []byte {
	buf := make([]byte, len(emb)*4)
	for i, v := range emb {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// decodeEmbedding deserializes a little-endian byte blob to a float32 slice.
func decodeEmbedding(data []byte) []float32 {
	n := len(data) / 4
	emb := make([]float32, n)
	for i := range emb {
		emb[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return emb
}

// ContentHash returns the SHA-256 hex digest of content.
func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}
