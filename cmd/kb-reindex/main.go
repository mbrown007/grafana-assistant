package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/marcusz/monitoring-assistant/pkg/kb"
)

func main() {
	var (
		structuredPath string
		outPath        string
		vectorPath     string
		vectorJSONL    string
		vectorDBPath   string
		embeddingModel string
		apiKey         string
	)
	flag.StringVar(&structuredPath, "structured-path", "KB/runbooks", "Path to structured KB folder (token index)")
	flag.StringVar(&outPath, "out", "", "Output token index path (default: <structured-path>/.kb_index.json)")
	flag.StringVar(&vectorPath, "vector-path", "", "Path to vector KB folder of .md files (empty = skip)")
	flag.StringVar(&vectorJSONL, "vector-jsonl", "", "Path to JSONL file from docs scraper (alternative to -vector-path)")
	flag.StringVar(&vectorDBPath, "vector-db", "KB/.kb_vectors.db", "Path to vector SQLite database")
	flag.StringVar(&embeddingModel, "embedding-model", "text-embedding-3-small", "OpenAI embedding model")
	flag.StringVar(&apiKey, "openai-api-key", "", "OpenAI API key (or set ASSISTANT_OPENAI_API_KEY)")

	// Backward compat: support old -kb-path flag as alias.
	flag.StringVar(&structuredPath, "kb-path", structuredPath, "Alias for -structured-path (backward compat)")

	flag.Parse()

	if apiKey == "" {
		apiKey = os.Getenv("ASSISTANT_OPENAI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	// 1. Build token index from structured path.
	if structuredPath != "" {
		if outPath == "" {
			outPath = kb.DefaultIndexPath(structuredPath)
		}

		idx, err := kb.BuildIndex(structuredPath)
		if err != nil {
			log.Fatalf("build token index: %v", err)
		}

		if err := kb.SaveIndex(outPath, idx); err != nil {
			log.Fatalf("save token index: %v", err)
		}

		fmt.Printf("Token index written to %s (sections=%d)\n", outPath, len(idx.Sections))
		_ = os.Chmod(outPath, 0o600)
	}

	// 2. Build vector index (from markdown dir or JSONL file).
	if vectorPath == "" && vectorJSONL == "" {
		return
	}
	if apiKey == "" {
		fmt.Println("Skipping vector index: no API key available")
		return
	}

	// Collect vector sections from the configured source.
	var vectorSections []kb.VectorSection

	switch {
	case vectorJSONL != "":
		// Load pre-chunked sections from scraper JSONL.
		sections, err := kb.LoadJSONLChunks(vectorJSONL)
		if err != nil {
			log.Fatalf("load JSONL chunks: %v", err)
		}
		vectorSections = sections
		fmt.Printf("Loaded %d chunks from %s\n", len(sections), vectorJSONL)

	case vectorPath != "":
		// Parse sections from markdown files.
		idx, err := kb.BuildIndex(vectorPath)
		if err != nil {
			log.Fatalf("build vector sections: %v", err)
		}
		for _, sec := range idx.Sections {
			vectorSections = append(vectorSections, kb.VectorSection{
				ID:          sec.ID,
				Title:       sec.Title,
				Content:     sec.Content,
				Path:        sec.Path,
				ContentHash: kb.ContentHash(sec.Content),
			})
		}
	}

	if len(vectorSections) == 0 {
		fmt.Println("No sections found for vector index, skipping")
		return
	}

	vi, err := kb.OpenVectorIndex(vectorDBPath)
	if err != nil {
		log.Fatalf("open vector db: %v", err)
	}
	defer vi.Close()

	embedder := kb.NewEmbedder(apiKey, embeddingModel)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Track current IDs for stale deletion.
	currentIDs := make(map[string]struct{}, len(vectorSections))

	var embedded, skipped int
	for _, sec := range vectorSections {
		currentIDs[sec.ID] = struct{}{}

		// Skip if content hasn't changed.
		existing := vi.ContentHashForSection(sec.ID)
		if existing == sec.ContentHash {
			skipped++
			continue
		}

		emb, err := embedder.EmbedSingle(ctx, sec.Title+"\n"+sec.Content)
		if err != nil {
			log.Printf("WARNING: failed to embed section %q: %v", sec.ID, err)
			continue
		}
		sec.Embedding = emb

		if err := vi.UpsertSection(sec); err != nil {
			log.Printf("WARNING: failed to upsert section %q: %v", sec.ID, err)
			continue
		}
		embedded++
	}

	deleted, err := vi.DeleteStale(currentIDs)
	if err != nil {
		log.Printf("WARNING: failed to delete stale sections: %v", err)
	}

	fmt.Printf("Vector index updated at %s (embedded=%d, skipped=%d, deleted=%d, total=%d)\n",
		vectorDBPath, embedded, skipped, deleted, vi.Count())
}
