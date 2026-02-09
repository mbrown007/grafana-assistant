package agent

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

const redactionMask = "[REDACTED]"

var sensitiveKeyMarkers = []string{
	"token",
	"secret",
	"password",
	"passwd",
	"api_key",
	"apikey",
	"authorization",
	"private_key",
	"client_secret",
	"refresh_token",
	"access_key",
	"secret_key",
	"credential",
	"session_key",
}

var (
	bearerTokenPattern = regexp.MustCompile(`(?i)\b(Bearer)\s+([A-Za-z0-9\-._~+/]+=*)`)
	basicAuthPattern   = regexp.MustCompile(`(?i)\b(Basic)\s+([A-Za-z0-9+/=]+)`)
	jwtPattern         = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9._-]+\.[A-Za-z0-9._-]+\b`)
	keyValuePattern    = regexp.MustCompile(`(?i)\b(api[_-]?key|token|password|passwd|secret|client[_-]?secret|access[_-]?key|refresh[_-]?token|private[_-]?key|authorization)\b(\s*[:=]\s*)([^\s,;]+)`)
	queryParamPattern  = regexp.MustCompile(`(?i)\b(api[_-]?key|token|password|passwd|secret|client[_-]?secret|access[_-]?key|refresh[_-]?token)\s*=\s*([^&\s]+)`)
)

func redactToolStreamValueForClient(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]any, []any:
		// Hot path for MCP payloads: avoid json marshal/unmarshal round-trip.
		return redactAny(v, "")
	case string:
		return redactSensitiveString(v)
	}

	// Fallback for uncommon structured types (for example structs from adapters).
	normalized, ok := normalizeForRedaction(value)
	if ok {
		return redactAny(normalized, "")
	}
	return value
}

func redactEvidencePayloadForClient(payload *api.EvidencePayload) *api.EvidencePayload {
	if payload == nil {
		return nil
	}

	out := &api.EvidencePayload{}
	if payload.KBSearch != nil {
		kbOut := &api.KBSearchEvidence{
			Query:   redactSensitiveString(payload.KBSearch.Query),
			Results: make([]api.EvidenceResult, len(payload.KBSearch.Results)),
		}
		for i, result := range payload.KBSearch.Results {
			kbOut.Results[i] = redactEvidenceResultForClient(result)
		}
		out.KBSearch = kbOut
	}
	if payload.VectorSearch != nil {
		vectorOut := &api.VectorSearchEvidence{
			Query:   redactSensitiveString(payload.VectorSearch.Query),
			Results: make([]api.EvidenceResult, len(payload.VectorSearch.Results)),
		}
		for i, result := range payload.VectorSearch.Results {
			vectorOut.Results[i] = redactEvidenceResultForClient(result)
		}
		out.VectorSearch = vectorOut
	}

	return out
}

func redactEvidenceResultForClient(result api.EvidenceResult) api.EvidenceResult {
	return api.EvidenceResult{
		ID:      redactSensitiveString(result.ID),
		Path:    redactSensitiveString(result.Path),
		Title:   redactSensitiveString(result.Title),
		Excerpt: redactSensitiveString(result.Excerpt),
		Score:   result.Score,
		Source:  redactSensitiveString(result.Source),
	}
}

func normalizeForRedaction(value any) (any, bool) {
	if value == nil {
		return nil, true
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

func redactAny(value any, parentKey string) any {
	if isSensitiveKey(parentKey) {
		return redactionMask
	}

	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = redactAny(item, k)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = redactAny(item, parentKey)
		}
		return out
	case string:
		return redactSensitiveString(v)
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(key))
	if trimmed == "" {
		return false
	}

	// Intentionally aggressive substring matching: false positives are preferred
	// over leaking secrets to client-visible evidence payloads.
	normalized := strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(trimmed)
	for _, marker := range sensitiveKeyMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func redactSensitiveString(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}

	redacted := value
	redacted = bearerTokenPattern.ReplaceAllString(redacted, "$1 "+redactionMask)
	redacted = basicAuthPattern.ReplaceAllString(redacted, "$1 "+redactionMask)
	redacted = jwtPattern.ReplaceAllString(redacted, redactionMask)
	redacted = keyValuePattern.ReplaceAllString(redacted, "$1$2"+redactionMask)
	redacted = queryParamPattern.ReplaceAllString(redacted, "$1="+redactionMask)
	return redacted
}
