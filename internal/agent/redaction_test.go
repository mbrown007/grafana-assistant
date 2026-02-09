package agent

import (
	"strings"
	"testing"

	"github.com/marcusz/monitoring-assistant/internal/api"
)

func TestRedactSensitiveString_KnownPatterns(t *testing.T) {
	input := strings.Join([]string{
		"Authorization: Bearer super-secret-token",
		"password=supersecret",
		"token: abc123",
		"url=https://example.local?q=ok&api_key=myapikey",
		"jwt=eyJhbGciOiJIUzI1NiJ9.payload.signature",
	}, "\n")

	redacted := redactSensitiveString(input)
	for _, forbidden := range []string{
		"super-secret-token",
		"supersecret",
		"abc123",
		"myapikey",
		"eyJhbGciOiJIUzI1NiJ9.payload.signature",
	} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("expected %q to be redacted, got: %s", forbidden, redacted)
		}
	}
	if !strings.Contains(redacted, redactionMask) {
		t.Fatalf("expected redaction marker in output, got: %s", redacted)
	}
}

func TestRedactToolStreamValueForClient_KeyBasedRedaction(t *testing.T) {
	raw := map[string]any{
		"status": "ok",
		"token":  "abc123",
		"nested": map[string]any{
			"client_secret": "secret-value",
			"safe":          "value",
		},
	}

	redactedAny := redactToolStreamValueForClient(raw)
	redacted, ok := redactedAny.(map[string]any)
	if !ok {
		t.Fatalf("expected map redacted payload, got %T", redactedAny)
	}
	if redacted["token"] != redactionMask {
		t.Fatalf("expected token to be redacted, got %#v", redacted["token"])
	}
	nested, ok := redacted["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", redacted["nested"])
	}
	if nested["client_secret"] != redactionMask {
		t.Fatalf("expected client_secret to be redacted, got %#v", nested["client_secret"])
	}
	if nested["safe"] != "value" {
		t.Fatalf("expected safe key unchanged, got %#v", nested["safe"])
	}
	if raw["token"] != "abc123" {
		t.Fatalf("expected original map unchanged, got %#v", raw["token"])
	}
}

func TestRedactToolStreamValueForClient_FallbackStructNormalization(t *testing.T) {
	type upstreamResult struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}

	raw := upstreamResult{
		Token: "abc123",
		Name:  "service-a",
	}

	redactedAny := redactToolStreamValueForClient(raw)
	redacted, ok := redactedAny.(map[string]any)
	if !ok {
		t.Fatalf("expected fallback struct to normalize as map, got %T", redactedAny)
	}
	if redacted["token"] != redactionMask {
		t.Fatalf("expected token to be redacted, got %#v", redacted["token"])
	}
	if redacted["name"] != "service-a" {
		t.Fatalf("expected non-sensitive fields unchanged, got %#v", redacted["name"])
	}
}

func TestRedactEvidencePayloadForClient(t *testing.T) {
	payload := &api.EvidencePayload{
		KBSearch: &api.KBSearchEvidence{
			Query: "show data for token=abc123",
			Results: []api.EvidenceResult{
				{
					Path:    "/tmp/log?api_key=myapikey",
					Title:   "KB Result",
					Excerpt: "Authorization: Bearer top-secret",
					Source:  "kb",
				},
			},
		},
		VectorSearch: &api.VectorSearchEvidence{
			Query: "jwt=eyJhbGciOiJIUzI1NiJ9.payload.signature",
			Results: []api.EvidenceResult{
				{
					Path:    "vector/doc",
					Excerpt: "password: p@ssw0rd",
					Source:  "vector",
				},
			},
		},
	}

	redacted := redactEvidencePayloadForClient(payload)
	if redacted == nil {
		t.Fatal("expected non-nil redacted payload")
	}
	if strings.Contains(redacted.KBSearch.Query, "abc123") {
		t.Fatalf("expected KB query token to be redacted, got %q", redacted.KBSearch.Query)
	}
	if strings.Contains(redacted.KBSearch.Results[0].Path, "myapikey") {
		t.Fatalf("expected KB result path api_key to be redacted, got %q", redacted.KBSearch.Results[0].Path)
	}
	if strings.Contains(redacted.KBSearch.Results[0].Excerpt, "top-secret") {
		t.Fatalf("expected KB excerpt bearer token to be redacted, got %q", redacted.KBSearch.Results[0].Excerpt)
	}
	if strings.Contains(redacted.VectorSearch.Query, "eyJhbGciOiJIUzI1NiJ9.payload.signature") {
		t.Fatalf("expected vector query jwt to be redacted, got %q", redacted.VectorSearch.Query)
	}
	if strings.Contains(redacted.VectorSearch.Results[0].Excerpt, "p@ssw0rd") {
		t.Fatalf("expected vector excerpt password to be redacted, got %q", redacted.VectorSearch.Results[0].Excerpt)
	}

	if strings.Contains(payload.KBSearch.Query, redactionMask) {
		t.Fatalf("expected original payload to remain unchanged, got %q", payload.KBSearch.Query)
	}
}
