package mcp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// tryDomainSummary attempts to produce a domain-enriched summary.
// Returns ("", false) when no domain-specific formatter matches.
func tryDomainSummary(value any) (string, bool) {
	switch v := value.(type) {
	case map[string]any:
		if isAlertResult(v) {
			summary := summarizeAlertResults(v)
			if summary != "" {
				return summary, true
			}
		}
		if isPrometheusResult(v) {
			summary := summarizePrometheusResults(v)
			if summary != "" {
				return summary, true
			}
		}
		if isLokiResult(v) {
			summary := summarizeLokiResults(v)
			if summary != "" {
				return summary, true
			}
		}
		return "", false
	case []any:
		if !looksLikeAlertList(v) {
			return "", false
		}
		summary := summarizeAlertList(v, nil)
		if summary == "" {
			return "", false
		}
		return summary, true
	default:
		return "", false
	}
}

func isAlertResult(obj map[string]any) bool {
	if alerts := extractAlertsFromEnvelope(obj); len(alerts) > 0 && looksLikeAlertObjects(alerts) {
		return true
	}
	return isAlertObject(obj)
}

func isPrometheusResult(obj map[string]any) bool {
	dataObj, ok := obj["data"].(map[string]any)
	if !ok {
		return false
	}

	resultType := strings.ToLower(strings.TrimSpace(toStringValue(dataObj["resultType"])))
	switch resultType {
	case "matrix", "vector", "scalar":
	default:
		return false
	}

	if _, ok := dataObj["result"]; !ok {
		return false
	}
	return true
}

func isLokiResult(obj map[string]any) bool {
	dataObj, ok := obj["data"].(map[string]any)
	if !ok {
		return false
	}
	if strings.ToLower(strings.TrimSpace(toStringValue(dataObj["resultType"]))) != "streams" {
		return false
	}
	_, ok = dataObj["result"].([]any)
	return ok
}

func summarizePrometheusResults(obj map[string]any) string {
	dataObj, ok := obj["data"].(map[string]any)
	if !ok {
		return ""
	}
	resultType := strings.ToLower(strings.TrimSpace(toStringValue(dataObj["resultType"])))
	if resultType == "" {
		return ""
	}

	switch resultType {
	case "scalar":
		samples := samplesFromPromPair(dataObj["result"])
		if len(samples) == 0 {
			return "Prometheus scalar query returned 1 value."
		}
		value := samples[len(samples)-1].value
		return fmt.Sprintf("Prometheus scalar query returned 1 value; value=%s.", formatPromNumber(value))
	case "matrix", "vector":
	default:
		return ""
	}

	series := asObjectSlice(dataObj["result"])
	if len(series) == 0 {
		return fmt.Sprintf("Prometheus %s query returned 0 series.", resultType)
	}

	metricNames := map[string]struct{}{}
	labelCardinality := map[string]map[string]struct{}{}
	valueStats := promValueStats{}
	totalSamples := 0

	for _, item := range series {
		metric := toStringMap(item["metric"])
		if name := strings.TrimSpace(metric["__name__"]); name != "" {
			metricNames[name] = struct{}{}
		}
		for key, val := range metric {
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)
			if key == "" || key == "__name__" || val == "" {
				continue
			}
			if _, ok := labelCardinality[key]; !ok {
				labelCardinality[key] = map[string]struct{}{}
			}
			labelCardinality[key][val] = struct{}{}
		}

		seriesSamples := samplesFromPromSeries(item, resultType)
		totalSamples += len(seriesSamples)
		for _, sample := range seriesSamples {
			valueStats.Observe(sample)
		}
	}

	parts := []string{
		fmt.Sprintf("Prometheus %s query returned %d series", resultType, len(series)),
	}

	if metrics := summarizePromMetricNames(metricNames, 3); metrics != "" {
		parts = append(parts, "metrics: "+metrics)
	}
	if valueStats.hasValue {
		valuePart := fmt.Sprintf("values min=%s, max=%s, latest=%s",
			formatPromNumber(valueStats.min),
			formatPromNumber(valueStats.max),
			formatPromNumber(valueStats.latest),
		)
		parts = append(parts, valuePart)
	}
	if labelPart := summarizePromLabelCardinality(labelCardinality, 4); labelPart != "" {
		parts = append(parts, "label cardinality: "+labelPart)
	}
	if totalSamples > 0 {
		parts = append(parts, fmt.Sprintf("sample points=%d", totalSamples))
	}
	if valueStats.hasTimeRange {
		parts = append(parts, "time range="+formatPromRangeDuration(valueStats.maxTS-valueStats.minTS))
	}

	if len(parts) == 1 {
		return parts[0] + "."
	}
	return parts[0] + "; " + strings.Join(parts[1:], "; ") + "."
}

