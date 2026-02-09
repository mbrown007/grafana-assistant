package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/marcusz/monitoring-assistant/internal/api"
	"github.com/marcusz/monitoring-assistant/internal/mcp"
)

const (
	investigationToolTimeout          = 8 * time.Second
	investigationPreviewMaxChars      = 1200
	investigationDefaultMetricDS      = "prometheus"
	investigationDefaultLogsDS        = "loki"
	investigationDefaultFallbackLimit = 500
)

var investigationMetricsToolCandidates = []string{
	"query_prometheus",
	"query_prometheus_range",
	"query_prometheus_instant",
	"query_metrics",
}

var investigationLogsToolCandidates = []string{
	"query_loki_logs",
	"query_loki",
	"query_logs",
}

type investigationToolContract struct {
	properties map[string]struct{}
	required   map[string]struct{}
}

type investigationInvocationAttempt struct {
	Args  map[string]any `json:"args"`
	Error string         `json:"error,omitempty"`
}

type investigationSyntaxRetryInfo struct {
	Detected       bool   `json:"detected"`
	Applied        bool   `json:"applied"`
	Reason         string `json:"reason,omitempty"`
	Hint           string `json:"hint,omitempty"`
	OriginalQuery  string `json:"originalQuery,omitempty"`
	CorrectedQuery string `json:"correctedQuery,omitempty"`
}

func (m *Manager) routeInvestigationAction(ctx context.Context, action, investigationID string, payload map[string]any, reqCtx *api.DashboardContext) map[string]any {
	switch action {
	case investigationActionPlan:
		return m.routeInvestigationPlan(investigationID, payload, reqCtx)
	case investigationActionFetchMetrics:
		return m.routeInvestigationFetch(ctx, action, investigationID, payload, reqCtx, investigationMetricsToolCandidates, investigationDefaultMetricDS)
	case investigationActionFetchLogs:
		return m.routeInvestigationFetch(ctx, action, investigationID, payload, reqCtx, investigationLogsToolCandidates, investigationDefaultLogsDS)
	case investigationActionSummarize:
		return m.routeInvestigationSummarize(investigationID, payload)
	case investigationActionNextStep:
		return m.routeInvestigationNextStep(investigationID, payload)
	default:
		return map[string]any{
			"status":  "unsupported_action",
			"action":  action,
			"message": "Unsupported investigation action.",
		}
	}
}

func (m *Manager) routeInvestigationPlan(investigationID string, payload map[string]any, reqCtx *api.DashboardContext) map[string]any {
	goal := strings.TrimSpace(toString(payload["goal"]))
	if goal == "" {
		return map[string]any{
			"status":    "input_error",
			"action":    investigationActionPlan,
			"retryable": true,
			"message":   "plan.goal is required for action=plan",
		}
	}

	scope := strings.TrimSpace(toString(payload["scope"]))
	hypotheses := parseInvestigationStringList(payload["hypotheses"])
	constraints := parseInvestigationStringList(payload["constraints"])
	maxStepsRaw, _ := toInt(payload["maxSteps"])
	maxSteps := clampInvestigationMaxSteps(maxStepsRaw, 4)
	timeRange := selectInvestigationTimeRange(payload["timeRange"], reqCtx)

	steps := []string{
		"Confirm objective, scope, and impacted systems.",
		"Fetch key metrics to validate impact and timeframe.",
		"Fetch correlated logs around anomalies or alert windows.",
		"Summarize findings, confidence, and recommended next action.",
	}
	if len(hypotheses) > 0 {
		steps = append([]string{"Test the top hypothesis first using targeted metrics/log queries."}, steps...)
	}
	if len(constraints) > 0 {
		steps = append(steps, "Respect stated constraints while choosing the next investigation step.")
	}
	if len(steps) > maxSteps {
		steps = steps[:maxSteps]
	}

	response := map[string]any{
		"status":          "ok",
		"action":          investigationActionPlan,
		"route":           "internal_plan",
		"investigationId": investigationID,
		"plan": map[string]any{
			"goal":        goal,
			"scope":       scope,
			"hypotheses":  hypotheses,
			"constraints": constraints,
			"timeRange":   timeRange,
			"steps":       steps,
			"maxSteps":    maxSteps,
		},
		"nextActions": []string{
			"Use investigation__manage with action=fetch_metrics for key signal checks.",
			"Use investigation__manage with action=fetch_logs for correlated event inspection.",
		},
	}
	if investigationID == "" {
		delete(response, "investigationId")
	}
	return response
}

