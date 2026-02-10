package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/brownster/grafana-assistant/internal/api"
	"github.com/brownster/grafana-assistant/internal/mcp"
)

const (
	schemaToolTimeout          = 4 * time.Second
	schemaDatasourceMaxRows    = 8
	schemaMetricPreviewMaxRows = 20
	schemaLabelPreviewMaxRows  = 25
)

type schemaTargets struct {
	prometheus bool
	loki       bool
}

type schemaToolSet struct {
	listDatasources          string
	listPrometheusMetricName string
	listPrometheusLabelName  string
	listLokiLabelName        string
}

type schemaDatasource struct {
	UID       string
	Name      string
	Type      string
	IsDefault bool
}

func shouldRouteSchemaFirst(intent IntentClass) bool {
	return intent == IntentLiveData || intent == IntentQueryHelp
}

func selectSchemaTargets(intent IntentClass, message string, reqCtx *api.DashboardContext) schemaTargets {
	targets := schemaTargets{}
	if !shouldRouteSchemaFirst(intent) {
		return targets
	}

	text := normalizeIntentText(message)

	if containsAnyNormalized(text,
		"logql",
		"loki",
		"logs",
		" logs",
		" log ",
		"log stream",
		"trace id",
		"|=",
		"|~",
	) {
		targets.loki = true
	}

	if containsAnyNormalized(text,
		"promql",
		"prometheus",
		"metric",
		"metrics",
		"histogram_quantile",
		"rate(",
		"increase(",
		"sum by",
		"count by",
		"p95",
		"p99",
		"cpu",
		"memory",
		"latency",
		"error rate",
	) {
		targets.prometheus = true
	}

	if reqCtx != nil && reqCtx.Explore != nil {
		ds := strings.ToLower(strings.TrimSpace(reqCtx.Explore.Datasource))
		if strings.Contains(ds, "loki") {
			targets.loki = true
		}
		if strings.Contains(ds, "prom") {
			targets.prometheus = true
		}
		for _, q := range reqCtx.Explore.Queries {
			qLower := strings.ToLower(strings.TrimSpace(q))
			if looksLogQLExpression(qLower) {
				targets.loki = true
			}
			if looksPromQLExpression(qLower) {
				targets.prometheus = true
			}
		}
	}

	if !targets.prometheus && !targets.loki {
		// Default to Prometheus when we cannot infer backend.
		targets.prometheus = true
	}

	return targets
}

func looksLogQLExpression(q string) bool {
	if q == "" {
		return false
	}
	return strings.HasPrefix(q, "{") || strings.Contains(q, "|=") || strings.Contains(q, "|~")
}

func looksPromQLExpression(q string) bool {
	if q == "" {
		return false
	}
	if strings.Contains(q, "{") && strings.Contains(q, "}") {
		return true
	}
	return containsAnyNormalized(q,
		"rate(",
		"increase(",
		"histogram_quantile(",
		"sum by",
		"count by",
	)
}

func resolveSchemaTools(tools []mcp.Tool) schemaToolSet {
	return schemaToolSet{
		listDatasources:          pickSchemaTool(tools, "list_datasources"),
		listPrometheusMetricName: pickSchemaTool(tools, "list_prometheus_metric_names"),
		listPrometheusLabelName:  pickSchemaTool(tools, "list_prometheus_label_names"),
		listLokiLabelName:        pickSchemaTool(tools, "list_loki_label_names"),
	}
}

func pickSchemaTool(tools []mcp.Tool, shortName string) string {
	best := ""
	for _, t := range tools {
		if toolShortName(t.Name) != shortName {
			continue
		}
		if best == "" {
			best = t.Name
			continue
		}
		if strings.HasPrefix(t.Name, "grafana__") && !strings.HasPrefix(best, "grafana__") {
			best = t.Name
		}
	}
	return best
}