func summarizeLokiResults(obj map[string]any) string {
	dataObj, ok := obj["data"].(map[string]any)
	if !ok {
		return ""
	}
	streamItems, ok := dataObj["result"].([]any)
	if !ok {
		return ""
	}

	streams := asObjectSlice(streamItems)
	if len(streams) == 0 {
		return "Loki query returned 0 log lines across 0 streams."
	}

	levelCounts := map[string]int{}
	serviceCounts := map[string]int{}
	totalLines := 0
	var minTS time.Time
	var maxTS time.Time
	hasRange := false

	for _, streamObj := range streams {
		streamLabels := toStringMap(streamObj["stream"])
		level := normalizeLokiLevel(firstNonEmpty(streamLabels["level"], streamLabels["severity"], streamLabels["lvl"]))
		service := firstNonEmpty(streamLabels["app"], streamLabels["service"], streamLabels["job"], streamLabels["container"])

		values, _ := streamObj["values"].([]any)
		lineCount := 0
		for _, raw := range values {
			ts, ok := parseLokiTimestampPair(raw)
			if ok {
				if !hasRange {
					minTS = ts
					maxTS = ts
					hasRange = true
				} else {
					if ts.Before(minTS) {
						minTS = ts
					}
					if ts.After(maxTS) {
						maxTS = ts
					}
				}
			}
			lineCount++
		}

		totalLines += lineCount
		if level != "" && lineCount > 0 {
			levelCounts[level] += lineCount
		}
		if service != "" && lineCount > 0 {
			serviceCounts[service] += lineCount
		}
	}

	parts := []string{
		fmt.Sprintf(
			"Loki query returned %d log line%s across %d stream%s",
			totalLines,
			pluralSuffix(totalLines),
			len(streams),
			pluralSuffix(len(streams)),
		),
	}

	if levelPart := summarizeCountMap(levelCounts, []string{"error", "warn", "info", "debug", "trace", "unknown"}); levelPart != "" {
		parts = append(parts, "levels: "+levelPart)
	}
	if servicesPart := summarizeTopValueCounts(serviceCounts, 4); servicesPart != "" {
		parts = append(parts, "apps/services: "+servicesPart)
	}
	if hasRange {
		parts = append(parts, fmt.Sprintf(
			"time span: %s-%s (%s)",
			minTS.UTC().Format("15:04:05"),
			maxTS.UTC().Format("15:04:05"),
			formatDurationCompact(maxTS.Sub(minTS)),
		))
	}

	if len(parts) == 1 {
		return parts[0] + "."
	}
	return parts[0] + "; " + strings.Join(parts[1:], "; ") + "."
}

func summarizeAlertResults(obj map[string]any) string {
	alerts := extractAlertsFromEnvelope(obj)
	if len(alerts) == 0 && isAlertObject(obj) {
		alerts = []map[string]any{obj}
	}
	if len(alerts) > 0 && !looksLikeAlertObjects(alerts) {
		return ""
	}
	return summarizeAlertListFromMaps(alerts, obj)
}

func summarizeAlertList(raw []any, envelope map[string]any) string {
	return summarizeAlertListFromMaps(asObjectSlice(raw), envelope)
}

