package mockserver

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/brownster/grafana-assistant/internal/mcp"
)

// EvalCaseIDHeader is an optional request header used by eval runners to tag
// tool calls with a stable case ID for fixture file naming.
const EvalCaseIDHeader = "X-Assistant-Eval-Case-ID"

type evalCaseIDContextKey struct{}

// ContextWithEvalCaseID stores an eval case ID in context for fixture recording.
func ContextWithEvalCaseID(ctx context.Context, caseID string) context.Context {
	caseID = normalizeFixtureCaseID(caseID)
	if caseID == "" {
		return ctx
	}
	return context.WithValue(ctx, evalCaseIDContextKey{}, caseID)
}

// EvalCaseIDFromContext gets the eval case ID previously set by ContextWithEvalCaseID.
func EvalCaseIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	raw, _ := ctx.Value(evalCaseIDContextKey{}).(string)
	return normalizeFixtureCaseID(raw)
}

// NewRecordingClient wraps an MCP client and records successful tool
// call/response pairs as replay fixtures when outputDir is non-empty.
func NewRecordingClient(inner mcp.Client, outputDir string) mcp.Client {
	outputDir = strings.TrimSpace(outputDir)
	if inner == nil || outputDir == "" {
		return inner
	}
	return &RecordingClient{
		inner:     inner,
		outputDir: outputDir,
	}
}

// RecordingClient wraps an MCP client and records tool calls to fixture files.
type RecordingClient struct {
	inner     mcp.Client
	outputDir string
	mu        sync.Mutex
	counter   int
}

func (r *RecordingClient) Connect(ctx context.Context) error {
	return r.inner.Connect(ctx)
}

func (r *RecordingClient) Health(ctx context.Context) error {
	return r.inner.Health(ctx)
}

func (r *RecordingClient) DiscoverTools(ctx context.Context) ([]mcp.Tool, error) {
	return r.inner.DiscoverTools(ctx)
}

func (r *RecordingClient) InvokeTool(ctx context.Context, name string, args map[string]any) (any, error) {
	result, err := r.inner.InvokeTool(ctx, name, args)
	if err != nil {
		return nil, err
	}
	if err := r.saveFixture(ctx, name, args, result); err != nil {
		slog.WarnContext(ctx, "failed to record eval fixture", "tool", name, "error", err)
	}
	return result, nil
}

func (r *RecordingClient) saveFixture(ctx context.Context, toolName string, args map[string]any, result any) error {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return fmt.Errorf("missing tool name")
	}

	if args == nil {
		args = map[string]any{}
	}

	caseID := EvalCaseIDFromContext(ctx)

	r.mu.Lock()
	defer r.mu.Unlock()

	if caseID == "" {
		r.counter++
		caseID = fmt.Sprintf("case_%04d", r.counter)
	}

	match := make(map[string]any, len(args))
	for key, value := range args {
		match[key] = exactMatchRegex(value)
	}

	fixture := Fixture{
		ToolName: toolName,
		Match:    match,
		Response: result,
	}

	toolDir := filepath.Join(r.outputDir, sanitizeFixturePathSegment(toolName))
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		return fmt.Errorf("mkdir fixture tool dir: %w", err)
	}

	argHash, err := fixtureArgHash(args)
	if err != nil {
		return fmt.Errorf("hash args: %w", err)
	}
	outPath := filepath.Join(toolDir, fmt.Sprintf("%s_%s.json", caseID, argHash))

	body, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fixture: %w", err)
	}
	body = append(body, '\n')

	// Keep writes idempotent for repeated runs.
	if existing, err := os.ReadFile(outPath); err == nil && string(existing) == string(body) {
		return nil
	}

	if err := os.WriteFile(outPath, body, 0o644); err != nil {
		return fmt.Errorf("write fixture file: %w", err)
	}
	return nil
}

func fixtureArgHash(args map[string]any) (string, error) {
	normalized := normalizeForHash(args)
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])[:10], nil
}

func exactMatchRegex(value any) string {
	switch v := value.(type) {
	case string:
		return "^" + regexp.QuoteMeta(v) + "$"
	default:
		data, err := json.Marshal(normalizeForHash(v))
		if err != nil {
			return "^" + regexp.QuoteMeta(fmt.Sprintf("%v", v)) + "$"
		}
		return "^" + regexp.QuoteMeta(string(data)) + "$"
	}
}

func sanitizeFixturePathSegment(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown_tool"
	}
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "unknown_tool"
	}
	return out
}

func normalizeFixtureCaseID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return ""
	}
	return out
}

func normalizeForHash(v any) any {
	switch value := v.(type) {
	case map[string]any:
		if len(value) == 0 {
			return map[string]any{}
		}
		keys := make([]string, 0, len(value))
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(value))
		for _, k := range keys {
			out[k] = normalizeForHash(value[k])
		}
		return out
	case []any:
		out := make([]any, 0, len(value))
		for _, item := range value {
			out = append(out, normalizeForHash(item))
		}
		return out
	default:
		return value
	}
}
