package agent

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brownster/grafana-assistant/internal/api"
	"github.com/brownster/grafana-assistant/pkg/kb"
)

const (
	dashboardLookupToolTimeout      = 4 * time.Second
	dashboardLookupMaxMetadataHits  = 8
	dashboardLookupMaxMetadataQuery = 4
	dashboardLookupSemanticMaxHits  = 4
	dashboardLookupSnippetMaxLength = 180
)

var uidCapturePattern = regexp.MustCompile(`(?i)\buid\s*[:=]?\s*([a-z0-9_-]{4,64})\b`)

type dashboardSearchHit struct {
	UID    string
	Title  string
	Folder string
	URL    string
	Tags   []string
}

type semanticFallbackHit struct {
	Path    string
	Title   string
	Snippet string
	Score   float64
}

func shouldRouteDashboardLookup(intent IntentClass) bool {
	return intent == IntentDashboardLookup
}

func isDashboardMetadataTool(name string) bool {
	switch toolShortName(name) {
	case "search_dashboards", "search_folders", "get_dashboard_summary", "get_dashboard_property", "get_dashboard_panel_queries", "get_dashboard_by_uid":
		return true
	default:
		return false
	}
}

func isSemanticTool(name string) bool {
	switch toolShortName(name) {
	case "search_kb_semantic":
		return true
	default:
		return false
	}
}

func (m *Manager) buildDashboardLookupContext(ctx context.Context, intent IntentResult, userMsg string, reqCtx *api.DashboardContext) (string, bool) {
	if !shouldRouteDashboardLookup(intent.Label) {
		return "", false
	}

	searchTool := pickSchemaTool(m.tools, "search_dashboards")
	semanticTool := pickSchemaTool(m.tools, "search_kb_semantic")

	queries := buildDashboardMetadataQueries(userMsg, reqCtx)
	metadataHits := make([]dashboardSearchHit, 0, dashboardLookupMaxMetadataHits)
	seenUID := make(map[string]struct{})

	if len(queries) > dashboardLookupMaxMetadataQuery {
		queries = queries[:dashboardLookupMaxMetadataQuery]
	}

	if searchTool != "" && len(queries) > 0 {
		type metadataQueryResult struct {
			index int
			hits  []dashboardSearchHit
		}
		resultCh := make(chan metadataQueryResult, len(queries))

		var wg sync.WaitGroup
		for i, q := range queries {
			wg.Add(1)
			go func(index int, query string) {
				defer wg.Done()
				result, err := m.invokeDashboardRoutingTool(ctx, searchTool, map[string]any{"query": query})
				if err != nil {
					slog.WarnContext(ctx, "dashboard metadata lookup failed",
						"event", "dashboard_lookup_routing",
						"tool", searchTool,
						"query", query,
						"error", err,
					)
					return
				}
				resultCh <- metadataQueryResult{
					index: index,
					hits:  parseDashboardSearchHits(result),
				}
			}(i, q)
		}
		wg.Wait()
		close(resultCh)

		ordered := make([][]dashboardSearchHit, len(queries))
		for res := range resultCh {
			if res.index >= 0 && res.index < len(ordered) {
				ordered[res.index] = res.hits
			}
		}

		for _, hits := range ordered {
			if len(metadataHits) >= dashboardLookupMaxMetadataHits {
				break
			}
			for _, hit := range hits {
				key := strings.TrimSpace(hit.UID)
				if key == "" {
					key = strings.ToLower(strings.TrimSpace(hit.Title))
				}
				if key == "" {
					continue
				}
				if _, exists := seenUID[key]; exists {
					continue
				}
				seenUID[key] = struct{}{}
				metadataHits = append(metadataHits, hit)
				if len(metadataHits) >= dashboardLookupMaxMetadataHits {
					break
				}
			}
		}
	}

	if len(metadataHits) > 0 {
		ctxText := formatDashboardMetadataContext(metadataHits)
		slog.InfoContext(ctx, "dashboard lookup metadata match",
			"event", "dashboard_lookup_routing",
			"intent_label", intent.Label,
			"query_count", len(queries),
			"metadata_hit_count", len(metadataHits),
			"used_semantic_fallback", false,
		)
		return ctxText, false
	}

	semanticHits := make([]semanticFallbackHit, 0, dashboardLookupSemanticMaxHits)
	if semanticTool != "" {
		result, err := m.invokeDashboardRoutingTool(ctx, semanticTool, map[string]any{
			"query": userMsg,
			"limit": dashboardLookupSemanticMaxHits,
		})
		if err != nil {
			slog.WarnContext(ctx, "dashboard semantic fallback via MCP failed",
				"event", "dashboard_lookup_routing",
				"tool", semanticTool,
				"error", err,
			)
		} else {
			semanticHits = append(semanticHits, parseSemanticFallbackHits(result)...)
		}
	}

	if len(semanticHits) == 0 {
		semanticHits = append(semanticHits, m.localDashboardSemanticFallback(ctx, userMsg)...)
	}

	if len(semanticHits) > dashboardLookupSemanticMaxHits {
		semanticHits = semanticHits[:dashboardLookupSemanticMaxHits]
	}

	if len(semanticHits) == 0 {
		slog.InfoContext(ctx, "dashboard lookup produced no metadata or semantic matches",
			"event", "dashboard_lookup_routing",
			"intent_label", intent.Label,
			"query_count", len(queries),
			"used_semantic_fallback", false,
		)
		return "", false
	}

	ctxText := formatDashboardSemanticFallbackContext(semanticHits)
	slog.InfoContext(ctx, "dashboard lookup semantic fallback used",
		"event", "dashboard_lookup_routing",
		"intent_label", intent.Label,
		"query_count", len(queries),
		"metadata_hit_count", 0,
		"semantic_hit_count", len(semanticHits),
		"used_semantic_fallback", true,
	)
	return ctxText, true
}

func (m *Manager) invokeDashboardRoutingTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if name == "" {
		return nil, fmt.Errorf("dashboard routing tool name is empty")
	}
	callCtx, cancel := context.WithTimeout(ctx, dashboardLookupToolTimeout)
	defer cancel()
	return m.invokeMCPTool(callCtx, name, args)
}

func buildDashboardMetadataQueries(message string, reqCtx *api.DashboardContext) []string {
	queries := make([]string, 0, dashboardLookupMaxMetadataQuery)
	seen := map[string]struct{}{}
	add := func(raw string) {
		q := strings.TrimSpace(raw)
		if q == "" {
			return
		}
		if len(q) > 120 {
			q = q[:120]
		}
		key := strings.ToLower(q)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		queries = append(queries, q)
	}

	msg := strings.TrimSpace(message)
	if msg == "" {
		return nil
	}

	for _, match := range uidCapturePattern.FindAllStringSubmatch(msg, -1) {
		if len(match) > 1 {
			add(match[1])
		}
	}

	if reqCtx != nil {
		add(reqCtx.UID)
		add(reqCtx.Name)
		for _, tag := range reqCtx.Tags {
			add(tag)
		}
	}

	for _, quoted := range extractQuotedSegments(msg) {
		add(quoted)
	}

	add(compactDashboardLookupQuery(msg))
	add(msg)

	if len(queries) > dashboardLookupMaxMetadataQuery {
		queries = queries[:dashboardLookupMaxMetadataQuery]
	}
	return queries
}

func extractQuotedSegments(msg string) []string {
	out := make([]string, 0)
	for _, quote := range []string{"\"", "'", "`"} {
		parts := strings.Split(msg, quote)
		for i := 1; i < len(parts); i += 2 {
			segment := strings.TrimSpace(parts[i])
			if len(segment) < 3 {
				continue
			}
			out = append(out, segment)
		}
	}
	return out
}

func compactDashboardLookupQuery(message string) string {
	text := strings.ToLower(message)
	replacer := strings.NewReplacer(
		",", " ",
		".", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"/", " ",
		"\\", " ",
		"?", " ",
	)
	text = replacer.Replace(text)

	stop := map[string]struct{}{
		"find": {}, "search": {}, "dashboard": {}, "dashboards": {}, "open": {}, "which": {}, "where": {},
		"locate": {}, "show": {}, "me": {}, "please": {}, "the": {}, "a": {}, "an": {}, "for": {}, "by": {},
		"uid": {}, "with": {}, "tag": {}, "tags": {}, "folder": {}, "panel": {}, "to": {},
	}

	tokens := strings.Fields(text)
	keep := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if len(tok) <= 1 {
			continue
		}
		if _, skip := stop[tok]; skip {
			continue
		}
		keep = append(keep, tok)
		if len(keep) >= 8 {
			break
		}
	}
	return strings.Join(keep, " ")
}