func (m *Manager) routeInvestigationFetch(
	ctx context.Context,
	action, investigationID string,
	payload map[string]any,
	reqCtx *api.DashboardContext,
	shortCandidates []string,
	defaultDatasourceUID string,
) map[string]any {
	query := strings.TrimSpace(toString(payload["query"]))
	if query == "" {
		return map[string]any{
			"status":    "input_error",
			"action":    action,
			"retryable": true,
			"message":   fmt.Sprintf("%s.query is required for action=%s", action, action),
		}
	}

	toolName := pickInvestigationTool(m.tools, shortCandidates...)
	if toolName == "" {
		return map[string]any{
			"status":          "tool_unavailable",
			"action":          action,
			"retryable":       true,
			"message":         fmt.Sprintf("No compatible MCP tool enabled for action=%s", action),
			"expectedToolSet": shortCandidates,
			"availableTools":  investigationToolNames(m.tools),
		}
	}

	timeRange := selectInvestigationTimeRange(payload["timeRange"], reqCtx)
	datasource := selectInvestigationDatasource(payload["datasource"], reqCtx, defaultDatasourceUID)
	contract := lookupInvestigationToolContract(m.tools, toolName)
	argAttempts := buildInvestigationArgAttempts(action, query, payload, datasource, timeRange, contract)

	timeout := m.investigationToolTimeout
	if timeout <= 0 {
		timeout = investigationToolTimeout
	}

	// A single timeout budget is shared across the initial attempts and any syntax retry pass.
	// This bounds total fetch latency and avoids retry loops extending request duration.
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	attempts := make([]investigationInvocationAttempt, 0, len(argAttempts))
	syntaxRetry := investigationSyntaxRetryInfo{}
	var (
		result  any
		used    map[string]any
		lastErr error
	)
	for _, args := range argAttempts {
		invokeResult, err := m.invokeMCPTool(callCtx, toolName, args)
		attempt := investigationInvocationAttempt{
			Args: cloneMap(args),
		}
		if err != nil {
			attempt.Error = err.Error()
			attempts = append(attempts, attempt)
			lastErr = err
			continue
		}

		attempts = append(attempts, attempt)
		result = invokeResult
		used = cloneMap(args)
		lastErr = nil
		break
	}

	if lastErr != nil {
		originalQuery := query
		if syntaxDetected, reason := isSyntaxRelatedToolError(lastErr); syntaxDetected {
			syntaxRetry.Detected = true
			syntaxRetry.Reason = reason
			syntaxRetry.OriginalQuery = originalQuery

			correctedQuery, hint := suggestSyntaxRetryQuery(action, originalQuery)
			syntaxRetry.Hint = hint
			if correctedQuery != "" && correctedQuery != originalQuery {
				syntaxRetry.Applied = true
				syntaxRetry.CorrectedQuery = correctedQuery

				retryAttempts := buildInvestigationArgAttempts(action, correctedQuery, payload, datasource, timeRange, contract)
				for _, args := range retryAttempts {
					invokeResult, err := m.invokeMCPTool(callCtx, toolName, args)
					attempt := investigationInvocationAttempt{
						Args: cloneMap(args),
					}
					if err != nil {
						attempt.Error = err.Error()
						attempts = append(attempts, attempt)
						lastErr = err
						continue
					}

					attempts = append(attempts, attempt)
					result = invokeResult
					used = cloneMap(args)
					lastErr = nil
					query = correctedQuery
					break
				}
			}
		}
	}

	if lastErr != nil {
		response := map[string]any{
			"status":         "tool_error",
			"action":         action,
			"route":          "mcp_tool",
			"tool":           toolName,
			"retryable":      true,
			"message":        "Composite fetch action failed after argument retries. Adjust query syntax or datasource and retry.",
			"lastError":      lastErr.Error(),
			"attemptDetails": attempts,
			"syntaxRetry":    map[string]any{"detected": syntaxRetry.Detected, "applied": syntaxRetry.Applied},
		}
		if syntaxRetry.Detected {
			response["message"] = "Composite fetch action failed. Syntax-related failure detected; use syntaxRetry guidance and retry once with corrected query context."
			response["syntaxRetry"] = map[string]any{
				"detected":       true,
				"applied":        syntaxRetry.Applied,
				"reason":         syntaxRetry.Reason,
				"hint":           syntaxRetry.Hint,
				"originalQuery":  syntaxRetry.OriginalQuery,
				"correctedQuery": syntaxRetry.CorrectedQuery,
			}
		}
		return response
	}

	preview := truncateForPrompt(mcp.FormatToolResult(result), investigationPreviewMaxChars)
	response := map[string]any{
		"status":          "ok",
		"action":          action,
		"route":           "mcp_tool",
		"tool":            toolName,
		"investigationId": investigationID,
		"request":         used,
		"normalized": map[string]any{
			"query":         query,
			"datasourceUid": strings.TrimSpace(toString(datasource["uid"])),
			"timeRange":     timeRange,
			"resultType":    fmt.Sprintf("%T", result),
			"resultPreview": preview,
		},
		"raw": result,
	}
	if syntaxRetry.Detected {
		response["syntaxRetry"] = map[string]any{
			"detected":       true,
			"applied":        syntaxRetry.Applied,
			"reason":         syntaxRetry.Reason,
			"hint":           syntaxRetry.Hint,
			"originalQuery":  syntaxRetry.OriginalQuery,
			"correctedQuery": syntaxRetry.CorrectedQuery,
		}
	}
	if investigationID == "" {
		delete(response, "investigationId")
	}
	return response
}