func orderMCPToolsForIntent(tools []mcp.Tool, intent IntentClass) []mcp.Tool {
	if len(tools) == 0 {
		return nil
	}

	ordered := make([]mcp.Tool, 0, len(tools))
	used := make([]bool, len(tools))
	appendMatching := func(match func(name string) bool) {
		for i, t := range tools {
			if used[i] || !match(t.Name) {
				continue
			}
			ordered = append(ordered, t)
			used[i] = true
		}
	}

	switch {
	case shouldRouteSchemaFirst(intent):
		appendMatching(isSchemaTool)
	case shouldRouteDashboardLookup(intent):
		appendMatching(isDashboardMetadataTool)
		appendMatching(isSemanticTool)
	case shouldRouteDocsKB(intent):
		appendMatching(isDocsRetrievalTool)
		appendMatching(isSemanticTool)
	}

	for i, t := range tools {
		if used[i] {
			continue
		}
		ordered = append(ordered, t)
	}

	return ordered
}

func isSchemaTool(name string) bool {
	switch toolShortName(name) {
	case "list_datasources", "list_prometheus_metric_names", "list_prometheus_label_names", "list_loki_label_names":
		return true
	default:
		return false
	}
}

func (m *Manager) buildSchemaContext(ctx context.Context, intent IntentResult, userMsg string, reqCtx *api.DashboardContext) string {
	if !shouldRouteSchemaFirst(intent.Label) {
		return ""
	}

	toolSet := resolveSchemaTools(m.tools)
	if toolSet.listDatasources == "" {
		slog.InfoContext(ctx, "schema-first routing skipped: list_datasources not available",
			"event", "schema_routing",
			"intent_label", intent.Label,
		)
		return ""
	}

	targets := selectSchemaTargets(intent.Label, userMsg, reqCtx)

	datasourceResult, err := m.invokeSchemaTool(ctx, toolSet.listDatasources, map[string]any{})
	if err != nil {
		slog.WarnContext(ctx, "schema-first datasource lookup failed",
			"event", "schema_routing",
			"tool", toolSet.listDatasources,
			"error", err,
		)
		return ""
	}

	datasources := parseDatasourceSummaries(datasourceResult)
	if len(datasources) == 0 {
		slog.WarnContext(ctx, "schema-first routing found no parseable datasources",
			"event", "schema_routing",
			"tool", toolSet.listDatasources,
		)
		return ""
	}
	sortSchemaDatasources(datasources)

	var b strings.Builder
	b.WriteString("Live schema snapshot (best effort). Prefer these datasource UIDs, metric names, and label keys when writing PromQL/LogQL queries.\n")
	b.WriteString("Datasources:\n")
	for _, ds := range limitDatasources(datasources, schemaDatasourceMaxRows) {
		display := ds.Name
		if strings.TrimSpace(display) == "" {
			display = ds.UID
		}
		dflt := ""
		if ds.IsDefault {
			dflt = " default"
		}
		b.WriteString(fmt.Sprintf("- `%s` (%s) uid=`%s`%s\n", display, ds.Type, ds.UID, dflt))
	}

	var promDS schemaDatasource
	var promFound bool
	if targets.prometheus {
		promDS, promFound = pickDatasourceByType(datasources, "prometheus")
	}

	var lokiDS schemaDatasource
	var lokiFound bool
	if targets.loki {
		lokiDS, lokiFound = pickDatasourceByType(datasources, "loki")
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex

		promMetrics    []string
		promMetricsErr error
		promLabels     []string
		promLabelsErr  error
		lokiLabels     []string
		lokiLabelsErr  error
	)

	if targets.prometheus && promFound && toolSet.listPrometheusMetricName != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics, err := m.invokeSchemaStringList(ctx, toolSet.listPrometheusMetricName, map[string]any{
				"datasourceUid": promDS.UID,
				"limit":         schemaMetricPreviewMaxRows,
				"page":          1,
			})
			mu.Lock()
			promMetrics = metrics
			promMetricsErr = err
			mu.Unlock()
		}()
	}

	if targets.prometheus && promFound && toolSet.listPrometheusLabelName != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			labels, err := m.invokeSchemaStringList(ctx, toolSet.listPrometheusLabelName, map[string]any{
				"datasourceUid": promDS.UID,
				"limit":         schemaLabelPreviewMaxRows,
			})
			mu.Lock()
			promLabels = labels
			promLabelsErr = err
			mu.Unlock()
		}()
	}

	if targets.loki && lokiFound && toolSet.listLokiLabelName != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			labels, err := m.invokeSchemaStringList(ctx, toolSet.listLokiLabelName, map[string]any{
				"datasourceUid": lokiDS.UID,
			})
			mu.Lock()
			lokiLabels = labels
			lokiLabelsErr = err
			mu.Unlock()
		}()
	}

	wg.Wait()

	if targets.prometheus {
		if !promFound {
			b.WriteString("Prometheus schema unavailable (no Prometheus datasource discovered).\n")
		} else {
			b.WriteString(fmt.Sprintf("Prometheus schema (uid=`%s`):\n", promDS.UID))
			if toolSet.listPrometheusMetricName != "" {
				if promMetricsErr != nil {
					slog.WarnContext(ctx, "schema-first metric-name lookup failed",
						"event", "schema_routing",
						"tool", toolSet.listPrometheusMetricName,
						"datasource_uid", promDS.UID,
						"error", promMetricsErr,
					)
				} else if len(promMetrics) > 0 {
					b.WriteString("- metric names (sample): " + strings.Join(limitStrings(promMetrics, schemaMetricPreviewMaxRows), ", ") + "\n")
				}
			}
			if toolSet.listPrometheusLabelName != "" {
				if promLabelsErr != nil {
					slog.WarnContext(ctx, "schema-first label-name lookup failed",
						"event", "schema_routing",
						"tool", toolSet.listPrometheusLabelName,
						"datasource_uid", promDS.UID,
						"error", promLabelsErr,
					)
				} else if len(promLabels) > 0 {
					b.WriteString("- label names (sample): " + strings.Join(limitStrings(promLabels, schemaLabelPreviewMaxRows), ", ") + "\n")
				}
			}
		}
	}

	if targets.loki {
		if !lokiFound {
			b.WriteString("Loki schema unavailable (no Loki datasource discovered).\n")
		} else {
			b.WriteString(fmt.Sprintf("Loki schema (uid=`%s`):\n", lokiDS.UID))
			if toolSet.listLokiLabelName != "" {
				if lokiLabelsErr != nil {
					slog.WarnContext(ctx, "schema-first loki label-name lookup failed",
						"event", "schema_routing",
						"tool", toolSet.listLokiLabelName,
						"datasource_uid", lokiDS.UID,
						"error", lokiLabelsErr,
					)
				} else if len(lokiLabels) > 0 {
					b.WriteString("- label names (sample): " + strings.Join(limitStrings(lokiLabels, schemaLabelPreviewMaxRows), ", ") + "\n")
				}
			}
		}
	}

	contextText := strings.TrimSpace(b.String())
	slog.InfoContext(ctx, "schema-first routing prepared context",
		"event", "schema_routing",
		"intent_label", intent.Label,
		"datasource_count", len(datasources),
		"targets_prometheus", targets.prometheus,
		"targets_loki", targets.loki,
		"context_chars", len(contextText),
	)
	return contextText
}