func parseDashboardSearchHits(result any) []dashboardSearchHit {
	normalized, ok := normalizeToolResult(result)
	if !ok {
		return nil
	}
	decoded, ok := coerceJSONValue(normalized)
	if !ok {
		return nil
	}

	arr, ok := decoded.([]any)
	if !ok {
		return nil
	}

	hits := make([]dashboardSearchHit, 0, len(arr))
	for _, raw := range arr {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		hit := dashboardSearchHit{
			UID:    strings.TrimSpace(toString(item["uid"])),
			Title:  strings.TrimSpace(toString(item["title"])),
			Folder: strings.TrimSpace(toString(item["folderTitle"])),
			URL:    strings.TrimSpace(toString(item["url"])),
			Tags:   parseStringList(item["tags"]),
		}
		if hit.Folder == "" {
			hit.Folder = strings.TrimSpace(toString(item["folder"]))
		}
		if hit.Title == "" && hit.UID == "" {
			continue
		}
		hit.Tags = dedupeStrings(hit.Tags)
		hits = append(hits, hit)
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Title != hits[j].Title {
			return hits[i].Title < hits[j].Title
		}
		return hits[i].UID < hits[j].UID
	})

	return hits
}

func parseSemanticFallbackHits(result any) []semanticFallbackHit {
	normalized, ok := normalizeToolResult(result)
	if !ok {
		return nil
	}
	decoded, ok := coerceJSONValue(normalized)
	if !ok {
		return nil
	}

	arr, ok := decoded.([]any)
	if !ok {
		return nil
	}

	hits := make([]semanticFallbackHit, 0, len(arr))
	for _, raw := range arr {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := strings.TrimSpace(toString(item["path"]))
		title := strings.TrimSpace(toString(item["title"]))
		snippet := strings.TrimSpace(toString(item["snippet"]))
		score, _ := toFloat(item["score"])
		if path == "" && title == "" {
			continue
		}
		if len(snippet) > dashboardLookupSnippetMaxLength {
			snippet = snippet[:dashboardLookupSnippetMaxLength] + "..."
		}
		hits = append(hits, semanticFallbackHit{
			Path:    path,
			Title:   title,
			Snippet: snippet,
			Score:   score,
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		if hits[i].Title != hits[j].Title {
			return hits[i].Title < hits[j].Title
		}
		return hits[i].Path < hits[j].Path
	})

	return hits
}

func (m *Manager) localDashboardSemanticFallback(ctx context.Context, query string) []semanticFallbackHit {
	vectorIndex := m.loadKBVectorIndex()
	if vectorIndex == nil || m.kbEmbedder == nil {
		return nil
	}

	emb, err := m.kbEmbedder.EmbedSingle(ctx, query)
	if err != nil {
		slog.WarnContext(ctx, "dashboard semantic fallback embedding failed",
			"event", "dashboard_lookup_routing",
			"error", err,
		)
		return nil
	}

	results := vectorIndex.Search(emb, dashboardLookupSemanticMaxHits)
	hits := make([]semanticFallbackHit, 0, len(results))
	for _, r := range results {
		if r.Score <= 0 {
			continue
		}
		hits = append(hits, semanticFallbackHit{
			Path:    r.Section.Path,
			Title:   r.Section.Title,
			Snippet: kb.Snippet(r.Section.Content, dashboardLookupSnippetMaxLength),
			Score:   r.Score,
		})
	}
	return hits
}

func formatDashboardMetadataContext(hits []dashboardSearchHit) string {
	var b strings.Builder
	b.WriteString("Dashboard metadata matches (exact/title/tag search):\n")
	for _, h := range hits {
		line := fmt.Sprintf("- `%s` uid=`%s`", safeDashboardTitle(h.Title), safeDashboardUID(h.UID))
		if h.Folder != "" {
			line += fmt.Sprintf(" folder=`%s`", h.Folder)
		}
		if len(h.Tags) > 0 {
			line += " tags=" + strings.Join(limitStrings(h.Tags, 6), ",")
		}
		if h.URL != "" {
			line += " url=" + h.URL
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("Use these UID/title matches first before broader search.")
	return strings.TrimSpace(b.String())
}

func formatDashboardSemanticFallbackContext(hits []semanticFallbackHit) string {
	var b strings.Builder
	b.WriteString("Dashboard semantic fallback results (metadata search had no exact/tag match):\n")
	for _, h := range hits {
		line := fmt.Sprintf("- %s", h.Path)
		if h.Score > 0 {
			line += fmt.Sprintf(" score=%.2f", h.Score)
		}
		if h.Title != "" {
			line += " title=\"" + h.Title + "\""
		}
		if h.Snippet != "" {
			line += " snippet=\"" + h.Snippet + "\""
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("Use these as fallback hints only.")
	return strings.TrimSpace(b.String())
}

func safeDashboardTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return "(untitled dashboard)"
	}
	return t
}

func safeDashboardUID(uid string) string {
	u := strings.TrimSpace(uid)
	if u == "" {
		return "unknown"
	}
	return u
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