func isSyntaxRelatedToolError(err error) (bool, string) {
	if err == nil {
		return false, ""
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return false, ""
	}

	match := func(patterns ...string) bool {
		for _, pattern := range patterns {
			if strings.Contains(text, pattern) {
				return true
			}
		}
		return false
	}

	switch {
	case match("syntax error", "parse error", "cannot parse", "failed to parse", "parse failure"):
		return true, "parse_failure"
	case match("unexpected end of input", "unexpected eof", "unterminated", "unclosed", "missing )", "missing ]", "missing }"):
		return true, "unterminated_expression"
	case match("invalid query", "bad_data", "bad data", "unexpected token", "unexpected character"):
		return true, "invalid_query_syntax"
	default:
		return false, ""
	}
}

func suggestSyntaxRetryQuery(action, query string) (string, string) {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return "", "Provide a non-empty query before retrying."
	}

	original := normalized
	normalized = strings.Trim(normalized, "`")
	normalized = strings.TrimSpace(normalized)
	normalized = strings.TrimSuffix(normalized, ";")
	normalized = strings.TrimSpace(normalized)
	normalized = strings.Trim(normalized, "`")
	normalized = strings.TrimSpace(normalized)
	normalized = closeUnbalancedDelimiters(normalized)

	if action == investigationActionFetchLogs {
		trimmed := strings.TrimSpace(normalized)
		if strings.HasSuffix(trimmed, "|=") || strings.HasSuffix(trimmed, "|~") {
			normalized = trimmed + ` ""`
		}
	}

	if normalized != original {
		return normalized, "Applied a conservative syntax correction (trimmed wrappers/terminators and balanced delimiters)."
	}
	return normalized, "No safe automatic rewrite was found; verify operators, delimiters, and quoting."
}

func closeUnbalancedDelimiters(query string) string {
	// This is a conservative best-effort balancer and intentionally does not parse quoted
	// string contexts. It only appends missing closing delimiters for unmatched open tokens.
	type pair struct {
		open  rune
		close rune
	}
	pairs := []pair{
		{open: '(', close: ')'},
		{open: '[', close: ']'},
		{open: '{', close: '}'},
	}

	balances := map[rune]int{
		'(': 0,
		'[': 0,
		'{': 0,
	}
	for _, r := range query {
		switch r {
		case '(', '[', '{':
			balances[r]++
		case ')':
			if balances['('] > 0 {
				balances['(']--
			}
		case ']':
			if balances['['] > 0 {
				balances['[']--
			}
		case '}':
			if balances['{'] > 0 {
				balances['{']--
			}
		}
	}

	var b strings.Builder
	b.WriteString(query)
	for _, p := range pairs {
		for i := 0; i < balances[p.open]; i++ {
			b.WriteRune(p.close)
		}
	}
	return b.String()
}

