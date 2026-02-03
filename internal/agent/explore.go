package agent

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

type exploreQuery struct {
	RefID        string         `json:"refId,omitempty"`
	Expr         string         `json:"expr,omitempty"`
	Range        *bool          `json:"range,omitempty"`
	Instant      *bool          `json:"instant,omitempty"`
	LegendFormat string         `json:"legendFormat,omitempty"`
	EditorMode   string         `json:"editorMode,omitempty"`
	Datasource   map[string]any `json:"datasource,omitempty"`
}

type exploreOpenArgs struct {
	Query      string            `json:"query,omitempty"`
	Queries    []exploreQuery    `json:"queries,omitempty"`
	Datasource map[string]any    `json:"datasource,omitempty"`
	TimeRange  map[string]string `json:"timeRange,omitempty"`
	OrgID      int               `json:"orgId,omitempty"`
}

func buildExploreURL(args map[string]any, reqCtx *api.DashboardContext) (string, error) {
	var parsed exploreOpenArgs
	if raw, err := json.Marshal(args); err == nil {
		_ = json.Unmarshal(raw, &parsed)
	}

	queries := parsed.Queries
	if len(queries) == 0 && parsed.Query != "" {
		queries = []exploreQuery{{RefID: "A", Expr: parsed.Query}}
	}
	if len(queries) == 0 {
		return "", fmt.Errorf("no query provided for explore")
	}

	datasource := parsed.Datasource
	if datasource == nil || datasource["uid"] == nil {
		uid := "prometheus"
		typeName := "prometheus"
		if reqCtx != nil && reqCtx.Explore != nil && reqCtx.Explore.Datasource != "" {
			uid = reqCtx.Explore.Datasource
		}
		datasource = map[string]any{"uid": uid, "type": typeName}
	}

	paneDatasource := datasource["uid"]
	if paneDatasource == nil {
		paneDatasource = "prometheus"
	}

	for i := range queries {
		if queries[i].RefID == "" {
			queries[i].RefID = string(rune('A' + i))
		}
		if queries[i].EditorMode == "" {
			queries[i].EditorMode = "code"
		}
		if queries[i].LegendFormat == "" {
			queries[i].LegendFormat = "__auto"
		}
		if queries[i].Datasource == nil {
			queries[i].Datasource = datasource
		}
	}

	from := "now-1h"
	to := "now"
	if parsed.TimeRange != nil {
		if v := parsed.TimeRange["from"]; v != "" {
			from = v
		}
		if v := parsed.TimeRange["to"]; v != "" {
			to = v
		}
	} else if reqCtx != nil && reqCtx.TimeRange != nil {
		if v := reqCtx.TimeRange["from"]; v != "" {
			from = v
		}
		if v := reqCtx.TimeRange["to"]; v != "" {
			to = v
		}
	}

	pane := map[string]any{
		"datasource": paneDatasource,
		"queries":    queries,
		"range": map[string]string{
			"from": from,
			"to":   to,
		},
	}
	panes := map[string]any{"A": pane}
	panesJSON, err := json.Marshal(panes)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("schemaVersion", "1")
	params.Set("panes", string(panesJSON))
	orgID := parsed.OrgID
	if orgID <= 0 {
		orgID = 1
	}
	params.Set("orgId", strconv.Itoa(orgID))

	return "/grafana/explore?" + params.Encode(), nil
}
