package mcp

import (
	"strings"
	"testing"
)

func TestFormatToolResult(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect string
	}{
		{"nil", nil, "No result returned"},
		{"string", "hello", "hello"},
		{"number", 42, "42"},
		{"map", map[string]any{"key": "val"}, `"key": "val"`},
		{"slice", []any{"a", "b"}, `"a"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatToolResult(tt.input)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("FormatToolResult(%v) = %q, want to contain %q", tt.input, got, tt.expect)
			}
		})
	}
}