func (m *Manager) routeInvestigationSummarize(investigationID string, payload map[string]any) map[string]any {
	findings := parseInvestigationStringList(payload["findings"])
	if len(findings) == 0 {
		return map[string]any{
			"status":    "input_error",
			"action":    investigationActionSummarize,
			"retryable": true,
			"message":   "summarize.findings must include at least one item",
		}
	}

	severity := strings.ToLower(strings.TrimSpace(toString(payload["severity"])))
	switch severity {
	case "", "info", "warning", "critical":
	default:
		severity = "info"
	}
	if severity == "" {
		severity = "info"
	}

	objective := strings.TrimSpace(toString(payload["objective"]))
	includeEvidence := toBool(payload["includeEvidence"])

	summaryText := strings.Join(limitStrings(findings, 3), " | ")
	response := map[string]any{
		"status":          "ok",
		"action":          investigationActionSummarize,
		"route":           "internal_summary",
		"investigationId": investigationID,
		"summary": map[string]any{
			"objective":       objective,
			"severity":        severity,
			"findingsCount":   len(findings),
			"findings":        findings,
			"includeEvidence": includeEvidence,
			"brief":           summaryText,
		},
	}
	if investigationID == "" {
		delete(response, "investigationId")
	}
	return response
}

func (m *Manager) routeInvestigationNextStep(investigationID string, payload map[string]any) map[string]any {
	state := strings.TrimSpace(toString(payload["currentState"]))
	if state == "" {
		return map[string]any{
			"status":    "input_error",
			"action":    investigationActionNextStep,
			"retryable": true,
			"message":   "next_step.currentState is required",
		}
	}

	options := parseInvestigationStringList(payload["options"])
	preferred := strings.TrimSpace(toString(payload["preferredOption"]))
	needsApproval := toBool(payload["needsApproval"])
	blockers := parseInvestigationStringList(payload["blockers"])

	recommended := preferred
	if recommended == "" && len(options) > 0 {
		recommended = options[0]
	}
	if recommended == "" {
		recommended = "collect_additional_evidence"
	}

	response := map[string]any{
		"status":          "ok",
		"action":          investigationActionNextStep,
		"route":           "internal_next_step",
		"investigationId": investigationID,
		"decision": map[string]any{
			"currentState":        state,
			"recommendedOption":   recommended,
			"providedOptions":     options,
			"preferredOption":     preferred,
			"needsApproval":       needsApproval,
			"approvalRequired":    needsApproval,
			"blockers":            blockers,
			"hasBlockingConcerns": len(blockers) > 0,
		},
	}
	if investigationID == "" {
		delete(response, "investigationId")
	}
	return response
}

func pickInvestigationTool(tools []mcp.Tool, shortCandidates ...string) string {
	for _, shortName := range shortCandidates {
		if tool := pickSchemaTool(tools, shortName); tool != "" {
			return tool
		}
	}
	return ""
}

func lookupInvestigationToolContract(tools []mcp.Tool, toolName string) investigationToolContract {
	for _, t := range tools {
		if t.Name != toolName {
			continue
		}

		contract := investigationToolContract{
			properties: map[string]struct{}{},
			required:   map[string]struct{}{},
		}
		props, _ := t.InputSchema["properties"].(map[string]any)
		for key := range props {
			contract.properties[key] = struct{}{}
		}
		if required, ok := t.InputSchema["required"].([]any); ok {
			for _, raw := range required {
				if key, ok := raw.(string); ok && strings.TrimSpace(key) != "" {
					contract.required[key] = struct{}{}
				}
			}
		}
		return contract
	}
	return investigationToolContract{}
}