func (m *Manager) invokeSchemaTool(ctx context.Context, toolName string, args map[string]any) (any, error) {
	if toolName == "" {
		return nil, fmt.Errorf("schema tool name is empty")
	}
	if args == nil {
		args = map[string]any{}
	}
	callCtx, cancel := context.WithTimeout(ctx, schemaToolTimeout)
	defer cancel()
	return m.invokeMCPTool(callCtx, toolName, args)
}

func (m *Manager) invokeSchemaStringList(ctx context.Context, toolName string, args map[string]any) ([]string, error) {
	result, err := m.invokeSchemaTool(ctx, toolName, args)
	if err != nil {
		return nil, err
	}
	return parseStringList(result), nil
}

func parseDatasourceSummaries(result any) []schemaDatasource {
	normalized, ok := normalizeToolResult(result)
	if !ok {
		return nil
	}

	decoded, ok := coerceJSONValue(normalized)
	if !ok {
		return nil
	}

	entries := extractDatasourceEntries(decoded)
	if len(entries) == 0 {
		return nil
	}

	out := make([]schemaDatasource, 0, len(entries))
	for _, raw := range entries {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ds, ok := parseDatasourceEntry(item)
		if !ok {
			continue
		}
		out = append(out, ds)
	}
	return out
}

func extractDatasourceEntries(decoded any) []any {
	switch v := decoded.(type) {
	case []any:
		return v
	case map[string]any:
		for _, key := range []string{"datasources", "items", "result", "data"} {
			if raw, exists := v[key]; exists {
				if arr, ok := raw.([]any); ok {
					return arr
				}
			}
		}
	}
	return nil
}

