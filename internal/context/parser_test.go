package context

import (
	"testing"
)

func TestParseDashboardURL_Standard(t *testing.T) {
	ctx := ParseDashboardURL("http://localhost:3000/d/abc123/my-dashboard")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.UID != "abc123" {
		t.Errorf("expected UID abc123, got %s", ctx.UID)
	}
}

func TestParseDashboardURL_WithTimeRangeAndVars(t *testing.T) {
	ctx := ParseDashboardURL("http://localhost:3000/d/xyz/dash?from=now-6h&to=now&var-server=prod-01&var-region=us-east")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.UID != "xyz" {
		t.Errorf("expected UID xyz, got %s", ctx.UID)
	}
	if ctx.TimeFrom != "now-6h" {
		t.Errorf("expected from now-6h, got %s", ctx.TimeFrom)
	}
	if ctx.TimeTo != "now" {
		t.Errorf("expected to now, got %s", ctx.TimeTo)
	}
	if ctx.Variables["server"] != "prod-01" {
		t.Errorf("expected var server=prod-01, got %s", ctx.Variables["server"])
	}
	if ctx.Variables["region"] != "us-east" {
		t.Errorf("expected var region=us-east, got %s", ctx.Variables["region"])
	}
}

func TestParseDashboardURL_GrafanaPrefix(t *testing.T) {
	ctx := ParseDashboardURL("http://localhost:8080/grafana/d/uid456/slug")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.UID != "uid456" {
		t.Errorf("expected UID uid456, got %s", ctx.UID)
	}
}

func TestParseDashboardURL_NoDashboardPath(t *testing.T) {
	ctx := ParseDashboardURL("http://localhost:3000/")
	if ctx != nil {
		t.Error("expected nil for home page URL")
	}
}

func TestParseDashboardURL_Malformed(t *testing.T) {
	ctx := ParseDashboardURL("://not-a-url")
	if ctx != nil {
		t.Error("expected nil for malformed URL")
	}
}
