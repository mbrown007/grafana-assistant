package kb

import (
	"math"
	"path/filepath"
	"testing"
)

func TestVectorIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_vectors.db")

	vi, err := OpenVectorIndex(dbPath)
	if err != nil {
		t.Fatalf("OpenVectorIndex: %v", err)
	}
	defer vi.Close()

	emb := []float32{0.1, 0.2, 0.3, 0.4}
	sec := VectorSection{
		ID:          "doc.md::Overview",
		Title:       "Overview",
		Content:     "Some content about the platform.",
		Path:        "doc.md",
		Embedding:   emb,
		ContentHash: ContentHash("Some content about the platform."),
	}

	if err := vi.UpsertSection(sec); err != nil {
		t.Fatalf("UpsertSection: %v", err)
	}

	if vi.Count() != 1 {
		t.Fatalf("expected count=1, got %d", vi.Count())
	}

	// Search with same embedding should return score ~1.0.
	results := vi.Search(emb, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Section.ID != sec.ID {
		t.Fatalf("unexpected section ID: %s", results[0].Section.ID)
	}
	if results[0].Score < 0.99 {
		t.Fatalf("expected score ~1.0, got %f", results[0].Score)
	}

	// Content hash check.
	hash := vi.ContentHashForSection(sec.ID)
	if hash != sec.ContentHash {
		t.Fatalf("content hash mismatch: %s vs %s", hash, sec.ContentHash)
	}
}

func TestVectorIndexUpsertAndDeleteStale(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_vectors.db")

	vi, err := OpenVectorIndex(dbPath)
	if err != nil {
		t.Fatalf("OpenVectorIndex: %v", err)
	}
	defer vi.Close()

	for _, id := range []string{"a", "b", "c"} {
		if err := vi.UpsertSection(VectorSection{
			ID:          id,
			Title:       id,
			Content:     id,
			Path:        id + ".md",
			Embedding:   []float32{1, 0, 0},
			ContentHash: ContentHash(id),
		}); err != nil {
			t.Fatalf("UpsertSection %s: %v", id, err)
		}
	}

	if vi.Count() != 3 {
		t.Fatalf("expected 3, got %d", vi.Count())
	}

	// Keep only "a" and "c".
	current := map[string]struct{}{"a": {}, "c": {}}
	deleted, err := vi.DeleteStale(current)
	if err != nil {
		t.Fatalf("DeleteStale: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}
	if vi.Count() != 2 {
		t.Fatalf("expected 2 remaining, got %d", vi.Count())
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	score := cosineSimilarity(a, b)
	if math.Abs(score-1.0) > 1e-6 {
		t.Fatalf("expected 1.0, got %f", score)
	}

	c := []float32{0, 1, 0}
	score = cosineSimilarity(a, c)
	if math.Abs(score) > 1e-6 {
		t.Fatalf("expected 0, got %f", score)
	}

	// Empty/mismatched vectors.
	if cosineSimilarity(nil, nil) != 0 {
		t.Fatal("expected 0 for nil")
	}
	if cosineSimilarity([]float32{1}, []float32{1, 2}) != 0 {
		t.Fatal("expected 0 for mismatched lengths")
	}
}

func TestEncodeDecodeEmbedding(t *testing.T) {
	original := []float32{0.1, 0.2, 0.3, -0.5, 1.0}
	encoded := encodeEmbedding(original)
	decoded := decodeEmbedding(encoded)

	if len(decoded) != len(original) {
		t.Fatalf("expected len %d, got %d", len(original), len(decoded))
	}
	for i := range original {
		if original[i] != decoded[i] {
			t.Fatalf("mismatch at %d: %f vs %f", i, original[i], decoded[i])
		}
	}
}

func TestContentHash(t *testing.T) {
	h1 := ContentHash("hello")
	h2 := ContentHash("hello")
	h3 := ContentHash("world")
	if h1 != h2 {
		t.Fatal("same content should produce same hash")
	}
	if h1 == h3 {
		t.Fatal("different content should produce different hash")
	}
}