func parseDatasourceEntry(item map[string]any) (schemaDatasource, bool) {
	uid := strings.TrimSpace(toString(item["uid"]))
	if uid == "" {
		return schemaDatasource{}, false
	}

	dsType := strings.TrimSpace(strings.ToLower(toString(item["type"])))
	name := strings.TrimSpace(toString(item["name"]))
	if name == "" {
		name = uid
	}

	return schemaDatasource{
		UID:       uid,
		Name:      name,
		Type:      dsType,
		IsDefault: toBool(item["isDefault"]),
	}, true
}

func sortSchemaDatasources(datasources []schemaDatasource) {
	sort.SliceStable(datasources, func(i, j int) bool {
		if datasources[i].IsDefault != datasources[j].IsDefault {
			return datasources[i].IsDefault
		}
		if datasources[i].Type != datasources[j].Type {
			return datasources[i].Type < datasources[j].Type
		}
		if datasources[i].Name != datasources[j].Name {
			return datasources[i].Name < datasources[j].Name
		}
		return datasources[i].UID < datasources[j].UID
	})
}

func limitDatasources(datasources []schemaDatasource, limit int) []schemaDatasource {
	if len(datasources) <= limit {
		return datasources
	}
	out := make([]schemaDatasource, 0, limit)
	out = append(out, datasources[:limit]...)
	return out
}

func pickDatasourceByType(datasources []schemaDatasource, typeHint string) (schemaDatasource, bool) {
	typeHint = strings.ToLower(strings.TrimSpace(typeHint))
	for _, ds := range datasources {
		if strings.Contains(strings.ToLower(ds.Type), typeHint) {
			return ds, true
		}
	}
	return schemaDatasource{}, false
}

func parseStringList(result any) []string {
	normalized, ok := normalizeToolResult(result)
	if !ok {
		return nil
	}

	switch v := normalized.(type) {
	case []string:
		return dedupeStrings(v)
	case []any:
		return dedupeStrings(anySliceToStrings(v))
	case string:
		return dedupeStrings(splitLooseList(v))
	case map[string]any:
		for _, key := range []string{"values", "items", "result", "data", "labels", "metrics"} {
			raw, exists := v[key]
			if !exists {
				continue
			}
			if arr, ok := raw.([]any); ok {
				return dedupeStrings(anySliceToStrings(arr))
			}
		}
	}

	decoded, ok := coerceJSONValue(normalized)
	if !ok {
		return nil
	}
	if arr, ok := decoded.([]any); ok {
		return dedupeStrings(anySliceToStrings(arr))
	}
	if m, ok := decoded.(map[string]any); ok {
		for _, key := range []string{"values", "items", "result", "data", "labels", "metrics"} {
			raw, exists := m[key]
			if !exists {
				continue
			}
			if arr, ok := raw.([]any); ok {
				return dedupeStrings(anySliceToStrings(arr))
			}
		}
	}

	return nil
}

func normalizeToolResult(result any) (any, bool) {
	if result == nil {
		return nil, false
	}

	if s, ok := result.(string); ok {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			return nil, false
		}
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded, true
		}
		return trimmed, true
	}

	return result, true
}

func coerceJSONValue(value any) (any, bool) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, false
	}
	return out, true
}

func splitLooseList(s string) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}

	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil
	}

	var parts []string
	switch {
	case strings.Contains(trimmed, "\n"):
		parts = strings.Split(trimmed, "\n")
	case strings.Contains(trimmed, ","):
		parts = strings.Split(trimmed, ",")
	default:
		parts = strings.Fields(trimmed)
	}

	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.Trim(p, "'\"`"))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func anySliceToStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		s := strings.TrimSpace(toString(v))
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		clean := strings.TrimSpace(v)
		if clean == "" {
			continue
		}
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	out := make([]string, 0, limit)
	out = append(out, values[:limit]...)
	return out
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case json.Number:
		return v.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func toBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}
