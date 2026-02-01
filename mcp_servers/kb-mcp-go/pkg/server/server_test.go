package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	kb "github.com/marcusz/monitoring-assistant/pkg/kb"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestSearchAndGetSection(t *testing.T) {
	dir := t.TempDir()
	content := `# Doc

## Overview
Genesys Cloud metrics use the genesyscloud_ prefix.

### Edge Collector
genesyscloud_edge_cpu_percent reports CPU usage.
`
	if err := os.WriteFile(filepath.Join(dir, "genesys.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	idx, err := kb.BuildIndex(dir)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	srv := NewMCPServer(idx)

	result, err := srv.handleSearch(map[string]any{"query": "edge cpu", "limit": 3})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}
	payload := mustTextResult(t, result)
	var hits []map[string]any
	if err := json.Unmarshal([]byte(payload), &hits); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected search hits")
	}

	id, _ := hits[0]["id"].(string)
	if id == "" {
		t.Fatalf("missing id in search results")
	}

	sectionResult, err := srv.handleGetSection(map[string]any{"id": id})
	if err != nil {
		t.Fatalf("handleGetSection error: %v", err)
	}
	sectionPayload := mustTextResult(t, sectionResult)
	var section map[string]any
	if err := json.Unmarshal([]byte(sectionPayload), &section); err != nil {
		t.Fatalf("decode section: %v", err)
	}
	if section["id"] != id {
		t.Fatalf("unexpected section id")
	}
}

func TestSearchValidation(t *testing.T) {
	idx := &kb.Index{Sections: []kb.Section{}}
	srv := NewMCPServer(idx)

	result, err := srv.handleSearch(map[string]any{"limit": 3})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result")
	}
}

func mustTextResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("empty tool result")
	}
	content, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected tool result type")
	}
	return content.Text
}
