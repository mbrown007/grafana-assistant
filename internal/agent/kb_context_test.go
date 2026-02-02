package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
	appcontext "github.com/marcusz/monitoring-assistant/internal/context"
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

	ctx := mgr.buildKBContext("what am I looking at", dash, &api.DashboardContext{})
	if !strings.Contains(strings.ToLower(ctx), "edge collector") {
		t.Fatalf("expected edge collector context, got: %s", ctx)
	}
}
