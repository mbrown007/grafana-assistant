package kb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIndexAndSearch(t *testing.T) {
	dir := t.TempDir()
	content := `# Title

## Overview
Genesys Cloud metrics use the genesyscloud_ prefix.

### Edge Collector
genesyscloud_edge_cpu_percent reports CPU usage.
`
	path := filepath.Join(dir, "genesys.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	idx, err := BuildIndex(dir)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if len(idx.Sections) < 2 {
		t.Fatalf("expected sections, got %d", len(idx.Sections))
	}

	weights := map[string]int{"genesyscloud_edge_cpu_percent": 3, "edge": 1}
	results := Search(idx, weights, 2)
	if len(results) == 0 {
		t.Fatalf("expected search results")
	}
	if results[0].Score <= 0 {
		t.Fatalf("expected positive score")
	}
}

func TestSaveLoadIndex(t *testing.T) {
	dir := t.TempDir()
	content := `## Section
Some content.`
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	idx, err := BuildIndex(dir)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	indexPath := filepath.Join(dir, "index.json")
	if err := SaveIndex(indexPath, idx); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}
	loaded, err := LoadIndex(indexPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if len(loaded.Sections) != len(idx.Sections) {
		t.Fatalf("expected %d sections, got %d", len(idx.Sections), len(loaded.Sections))
	}
}
