package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
	"github.com/marcusz/monitoring-assistant/pkg/kb"
)

func TestBuildKBContextPrefersDashboard(t *testing.T) {
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

	mgr := NewManager(nil, nil, nil, nil, nil, ManagerConfig{KBPath: dir, KBMaxSections: 2, KBMaxSectionChars: 500})
	dash := &appcontext.DashboardSummary{
		Title: "Genesys Ops",
		Panels: []appcontext.PanelSummary{
			{
				Title:   "Edge CPU",
				Type:    "stat",
				Queries: []string{"genesyscloud_edge_cpu_percent"},
			},
		},
	}

	ctx, _, _ := mgr.buildKBContext("what am I looking at", dash, &api.DashboardContext{}, false)
	if !strings.Contains(strings.ToLower(ctx), "edge collector") {
		t.Fatalf("expected edge collector context, got: %s", ctx)
	}
}

func TestBuildKBEvidenceSplitsSourcesAndTruncates(t *testing.T) {
	results := []kb.UnifiedResult{
		{
			ID:      "doc1::Overview",
			Title:   "Overview",
			Content: strings.Repeat("a", 50),
			Path:    "docs/overview.md",
			Score:   0.9,
			Source:  "token",
		},
		{
			ID:      "doc2::Vector",
			Title:   "Vector",
			Content: strings.Repeat("b", 50),
			Path:    "docs/vector.md",
			Score:   0.8,
			Source:  "vector",
		},
	}

	kbEvidence, vectorEvidence := buildKBEvidence(results, 20)
	if kbEvidence == nil || vectorEvidence == nil {
		t.Fatalf("expected both KB and vector evidence, got kb=%v vector=%v", kbEvidence, vectorEvidence)
	}

	if len(kbEvidence.Results) != 1 {
		t.Fatalf("expected 1 KB result, got %d", len(kbEvidence.Results))
	}
	if len(vectorEvidence.Results) != 1 {
		t.Fatalf("expected 1 vector result, got %d", len(vectorEvidence.Results))
	}

	kbExcerpt := kbEvidence.Results[0].Excerpt
	if !strings.HasSuffix(kbExcerpt, "...") {
		t.Fatalf("expected KB excerpt to be truncated, got %q", kbExcerpt)
	}
	vectorExcerpt := vectorEvidence.Results[0].Excerpt
	if !strings.HasSuffix(vectorExcerpt, "...") {
		t.Fatalf("expected vector excerpt to be truncated, got %q", vectorExcerpt)
	}
}

func TestBuildKBContextPrefersDashboardMap(t *testing.T) {
	dir := t.TempDir()
	content := `# Monitoring Assistant Observability

## Overview
This dashboard tracks chat volume and audit logs.
`
	platformDir := filepath.Join(dir, "platform", "Monitoring_Assistant")
	if err := os.MkdirAll(platformDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(platformDir, "monitoring_assistant_observability_dashboard.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	mgr := NewManager(nil, nil, nil, nil, nil, ManagerConfig{
		KBPath:            dir,
		KBMaxSections:     2,
		KBMaxSectionChars: 500,
		KBDashboardMap: map[string]string{
			"monitoring assistant observability": "Monitoring_Assistant/monitoring_assistant_observability_dashboard.md",
		},
	})
	reqCtx := &api.DashboardContext{Name: "Monitoring Assistant Observability"}

	ctx, _, _ := mgr.buildKBContext("what is this dashboard", nil, reqCtx, true)
	if !strings.Contains(strings.ToLower(ctx), "chat volume") {
		t.Fatalf("expected mapped dashboard context, got: %s", ctx)
	}
}