func summarizeAlertListFromMaps(alerts []map[string]any, envelope map[string]any) string {
	if len(alerts) == 0 {
		return ""
	}

	stateCounts := map[string]int{}
	severityCounts := map[string]int{}
	labelGroupCounts := map[string]int{}
	var oldestFiring time.Duration

	now := time.Now().UTC()
	for _, alert := range alerts {
		state := alertState(alert)
		if state == "" {
			state = "unknown"
		}
		stateCounts[state]++

		severity := alertSeverity(alert)
		if severity == "" {
			severity = "unknown"
		}
		severityCounts[severity]++

		if state == "firing" {
			if d, ok := alertFiringDuration(alert, now); ok && d > oldestFiring {
				oldestFiring = d
			}
		}

		if group := commonAlertLabelGroup(alert); group != "" {
			labelGroupCounts[group]++
		}
	}

	parts := []string{
		fmt.Sprintf("%d alerts", len(alerts)),
	}

	if statePart := summarizeCountMap(stateCounts, []string{"firing", "pending", "resolved", "unknown"}); statePart != "" {
		parts = append(parts, "states: "+statePart)
	}
	if severityPart := summarizeCountMap(severityCounts, []string{"critical", "warning", "info", "unknown"}); severityPart != "" {
		parts = append(parts, "severity: "+severityPart)
	}
	if oldestFiring > 0 {
		parts = append(parts, "firing oldest "+formatDurationCompact(oldestFiring))
	}
	if labelsPart := summarizeLabelGroups(labelGroupCounts, 3); labelsPart != "" {
		parts = append(parts, "common labels: "+labelsPart)
	}
	if source := detectAlertDatasource(envelope); source != "" {
		parts = append(parts, "datasource: "+source)
	}

	if len(parts) == 1 {
		return parts[0] + "."
	}
	return strings.Join(parts[:1], "") + "; " + strings.Join(parts[1:], "; ") + "."
}

func extractAlertsFromEnvelope(obj map[string]any) []map[string]any {
	if obj == nil {
		return nil
	}
	if alerts, ok := obj["alerts"]; ok {
		return asObjectSlice(alerts)
	}
	if dataObj, ok := obj["data"].(map[string]any); ok {
		if alerts, ok := dataObj["alerts"]; ok {
			return asObjectSlice(alerts)
		}
	}
	return nil
}

func asObjectSlice(value any) []map[string]any {
	switch v := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				out = append(out, obj)
			}
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(v))
		copy(out, v)
		return out
	default:
		return nil
	}
}

func looksLikeAlertList(items []any) bool {
	if len(items) == 0 {
		return false
	}
	return looksLikeAlertObjects(asObjectSlice(items))
}

func looksLikeAlertObjects(items []map[string]any) bool {
	for _, item := range items {
		if isAlertObject(item) {
			return true
		}
	}
	return false
}

func isAlertObject(obj map[string]any) bool {
	if obj == nil {
		return false
	}

	if alertState(obj) != "" || alertSeverity(obj) != "" {
		return true
	}

	labels := toStringMap(obj["labels"])
	if labels["alertname"] != "" || labels["severity"] != "" || labels["job"] != "" || labels["service"] != "" || labels["app"] != "" {
		return true
	}

	return false
}

func alertState(alert map[string]any) string {
	state := normalizeAlertState(toStringValue(alert["state"]))
	if state != "" {
		return state
	}
	return normalizeAlertState(toStringValue(alert["status"]))
}

func normalizeAlertState(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "firing", "active", "alerting":
		return "firing"
	case "pending":
		return "pending"
	case "resolved", "inactive":
		return "resolved"
	default:
		return ""
	}
}

func alertSeverity(alert map[string]any) string {
	severity := normalizeAlertSeverity(toStringValue(alert["severity"]))
	if severity != "" {
		return severity
	}
	labels := toStringMap(alert["labels"])
	return normalizeAlertSeverity(labels["severity"])
}

func normalizeAlertSeverity(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "critical", "crit":
		return "critical"
	case "warning", "warn":
		return "warning"
	case "info", "informational", "information":
		return "info"
	default:
		return ""
	}
}

func commonAlertLabelGroup(alert map[string]any) string {
	labels := toStringMap(alert["labels"])
	for _, key := range []string{"job", "service", "app", "alertname"} {
		val := strings.TrimSpace(labels[key])
		if val != "" {
			return key + "=" + val
		}
	}
	return ""
}

func toStringMap(value any) map[string]string {
	out := map[string]string{}
	switch v := value.(type) {
	case map[string]any:
		for key, raw := range v {
			if s := toStringValue(raw); s != "" {
				out[key] = s
			}
		}
	case map[string]string:
		for key, s := range v {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out[key] = trimmed
			}
		}
	}
	return out
}

