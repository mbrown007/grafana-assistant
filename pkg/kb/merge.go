package kb

import "sort"

// UnifiedResult represents a search result from either token or vector search.
type UnifiedResult struct {
	ID      string
	Title   string
	Content string
	Path    string
	Score   float64 // normalized 0-1
	Source  string  // "token" or "vector"
}

const vectorScoreThreshold = 0.3

// MergeResults combines token-based and vector-based search results into a
// single ranked list. Token scores are normalized by dividing by the maximum
// token score. Vector scores are already in [0,1] (cosine similarity) and
// are filtered below the threshold. The merged list is sorted by score
// descending and truncated to limit.
func MergeResults(tokenResults []SearchResult, vectorResults []VectorSearchResult, limit int) []UnifiedResult {
	var merged []UnifiedResult

	// Find max token score for normalization.
	var maxToken int
	for _, r := range tokenResults {
		if r.Score > maxToken {
			maxToken = r.Score
		}
	}

	for _, r := range tokenResults {
		if r.Score <= 0 {
			continue
		}
		norm := float64(r.Score) / float64(maxToken)
		merged = append(merged, UnifiedResult{
			ID:      r.Section.ID,
			Title:   r.Section.Title,
			Content: r.Section.Content,
			Path:    r.Section.Path,
			Score:   norm,
			Source:  "token",
		})
	}

	for _, r := range vectorResults {
		if r.Score < vectorScoreThreshold {
			continue
		}
		merged = append(merged, UnifiedResult{
			ID:      r.Section.ID,
			Title:   r.Section.Title,
			Content: r.Section.Content,
			Path:    r.Section.Path,
			Score:   r.Score,
			Source:  "vector",
		})
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}
