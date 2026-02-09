package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

const contextSearchMaxResults = 30

// SearchContext implements api.ContextSearcher by dispatching to the relevant MCP tool.
func (m *Manager) SearchContext(ctx context.Context, entityType api.ContextEntityType, query string) ([]api.ContextEntity, error) {
	switch entityType {
	case api.ContextEntityDatasource:
		return m.searchContextDatasources(ctx, query)
	case api.ContextEntityDashboard:
		return m.searchContextDashboards(ctx, query)
	case api.ContextEntityMetric:
		return m.searchContextMetrics(ctx, query)
	case api.ContextEntityLabel:
		return m.searchContextLabels(ctx, query)
	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

func (m *Manager) searchContextDatasources(ctx context.Context, query string) ([]api.ContextEntity, error) {
	toolSet := resolveSchemaTools(m.tools)
	if toolSet.listDatasources == "" {
		return nil, fmt.Errorf("list_datasources tool not available")
	}

	result, err := m.invokeSchemaTool(ctx, toolSet.listDatasources, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("list_datasources: %w", err)
	}

	datasources := parseDatasourceSummaries(result)
	queryLower := strings.ToLower(strings.TrimSpace(query))

	out := make([]api.ContextEntity, 0, len(datasources))
	for _, ds := range datasources {
		if queryLower != "" && !strings.Contains(strings.ToLower(ds.Name), queryLower) && !strings.Contains(strings.ToLower(ds.Type), queryLower) {
			continue
		}
		out = append(out, api.ContextEntity{
			Type:        api.ContextEntityDatasource,
			ID:          ds.UID,
			DisplayName: ds.Name,
			Metadata:    map[string]string{"ds_type": ds.Type},
		})
		if len(out) >= contextSearchMaxResults {
			break
		}
	}
	return out, nil
}

func (m *Manager) searchContextDashboards(ctx context.Context, query string) ([]api.ContextEntity, error) {
	toolName := pickSchemaTool(m.tools, "search_dashboards")
	if toolName == "" {
		return nil, fmt.Errorf("search_dashboards tool not available")
	}

	args := map[string]any{}
	if query != "" {
		args["query"] = query
	}

	result, err := m.invokeSchemaTool(ctx, toolName, args)
	if err != nil {
		return nil, fmt.Errorf("search_dashboards: %w", err)
	}

	return parseDashboardSearchResults(result), nil
}

func parseDashboardSearchResults(result any) []api.ContextEntity {
	normalized, ok := normalizeToolResult(result)
	if !ok {
		return nil
	}

	decoded, ok := coerceJSONValue(normalized)
	if !ok {
		decoded = normalized
	}

	var items []any
	switch v := decoded.(type) {
	case []any:
		items = v
	case map[string]any:
		for _, key := range []string{"dashboards", "items", "result", "data"} {
			if raw, exists := v[key]; exists {
				if arr, ok := raw.([]any); ok {
					items = arr
					break
				}
			}
		}
	}

	out := make([]api.ContextEntity, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid := strings.TrimSpace(toString(item["uid"]))
		if uid == "" {
			continue
		}
		title := strings.TrimSpace(toString(item["title"]))
		if title == "" {
			title = uid
		}
		meta := map[string]string{}
		if folder := strings.TrimSpace(toString(item["folderTitle"])); folder != "" {
			meta["folder"] = folder
		}
		out = append(out, api.ContextEntity{
			Type:        api.ContextEntityDashboard,
			ID:          uid,
			DisplayName: title,
			Metadata:    meta,
		})
		if len(out) >= contextSearchMaxResults {
			break
		}
	}
	return out
}

func (m *Manager) searchContextMetrics(ctx context.Context, query string) ([]api.ContextEntity, error) {
	toolSet := resolveSchemaTools(m.tools)
	if toolSet.listPrometheusMetricName == "" {
		return nil, fmt.Errorf("list_prometheus_metric_names tool not available")
	}

	promDS, err := m.resolvePrometheusDatasource(ctx, toolSet)
	if err != nil {
		return nil, err
	}

	args := map[string]any{
		"datasourceUid": promDS.UID,
		"limit":         contextSearchMaxResults,
		"page":          1,
	}
	if query != "" {
		args["regex"] = query
	}

	names, err := m.invokeSchemaStringList(ctx, toolSet.listPrometheusMetricName, args)
	if err != nil {
		return nil, fmt.Errorf("list_prometheus_metric_names: %w", err)
	}

	out := make([]api.ContextEntity, 0, len(names))
	for _, name := range names {
		out = append(out, api.ContextEntity{
			Type:        api.ContextEntityMetric,
			ID:          name,
			DisplayName: name,
		})
		if len(out) >= contextSearchMaxResults {
			break
		}
	}
	return out, nil
}

func (m *Manager) searchContextLabels(ctx context.Context, query string) ([]api.ContextEntity, error) {
	toolSet := resolveSchemaTools(m.tools)
	if toolSet.listPrometheusLabelName == "" {
		return nil, fmt.Errorf("list_prometheus_label_names tool not available")
	}

	promDS, err := m.resolvePrometheusDatasource(ctx, toolSet)
	if err != nil {
		return nil, err
	}

	labels, err := m.invokeSchemaStringList(ctx, toolSet.listPrometheusLabelName, map[string]any{
		"datasourceUid": promDS.UID,
	})
	if err != nil {
		return nil, fmt.Errorf("list_prometheus_label_names: %w", err)
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	out := make([]api.ContextEntity, 0, len(labels))
	for _, label := range labels {
		if queryLower != "" && !strings.Contains(strings.ToLower(label), queryLower) {
			continue
		}
		out = append(out, api.ContextEntity{
			Type:        api.ContextEntityLabel,
			ID:          label,
			DisplayName: label,
		})
		if len(out) >= contextSearchMaxResults {
			break
		}
	}
	return out, nil
}

// resolvePrometheusDatasource discovers the Prometheus datasource UID via MCP.
func (m *Manager) resolvePrometheusDatasource(ctx context.Context, toolSet schemaToolSet) (schemaDatasource, error) {
	if toolSet.listDatasources == "" {
		return schemaDatasource{}, fmt.Errorf("list_datasources tool not available")
	}

	result, err := m.invokeSchemaTool(ctx, toolSet.listDatasources, map[string]any{})
	if err != nil {
		return schemaDatasource{}, fmt.Errorf("list_datasources: %w", err)
	}

	datasources := parseDatasourceSummaries(result)
	promDS, found := pickDatasourceByType(datasources, "prometheus")
	if !found {
		return schemaDatasource{}, fmt.Errorf("no prometheus datasource found")
	}
	return promDS, nil
}

// buildSelectedContextBlock formats user-selected entities into a text block for the LLM.
func buildSelectedContextBlock(entities []api.ContextEntity) string {
	if len(entities) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("The user explicitly selected the following entities as relevant context.\n")
	b.WriteString("IMPORTANT: Datasource UIDs are NOT dashboard UIDs — do not pass a datasource UID to dashboard tools or vice versa.\n")
	b.WriteString("- Use datasource UIDs with query tools (e.g. datasourceUid parameter in grafana__query_prometheus).\n")
	b.WriteString("- Use dashboard UIDs with dashboard tools (e.g. grafana__get_dashboard_by_uid).\n\n")
	for _, e := range entities {
		switch e.Type {
		case api.ContextEntityDatasource:
			dsType := e.Metadata["ds_type"]
			b.WriteString(fmt.Sprintf("- Datasource: %s (type=%s, datasourceUid=%q) — use with query tools\n", e.DisplayName, dsType, e.ID))
		case api.ContextEntityDashboard:
			folder := e.Metadata["folder"]
			if folder != "" {
				b.WriteString(fmt.Sprintf("- Dashboard: %s (dashboardUid=%q, folder=%s) — use with dashboard tools\n", e.DisplayName, e.ID, folder))
			} else {
				b.WriteString(fmt.Sprintf("- Dashboard: %s (dashboardUid=%q) — use with dashboard tools\n", e.DisplayName, e.ID))
			}
		case api.ContextEntityMetric:
			b.WriteString(fmt.Sprintf("- Prometheus metric: %s\n", e.ID))
		case api.ContextEntityLabel:
			b.WriteString(fmt.Sprintf("- Prometheus label: %s\n", e.ID))
		default:
			b.WriteString(fmt.Sprintf("- %s: %s (%s)\n", e.Type, e.DisplayName, e.ID))
		}
	}
	return strings.TrimSpace(b.String())
}
