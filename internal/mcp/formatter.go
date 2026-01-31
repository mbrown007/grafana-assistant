package mcp

import (
	"encoding/json"
	"fmt"
)

// FormatToolResult formats an MCP tool result for inclusion in LLM prompts.
func FormatToolResult(result any) string {
	if result == nil {
		return "No result returned"
	}

	switch v := result.(type) {
	case string:
		return v
	case map[string]any, []any:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}
