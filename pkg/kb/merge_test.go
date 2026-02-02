package kb

import "testing"

func TestMergeResults(t *testing.T) {
	tokenResults := []SearchResult{
		{Section: Section{ID: "t1", Title: "Token A", Content: "content a", Path: "a.md"}, Score: 10},
		{Section: Section{ID: "t2", Title: "Token B", Content: "content b", Path: "b.md"}, Score: 5},
		{Section: Section{ID: "t3", Title: "Token C", Content: "content c", Path: "c.md"}, Score: 0}, // filtered
	}
	vectorResults := []VectorSearchResult{
		{Section: VectorSection{ID: "v1", Title: "Vector A", Content: "vec content a", Path: "va.md"}, Score: 0.9},
		{Section: VectorSection{ID: "v2", Title: "Vector B", Content: "vec content b", Path: "vb.md"}, Score: 0.2}, // below threshold
	}

	merged := MergeResults(tokenResults, vectorResults, 10)

	// Should have: t1 (score=1.0), v1 (score=0.9), t2 (score=0.5)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged results, got %d", len(merged))
	}

	if merged[0].ID != "t1" || merged[0].Source != "token" {
		t.Fatalf("expected t1 first, got %s (%s)", merged[0].ID, merged[0].Source)
	}
	if merged[0].Score != 1.0 {
		t.Fatalf("expected score 1.0, got %f", merged[0].Score)
	}

	if merged[1].ID != "v1" || merged[1].Source != "vector" {
		t.Fatalf("expected v1 second, got %s (%s)", merged[1].ID, merged[1].Source)
	}

	if merged[2].ID != "t2" || merged[2].Source != "token" {
		t.Fatalf("expected t2 third, got %s (%s)", merged[2].ID, merged[2].Source)
	}
	if merged[2].Score != 0.5 {
		t.Fatalf("expected score 0.5, got %f", merged[2].Score)
	}
}

func TestMergeResultsLimit(t *testing.T) {
	tokenResults := []SearchResult{
		{Section: Section{ID: "t1"}, Score: 10},
		{Section: Section{ID: "t2"}, Score: 8},
	}
	vectorResults := []VectorSearchResult{
		{Section: VectorSection{ID: "v1"}, Score: 0.9},
	}

	merged := MergeResults(tokenResults, vectorResults, 2)
	if len(merged) != 2 {
		t.Fatalf("expected 2 results (limited), got %d", len(merged))
	}
}

func TestMergeResultsEmpty(t *testing.T) {
	merged := MergeResults(nil, nil, 5)
	if len(merged) != 0 {
		t.Fatalf("expected 0 results, got %d", len(merged))
	}
}

func TestMergeResultsTokenOnly(t *testing.T) {
	tokenResults := []SearchResult{
		{Section: Section{ID: "t1"}, Score: 5},
	}
	merged := MergeResults(tokenResults, nil, 5)
	if len(merged) != 1 {
		t.Fatalf("expected 1 result, got %d", len(merged))
	}
	if merged[0].Source != "token" {
		t.Fatalf("expected token source, got %s", merged[0].Source)
	}
}

func TestMergeResultsVectorOnly(t *testing.T) {
	vectorResults := []VectorSearchResult{
		{Section: VectorSection{ID: "v1"}, Score: 0.8},
	}
	merged := MergeResults(nil, vectorResults, 5)
	if len(merged) != 1 {
		t.Fatalf("expected 1 result, got %d", len(merged))
	}
	if merged[0].Source != "vector" {
		t.Fatalf("expected vector source, got %s", merged[0].Source)
	}
}