func buildInvestigationArgAttempts(
	action, query string,
	payload map[string]any,
	datasource map[string]any,
	timeRange map[string]string,
	contract investigationToolContract,
) []map[string]any {
	rich := map[string]any{}
	setIfNotEmpty(rich, "query", query)
	setIfNotEmpty(rich, "expr", query)
	if action == investigationActionFetchMetrics {
		setIfNotEmpty(rich, "promql", query)
	} else {
		setIfNotEmpty(rich, "logql", query)
	}

	if uid := strings.TrimSpace(toString(datasource["uid"])); uid != "" {
		rich["datasourceUid"] = uid
		rich["datasource_uid"] = uid
	}
	if len(datasource) > 0 {
		rich["datasource"] = datasource
	}

	if from := strings.TrimSpace(timeRange["from"]); from != "" {
		rich["from"] = from
		rich["start"] = from
	}
	if to := strings.TrimSpace(timeRange["to"]); to != "" {
		rich["to"] = to
		rich["end"] = to
	}
	if len(timeRange) > 0 {
		rich["timeRange"] = timeRange
		rich["range"] = timeRange
	}

	if limit, ok := toInt(payload["limit"]); ok && limit > 0 {
		rich["limit"] = limit
	}
	if step := strings.TrimSpace(toString(payload["step"])); step != "" {
		rich["step"] = step
	}
	if _, exists := payload["instant"]; exists {
		rich["instant"] = toBool(payload["instant"])
	}
	if direction := strings.ToLower(strings.TrimSpace(toString(payload["direction"]))); direction == "forward" || direction == "backward" {
		rich["direction"] = direction
	}

	attempts := make([]map[string]any, 0, 4)

	if len(contract.properties) > 0 {
		filtered := map[string]any{}
		for key := range contract.properties {
			if val, ok := rich[key]; ok {
				filtered[key] = val
			}
		}
		ensureInvestigationRequiredArgs(filtered, query, datasource, timeRange, contract)
		if len(filtered) > 0 {
			attempts = append(attempts, filtered)
		}
	}

	primary := map[string]any{}
	primary["query"] = query
	primary["datasource"] = datasource
	if uid := strings.TrimSpace(toString(datasource["uid"])); uid != "" {
		primary["datasourceUid"] = uid
	}
	if len(timeRange) > 0 {
		primary["timeRange"] = timeRange
	}
	if from := strings.TrimSpace(timeRange["from"]); from != "" {
		primary["from"] = from
	}
	if to := strings.TrimSpace(timeRange["to"]); to != "" {
		primary["to"] = to
	}
	if limit, ok := toInt(payload["limit"]); ok && limit > 0 {
		primary["limit"] = limit
	}
	if step := strings.TrimSpace(toString(payload["step"])); step != "" {
		primary["step"] = step
	}
	if _, exists := payload["instant"]; exists {
		primary["instant"] = toBool(payload["instant"])
	}
	if direction := strings.ToLower(strings.TrimSpace(toString(payload["direction"]))); direction == "forward" || direction == "backward" {
		primary["direction"] = direction
	}
	attempts = append(attempts, primary)

	fallbackExpr := cloneMap(primary)
	delete(fallbackExpr, "query")
	fallbackExpr["expr"] = query
	attempts = append(attempts, fallbackExpr)

	if action == investigationActionFetchLogs {
		fallbackLogQL := cloneMap(primary)
		delete(fallbackLogQL, "query")
		fallbackLogQL["logql"] = query
		attempts = append(attempts, fallbackLogQL)
	}
	if action == investigationActionFetchMetrics {
		fallbackPromQL := cloneMap(primary)
		delete(fallbackPromQL, "query")
		fallbackPromQL["promql"] = query
		attempts = append(attempts, fallbackPromQL)
	}

	return dedupeInvestigationArgs(attempts)
}

func ensureInvestigationRequiredArgs(
	args map[string]any,
	query string,
	datasource map[string]any,
	timeRange map[string]string,
	contract investigationToolContract,
) {
	if len(contract.required) == 0 {
		return
	}
	for key := range contract.required {
		if _, exists := args[key]; exists {
			continue
		}
		switch key {
		case "query", "expr", "promql", "logql":
			args[key] = query
		case "datasourceUid", "datasource_uid":
			if uid := strings.TrimSpace(toString(datasource["uid"])); uid != "" {
				args[key] = uid
			}
		case "datasource":
			if len(datasource) > 0 {
				args[key] = datasource
			}
		case "from", "start":
			if from := strings.TrimSpace(timeRange["from"]); from != "" {
				args[key] = from
			}
		case "to", "end":
			if to := strings.TrimSpace(timeRange["to"]); to != "" {
				args[key] = to
			}
		case "timeRange", "range":
			if len(timeRange) > 0 {
				args[key] = timeRange
			}
		case "limit":
			args[key] = investigationDefaultFallbackLimit
		}
	}
}

