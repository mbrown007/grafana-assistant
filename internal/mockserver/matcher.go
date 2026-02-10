package mockserver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const toolsManifestFile = "tools.json"

// Fixture defines one replayable tool response.
type Fixture struct {
	ToolName string         `json:"tool_name"`
	Match    map[string]any `json:"match"`
	Response any            `json:"response"`

	SourcePath string `json:"-"`
}

type compiledFixture struct {
	fixture Fixture
	regex   map[string]*regexp.Regexp
}

// Matcher resolves incoming tool calls to fixture responses.
type Matcher struct {
	fixtures []compiledFixture
	byTool   map[string][]int
}

// LoadFixtures scans fixtureDir recursively and loads JSON fixture files.
func LoadFixtures(fixtureDir string) ([]Fixture, error) {
	info, err := os.Stat(fixtureDir)
	if err != nil {
		return nil, fmt.Errorf("fixture directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fixture path is not a directory: %s", fixtureDir)
	}

	var files []string
	if err := filepath.WalkDir(fixtureDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), toolsManifestFile) {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan fixture directory: %w", err)
	}
	sort.Strings(files)

	fixtures := make([]Fixture, 0, len(files))
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read fixture %s: %w", path, err)
		}

		var f Fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("parse fixture %s: %w", path, err)
		}

		f.ToolName = strings.TrimSpace(f.ToolName)
		if f.ToolName == "" {
			f.ToolName = deriveToolNameFromPath(fixtureDir, path)
		}
		if f.ToolName == "" {
			return nil, fmt.Errorf("fixture %s has empty tool_name and no derivable tool", path)
		}
		if f.Match == nil {
			f.Match = map[string]any{}
		}
		f.SourcePath = path
		fixtures = append(fixtures, f)
	}

	return fixtures, nil
}

// FilterFixturesByServerType keeps fixtures applicable to one MCP server type.
// Prefixed tool names (e.g. "grafana__query_prometheus") are filtered by
// matching prefix. Unprefixed names are retained for server-local fixture sets.
func FilterFixturesByServerType(fixtures []Fixture, serverType string) []Fixture {
	serverType = strings.ToLower(strings.TrimSpace(serverType))
	if serverType == "" {
		out := make([]Fixture, len(fixtures))
		copy(out, fixtures)
		return out
	}

	out := make([]Fixture, 0, len(fixtures))
	for _, fixture := range fixtures {
		prefix, hasPrefix := toolPrefix(fixture.ToolName)
		if hasPrefix && prefix != serverType {
			continue
		}
		out = append(out, fixture)
	}
	return out
}

// NewMatcher compiles fixture regex patterns and prepares an index by tool name.
func NewMatcher(fixtures []Fixture) (*Matcher, error) {
	compiled := make([]compiledFixture, 0, len(fixtures))
	byTool := make(map[string][]int)

	for _, fixture := range fixtures {
		cf := compiledFixture{
			fixture: fixture,
			regex:   make(map[string]*regexp.Regexp, len(fixture.Match)),
		}

		for key, pattern := range fixture.Match {
			patternText := strings.TrimSpace(argString(pattern))
			if patternText == "" {
				return nil, fmt.Errorf("fixture %s has empty regex for match key %q", fixture.SourcePath, key)
			}
			re, err := regexp.Compile(patternText)
			if err != nil {
				return nil, fmt.Errorf("fixture %s invalid regex for key %q: %w", fixture.SourcePath, key, err)
			}
			cf.regex[key] = re
		}

		idx := len(compiled)
		compiled = append(compiled, cf)
		for _, alias := range toolAliases(fixture.ToolName) {
			byTool[alias] = append(byTool[alias], idx)
		}
	}

	return &Matcher{
		fixtures: compiled,
		byTool:   byTool,
	}, nil
}

// Match returns the fixture response for a tool call when a fixture matches.
func (m *Matcher) Match(toolName string, args map[string]any) (any, bool) {
	if m == nil {
		return nil, false
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return nil, false
	}
	if args == nil {
		args = map[string]any{}
	}

	candidateIDs := m.byTool[toolName]
	for _, idx := range candidateIDs {
		candidate := m.fixtures[idx]
		if fixtureMatches(candidate, args) {
			return candidate.fixture.Response, true
		}
	}

	return nil, false
}

// ToolNames returns unique normalized tool names derived from fixtures.
func (m *Matcher) ToolNames() []string {
	if m == nil || len(m.fixtures) == 0 {
		return nil
	}

	shortToPrefixed := make(map[string]map[string]struct{}, len(m.fixtures))
	for _, f := range m.fixtures {
		full := strings.TrimSpace(f.fixture.ToolName)
		if full == "" {
			continue
		}
		short := shortToolName(full)
		if short == "" {
			continue
		}
		if !strings.Contains(full, "__") {
			continue
		}
		if shortToPrefixed[short] == nil {
			shortToPrefixed[short] = map[string]struct{}{}
		}
		shortToPrefixed[short][full] = struct{}{}
	}

	seen := make(map[string]struct{}, len(m.fixtures))
	toolNames := make([]string, 0, len(m.fixtures))
	for _, f := range m.fixtures {
		full := strings.TrimSpace(f.fixture.ToolName)
		if full == "" {
			continue
		}
		short := shortToolName(full)
		name := short
		if len(shortToPrefixed[short]) > 1 {
			name = full
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)
	return toolNames
}

func fixtureMatches(f compiledFixture, args map[string]any) bool {
	for key, re := range f.regex {
		value, ok := args[key]
		if !ok {
			return false
		}
		if !re.MatchString(argString(value)) {
			return false
		}
	}
	return true
}

func deriveToolNameFromPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" {
		return ""
	}

	dir := filepath.Dir(rel)
	if dir != "." && dir != "" {
		base := filepath.Base(dir)
		if base != "." && base != "" {
			return strings.TrimSpace(base)
		}
	}
	return strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
}

func toolAliases(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	short := shortToolName(name)
	if short == name {
		return []string{name}
	}
	return []string{name, short}
}

func shortToolName(name string) string {
	parts := strings.SplitN(strings.TrimSpace(name), "__", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(name)
}

func toolPrefix(name string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(name), "__", 2)
	if len(parts) != 2 {
		return "", false
	}
	prefix := strings.ToLower(strings.TrimSpace(parts[0]))
	if prefix == "" {
		return "", false
	}
	return prefix, true
}

func argString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
