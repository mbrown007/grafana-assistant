package chatops

import (
	"fmt"
	"strings"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

// FormatResponse collects stream chunks into a single markdown message
// suitable for posting to a messaging platform.
func FormatResponse(chunks []api.StreamChunk) string {
	var body strings.Builder
	var tools []toolSummary
	var hasError bool

	for _, c := range chunks {
		switch c.Type {
		case "token":
			body.WriteString(c.Message)
		case "complete":
			if body.Len() == 0 && c.Message != "" {
				body.WriteString(c.Message)
			}
		case "tool":
			if c.Arguments != nil {
				// Tool invocation — start tracking.
				tools = append(tools, toolSummary{
					name:   c.Tool,
					reason: c.Reason,
				})
			}
		case "error":
			hasError = true
			if c.Message != "" {
				body.WriteString("\n\n> :warning: **Error:** " + c.Message + "\n")
			}
		}
	}

	text := strings.TrimSpace(body.String())

	if text == "" && !hasError {
		return "_No response generated._"
	}

	if len(tools) > 0 {
		text += "\n\n---\n**Tools used:**\n"
		for _, t := range tools {
			line := fmt.Sprintf("- `%s`", t.name)
			if t.reason != "" {
				line += fmt.Sprintf(": *%s*", t.reason)
			}
			text += line + "\n"
		}
	}

	return text
}

type toolSummary struct {
	name   string
	reason string
}
