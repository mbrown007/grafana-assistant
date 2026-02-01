package kb

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const (
	DefaultIndexFileName = ".kb_index.json"
	IndexVersion         = 1
)

type Index struct {
	Version  int       `json:"version"`
	Sections []Section `json:"sections"`
}

type Section struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Path    string   `json:"path"`
	Tokens  []string `json:"tokens,omitempty"`

	tokenSet map[string]struct{}
}

type SearchResult struct {
	Section Section
	Score   int
}

func DefaultIndexPath(root string) string {
	return filepath.Join(root, DefaultIndexFileName)
}

func BuildIndex(root string) (*Index, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("kb path is not a directory: %s", root)
	}

	var sections []Section
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(root, path)
		sections = append(sections, splitMarkdownSections(relPath, string(content))...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Index{Version: IndexVersion, Sections: sections}, nil
}

func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	if idx.Version != IndexVersion {
		return nil, fmt.Errorf("kb index version mismatch: %d", idx.Version)
	}
	for i := range idx.Sections {
		idx.Sections[i].ensureTokenSet()
	}
	return &idx, nil
}

func SaveIndex(path string, idx *Index) error {
	if idx == nil {
		return fmt.Errorf("kb index is nil")
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadOrBuild(root string) (*Index, error) {
	indexPath := DefaultIndexPath(root)
	idx, err := LoadIndex(indexPath)
	if err == nil {
		return idx, nil
	}
	return BuildIndex(root)
}

func LoadOrBuildWithPath(root, indexPath string) (*Index, error) {
	if indexPath == "" {
		return LoadOrBuild(root)
	}
	idx, err := LoadIndex(indexPath)
	if err == nil {
		return idx, nil
	}
	return BuildIndex(root)
}

func Search(idx *Index, weights map[string]int, limit int) []SearchResult {
	if idx == nil || len(idx.Sections) == 0 || limit <= 0 {
		return nil
	}

	results := make([]SearchResult, 0, len(idx.Sections))
	for _, sec := range idx.Sections {
		sec.ensureTokenSet()
		score := scoreSection(sec, weights)
		results = append(results, SearchResult{Section: sec, Score: score})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Section.Title < results[j].Section.Title
		}
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func FindSection(idx *Index, id string) (Section, bool) {
	if idx == nil || id == "" {
		return Section{}, false
	}
	for _, sec := range idx.Sections {
		if sec.ID == id {
			return sec, true
		}
	}
	return Section{}, false
}

func splitMarkdownSections(path, content string) []Section {
	lines := strings.Split(content, "\n")
	var sections []Section

	var currentTitle string
	var current []string
	sectionIndex := 0

	flush := func() {
		if currentTitle == "" && len(current) == 0 {
			return
		}
		body := strings.TrimSpace(strings.Join(current, "\n"))
		section := Section{
			ID:      makeSectionID(path, currentTitle, sectionIndex),
			Title:   currentTitle,
			Content: body,
			Path:    path,
		}
		section.setTokens(tokensFromText(currentTitle + "\n" + body))
		sections = append(sections, section)
		currentTitle = ""
		current = nil
		sectionIndex++
	}

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## ") || strings.HasPrefix(trim, "### ") {
			flush()
			currentTitle = strings.TrimLeft(trim, "# ")
			continue
		}
		current = append(current, line)
	}
	flush()

	if len(sections) == 0 {
		section := Section{
			ID:      makeSectionID(path, filepath.Base(path), 0),
			Title:   filepath.Base(path),
			Content: strings.TrimSpace(content),
			Path:    path,
		}
		section.setTokens(tokensFromText(section.Title + "\n" + section.Content))
		sections = append(sections, section)
	}
	return sections
}

func makeSectionID(path, title string, index int) string {
	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		cleanTitle = fmt.Sprintf("section-%d", index)
	}
	return fmt.Sprintf("%s::%s", path, cleanTitle)
}

func tokensFromText(text string) []string {
	tokens := map[string]struct{}{}
	var b strings.Builder
	flush := func() {
		if b.Len() < 2 {
			b.Reset()
			return
		}
		token := strings.ToLower(b.String())
		tokens[token] = struct{}{}
		b.Reset()
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()

	result := make([]string, 0, len(tokens))
	for token := range tokens {
		result = append(result, token)
	}
	sort.Strings(result)
	return result
}

func (s *Section) ensureTokenSet() {
	if s.tokenSet == nil {
		s.tokenSet = make(map[string]struct{}, len(s.Tokens))
		for _, t := range s.Tokens {
			s.tokenSet[t] = struct{}{}
		}
	}
}

func (s *Section) setTokens(tokens []string) {
	s.Tokens = tokens
	s.ensureTokenSet()
}

func scoreSection(section Section, weights map[string]int) int {
	score := 0
	section.ensureTokenSet()
	for token, w := range weights {
		if _, ok := section.tokenSet[token]; ok {
			score += w
		}
	}
	return score
}

func Tokenize(text string) map[string]struct{} {
	tokens := map[string]struct{}{}
	var b strings.Builder
	flush := func() {
		if b.Len() < 2 {
			b.Reset()
			return
		}
		token := strings.ToLower(b.String())
		tokens[token] = struct{}{}
		b.Reset()
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func WeightsFromTokens(tokens map[string]struct{}, weight int) map[string]int {
	weights := map[string]int{}
	if weight <= 0 {
		weight = 1
	}
	for token := range tokens {
		weights[token] = weight
	}
	return weights
}

func Snippet(content string, limit int) string {
	text := strings.TrimSpace(content)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}