func alertFiringDuration(alert map[string]any, now time.Time) (time.Duration, bool) {
	start, ok := parseAlertTime(alert["startsAt"])
	if !ok {
		start, ok = parseAlertTime(alert["starts_at"])
	}
	if !ok || !now.After(start) {
		return 0, false
	}
	return now.Sub(start), true
}

func parseAlertTime(value any) (time.Time, bool) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return time.Time{}, false
		}
		if ts, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
			return ts.UTC(), true
		}
		if ts, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			// Alertmanager may emit unix seconds as strings.
			return time.Unix(ts, 0).UTC(), true
		}
	case float64:
		return time.Unix(int64(v), 0).UTC(), true
	case int64:
		return time.Unix(v, 0).UTC(), true
	case int:
		return time.Unix(int64(v), 0).UTC(), true
	}
	return time.Time{}, false
}

func formatDurationCompact(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int((d - time.Duration(h)*time.Hour).Minutes())
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}

	days := int(d / (24 * time.Hour))
	hours := int((d - time.Duration(days)*24*time.Hour) / time.Hour)
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, hours)
}

func summarizeCountMap(counts map[string]int, preferredOrder []string) string {
	if len(counts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(counts))
	used := map[string]bool{}

	for _, key := range preferredOrder {
		if count := counts[key]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", key, count))
			used[key] = true
		}
	}

	rest := make([]string, 0, len(counts))
	for key, count := range counts {
		if count <= 0 || used[key] {
			continue
		}
		rest = append(rest, fmt.Sprintf("%s=%d", key, count))
	}
	sort.Strings(rest)
	parts = append(parts, rest...)
	return strings.Join(parts, ", ")
}

func summarizeLabelGroups(labelCounts map[string]int, limit int) string {
	if len(labelCounts) == 0 || limit <= 0 {
		return ""
	}
	type labelEntry struct {
		label string
		count int
	}
	entries := make([]labelEntry, 0, len(labelCounts))
	for label, count := range labelCounts {
		if count > 0 {
			entries = append(entries, labelEntry{label: label, count: count})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].label < entries[j].label
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, fmt.Sprintf("%s (%d)", entry.label, entry.count))
	}
	return strings.Join(parts, ", ")
}

func detectAlertDatasource(envelope map[string]any) string {
	if envelope == nil {
		return ""
	}
	if ds := toStringValue(envelope["datasource"]); ds != "" {
		return ds
	}
	if src := toStringValue(envelope["source"]); src != "" {
		return src
	}
	if dataObj, ok := envelope["data"].(map[string]any); ok {
		if ds := toStringValue(dataObj["datasource"]); ds != "" {
			return ds
		}
		if _, hasAlerts := dataObj["alerts"]; hasAlerts && strings.EqualFold(toStringValue(envelope["status"]), "success") {
			return "alertmanager"
		}
	}
	return ""
}

type promSample struct {
	ts    float64
	value float64
}

type promValueStats struct {
	hasValue     bool
	min          float64
	max          float64
	latest       float64
	latestTS     float64
	minTS        float64
	maxTS        float64
	hasTimeRange bool
}

func (s *promValueStats) Observe(sample promSample) {
	if !s.hasValue {
		s.hasValue = true
		s.min = sample.value
		s.max = sample.value
		s.latest = sample.value
		s.latestTS = sample.ts
		if sample.ts > 0 {
			s.minTS = sample.ts
			s.maxTS = sample.ts
			s.hasTimeRange = true
		}
		return
	}
	if sample.value < s.min {
		s.min = sample.value
	}
	if sample.value > s.max {
		s.max = sample.value
	}
	if sample.ts >= s.latestTS {
		s.latestTS = sample.ts
		s.latest = sample.value
	}
	if sample.ts > 0 {
		if !s.hasTimeRange {
			s.hasTimeRange = true
			s.minTS = sample.ts
			s.maxTS = sample.ts
			return
		}
		if sample.ts < s.minTS {
			s.minTS = sample.ts
		}
		if sample.ts > s.maxTS {
			s.maxTS = sample.ts
		}
	}
}

