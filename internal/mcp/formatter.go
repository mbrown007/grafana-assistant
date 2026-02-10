package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	formatterPreviewTextLimit = 220
	// Keep model-context machine details compact; UI still receives full raw payload.
	// Aligned with investigation preview truncation budget to avoid oversized tool context.
	formatterDetailCharLimit = 1200
)

// FormatToolResult formats an MCP tool result for inclusion in LLM prompts.
func FormatToolResult(result any) string {
	if result == nil {
		return "Summary: No result returned."
	}

	normalized := normalizeToolResultValue(result)

	if domainSummary, ok := tryDomainSummary(normalized); ok {
		details := formatToolResultDetails(normalized)
		if details == "" {
			return "Summary: " + domainSummary
		}
		return "Summary: " + domainSummary + "\nMachine details:\n" + details
	}

	summary := buildToolResultSummary(normalized)
	details := formatToolResultDetails(normalized)

	if details == "" {
		return "Summary: " + summary
	}
	return "Summary: " + summary + "\nMachine details:\n" + details
}

func normalizeToolResultValue(result any) any {
	switch v := result.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return ""
		}
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
		return trimmed
	default:
		return result
	}
}

func buildToolResultSummary(value any) string {
	switch v := value.(type) {
	case nil:
		return "No result returned."
	case string:
		if strings.TrimSpace(v) == "" {
			return "Tool returned an empty string."
		}
		return "Tool returned text: " + quoteIfNeeded(truncateForSummary(v, formatterPreviewTextLimit)) + "."
	case map[string]any:
		return summarizeObject(v)
	case []any:
		return summarizeArray(v)
	default:
		return "Tool returned value " + quoteIfNeeded(fmt.Sprintf("%v", v)) + "."
	}
}

func summarizeObject(obj map[string]any) string {
	if len(obj) == 0 {
		return "Tool returned an empty object."
	}

	parts := make([]string, 0, 6)

	if status := toStringValue(obj["status"]); status != "" {
		parts = append(parts, "status="+status)
	}
	if action := toStringValue(obj["action"]); action != "" {
		parts = append(parts, "action="+action)
	}
	if tool := toStringValue(obj["tool"]); tool != "" {
		parts = append(parts, "tool="+tool)
	}
	if route := toStringValue(obj["route"]); route != "" {
		parts = append(parts, "route="+route)
	}

	if message := toStringValue(obj["message"]); message != "" {
		parts = append(parts, "message="+quoteIfNeeded(truncateForSummary(message, 140)))
	}

	for _, key := range []string{"attemptDetails", "findings", "rows", "series", "alerts", "items"} {
		if count := countCollection(obj[key]); count >= 0 {
			parts = append(parts, key+"="+strconv.Itoa(count))
		}
	}

	keys := sortedKeys(obj)
	keyPreview := keys
	if len(keyPreview) > 8 {
		keyPreview = keyPreview[:8]
	}

	if len(parts) == 0 {
		return fmt.Sprintf("Tool returned object with keys: %s.", strings.Join(keyPreview, ", "))
	}
	return fmt.Sprintf("Tool returned object (%s) with keys: %s.", strings.Join(parts, ", "), strings.Join(keyPreview, ", "))
}

func summarizeArray(arr []any) string {
	count := len(arr)
	if count == 0 {
		return "Tool returned an empty list."
	}

	typeCounts := map[string]int{}
	for _, item := range arr {
		typeCounts[valueTypeLabel(item)]++
	}
	typeSummary := summarizeTypeCounts(typeCounts, 4)
	return fmt.Sprintf("Tool returned list with %d item(s); item types: %s.", count, typeSummary)
}

func valueTypeLabel(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "list"
	case string:
		return "string"
	case float64, float32, int, int64, int32, int16, int8:
		return "number"
	case bool:
		return "boolean"
	default:
		return "value"
	}
}

func summarizeTypeCounts(typeCounts map[string]int, limit int) string {
	type entry struct {
		typ   string
		count int
	}
	entries := make([]entry, 0, len(typeCounts))
	for typ, count := range typeCounts {
		entries = append(entries, entry{typ: typ, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].typ < entries[j].typ
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}

	parts := make([]string, 0, len(entries))
	for _, item := range entries {
		parts = append(parts, item.typ+"="+strconv.Itoa(item.count))
	}
	return strings.Join(parts, ", ")
}

func formatToolResultDetails(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		if strings.TrimSpace(v) == "" {
			return ""
		}
		return truncateWithSuffix(v, formatterDetailCharLimit)
	case map[string]any, []any:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return truncateWithSuffix(fmt.Sprintf("%v", v), formatterDetailCharLimit)
		}
		return truncateWithSuffix(string(b), formatterDetailCharLimit)
	default:
		return truncateWithSuffix(fmt.Sprintf("%v", v), formatterDetailCharLimit)
	}
}

func truncateForSummary(s string, max int) string {
	trimmed := strings.TrimSpace(s)
	runes := []rune(trimmed)
	if len(runes) <= max {
		return trimmed
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func truncateWithSuffix(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	suffix := "\n... [truncated]"
	suffixRunes := []rune(suffix)
	if max <= len(suffixRunes) {
		return string(runes[:max])
	}
	return string(runes[:max-len(suffixRunes)]) + suffix
}

func quoteIfNeeded(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return `""`
	}
	return `"` + strings.ReplaceAll(trimmed, `"`, `'`) + `"`
}

func countCollection(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	case []string:
		return len(v)
	case map[string]any:
		return len(v)
	default:
		return -1
	}
}

func sortedKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toStringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}