func selectInvestigationDatasource(raw any, reqCtx *api.DashboardContext, fallbackUID string) map[string]any {
	if ds, ok := raw.(map[string]any); ok {
		uid := strings.TrimSpace(toString(ds["uid"]))
		if uid != "" {
			if _, exists := ds["type"]; !exists || strings.TrimSpace(toString(ds["type"])) == "" {
				ds["type"] = guessDatasourceType(uid)
			}
			return cloneMap(ds)
		}
	}

	if reqCtx != nil && reqCtx.Explore != nil && strings.TrimSpace(reqCtx.Explore.Datasource) != "" {
		uid := strings.TrimSpace(reqCtx.Explore.Datasource)
		return map[string]any{
			"uid":  uid,
			"type": guessDatasourceType(uid),
		}
	}

	uid := strings.TrimSpace(fallbackUID)
	if uid == "" {
		uid = investigationDefaultMetricDS
	}
	return map[string]any{
		"uid":  uid,
		"type": guessDatasourceType(uid),
	}
}

func selectInvestigationTimeRange(raw any, reqCtx *api.DashboardContext) map[string]string {
	timeRange := map[string]string{}
	if tr, ok := raw.(map[string]any); ok {
		from := strings.TrimSpace(toString(tr["from"]))
		to := strings.TrimSpace(toString(tr["to"]))
		if from != "" {
			timeRange["from"] = from
		}
		if to != "" {
			timeRange["to"] = to
		}
	}

	if len(timeRange) == 0 && reqCtx != nil && reqCtx.TimeRange != nil {
		if from := strings.TrimSpace(reqCtx.TimeRange["from"]); from != "" {
			timeRange["from"] = from
		}
		if to := strings.TrimSpace(reqCtx.TimeRange["to"]); to != "" {
			timeRange["to"] = to
		}
	}

	if len(timeRange) == 0 {
		timeRange["from"] = "now-1h"
		timeRange["to"] = "now"
	}
	if timeRange["to"] == "" {
		timeRange["to"] = "now"
	}
	if timeRange["from"] == "" {
		timeRange["from"] = "now-1h"
	}

	return timeRange
}

func dedupeInvestigationArgs(in []map[string]any) []map[string]any {
	seen := map[string]struct{}{}
	out := make([]map[string]any, 0, len(in))
	for _, args := range in {
		clean := cleanArgs(args)
		if len(clean) == 0 {
			continue
		}
		keyBytes, err := json.Marshal(clean)
		if err != nil {
			continue
		}
		key := string(keyBytes)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func cleanArgs(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		switch raw := v.(type) {
		case string:
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				continue
			}
			out[k] = trimmed
		case map[string]string:
			if len(raw) == 0 {
				continue
			}
			cloned := make(map[string]string, len(raw))
			for key, value := range raw {
				cloned[key] = value
			}
			out[k] = cloned
		case map[string]any:
			if len(raw) == 0 {
				continue
			}
			out[k] = cloneMap(raw)
		default:
			if v == nil {
				continue
			}
			out[k] = v
		}
	}
	return out
}

func setIfNotEmpty(target map[string]any, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	target[key] = value
}

func toInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(trimmed, "%d", &parsed); err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func clampInvestigationMaxSteps(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value > 10 {
		return 10
	}
	return value
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCloneAny(v)
	}
	return out
}

func deepCloneAny(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMap(v)
	case map[string]string:
		cloned := make(map[string]string, len(v))
		for key, item := range v {
			cloned[key] = item
		}
		return cloned
	case []any:
		cloned := make([]any, len(v))
		for i, item := range v {
			cloned[i] = deepCloneAny(item)
		}
		return cloned
	case []string:
		cloned := make([]string, len(v))
		copy(cloned, v)
		return cloned
	default:
		return value
	}
}

func parseInvestigationStringList(raw any) []string {
	switch value := raw.(type) {
	case nil:
		return nil
	case []string:
		return dedupeStrings(value)
	case []any:
		return dedupeStrings(anySliceToStrings(value))
	default:
		return nil
	}
}

func investigationToolNames(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	return names
}