func samplesFromPromSeries(series map[string]any, resultType string) []promSample {
	if series == nil {
		return nil
	}

	switch resultType {
	case "matrix":
		raw, ok := series["values"].([]any)
		if !ok {
			return nil
		}
		samples := make([]promSample, 0, len(raw))
		for _, value := range raw {
			samples = append(samples, samplesFromPromPair(value)...)
		}
		return samples
	case "vector":
		return samplesFromPromPair(series["value"])
	default:
		return nil
	}
}

func samplesFromPromPair(value any) []promSample {
	pair, ok := value.([]any)
	if !ok || len(pair) < 2 {
		return nil
	}
	ts, okTS := toFloat64(pair[0])
	val, okVal := toFloat64(pair[1])
	if !okVal {
		return nil
	}
	if !okTS {
		ts = 0
	}
	return []promSample{{ts: ts, value: val}}
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case jsonNumberString:
		f, err := strconv.ParseFloat(string(v), 64)
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// jsonNumberString documents that numeric JSON values may be represented as strings.
type jsonNumberString string

func summarizePromMetricNames(metricNames map[string]struct{}, limit int) string {
	if len(metricNames) == 0 || limit <= 0 {
		return ""
	}
	names := make([]string, 0, len(metricNames))
	for name := range metricNames {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > limit {
		names = names[:limit]
	}
	return strings.Join(names, ", ")
}

func summarizePromLabelCardinality(labelValues map[string]map[string]struct{}, limit int) string {
	if len(labelValues) == 0 || limit <= 0 {
		return ""
	}
	type card struct {
		label string
		count int
	}
	entries := make([]card, 0, len(labelValues))
	for label, values := range labelValues {
		entries = append(entries, card{label: label, count: len(values)})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].label < entries[j].label
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	parts := make([]string, 0, len(entries))
	for _, item := range entries {
		parts = append(parts, fmt.Sprintf("%s=%d", item.label, item.count))
	}
	return strings.Join(parts, ", ")
}

func formatPromNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', 6, 64)
}

func formatPromRangeDuration(seconds float64) string {
	if seconds <= 0 {
		return "0s"
	}
	return formatDurationCompact(time.Duration(seconds * float64(time.Second)))
}

func parseLokiTimestampPair(value any) (time.Time, bool) {
	pair, ok := value.([]any)
	if !ok || len(pair) < 1 {
		return time.Time{}, false
	}
	return parseLokiTimestamp(pair[0])
}

func parseLokiTimestamp(value any) (time.Time, bool) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return time.Time{}, false
		}
		if ts, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return unixTimestampToTime(ts)
		}
		if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
			return parsed.UTC(), true
		}
	case float64:
		return unixTimestampToTime(int64(v))
	case int64:
		return unixTimestampToTime(v)
	case int:
		return unixTimestampToTime(int64(v))
	}
	return time.Time{}, false
}

func unixTimestampToTime(ts int64) (time.Time, bool) {
	if ts <= 0 {
		return time.Time{}, false
	}

	switch {
	case ts >= 1_000_000_000_000_000:
		return time.Unix(0, ts).UTC(), true // nanoseconds
	case ts >= 1_000_000_000_000:
		return time.UnixMilli(ts).UTC(), true // milliseconds
	default:
		return time.Unix(ts, 0).UTC(), true // seconds
	}
}

func normalizeLokiLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error", "err":
		return "error"
	case "warning", "warn":
		return "warn"
	case "info", "information":
		return "info"
	case "debug":
		return "debug"
	case "trace":
		return "trace"
	default:
		if strings.TrimSpace(value) == "" {
			return ""
		}
		return "unknown"
	}
}

func summarizeTopValueCounts(counts map[string]int, limit int) string {
	if len(counts) == 0 || limit <= 0 {
		return ""
	}
	type entry struct {
		value string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for value, count := range counts {
		if strings.TrimSpace(value) == "" || count <= 0 {
			continue
		}
		entries = append(entries, entry{value: value, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].value < entries[j].value
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	parts := make([]string, 0, len(entries))
	for _, item := range entries {
		parts = append(parts, fmt.Sprintf("%s (%d)", item.value, item.count))
	}
	return strings.Join(parts, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
