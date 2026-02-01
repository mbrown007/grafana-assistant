package server

import (
	"testing"
)

func TestValidatePaginationParams(t *testing.T) {
	tests := []struct {
		name          string
		count         int
		offset        int
		maxCount      int
		wantCount     int
		wantOffset    int
		wantErr       bool
		wantErrString string
	}{
		{
			name:       "valid params",
			count:      10,
			offset:     0,
			maxCount:   25,
			wantCount:  10,
			wantOffset: 0,
			wantErr:    false,
		},
		{
			name:          "zero count",
			count:         0,
			offset:        0,
			maxCount:      25,
			wantErr:       true,
			wantErrString: "count parameter (0) must be at least 1",
		},
		{
			name:          "negative count",
			count:         -5,
			offset:        0,
			maxCount:      25,
			wantErr:       true,
			wantErrString: "count parameter (-5) must be at least 1",
		},
		{
			name:          "negative offset",
			count:         10,
			offset:        -1,
			maxCount:      25,
			wantErr:       true,
			wantErrString: "offset parameter (-1) must be non-negative (>= 0)",
		},
		{
			name:      "count exceeds max",
			count:     100,
			offset:    0,
			maxCount:  25,
			wantErr:   true,
		},
		{
			name:       "large offset",
			count:      10,
			offset:     1000,
			maxCount:   25,
			wantCount:  10,
			wantOffset: 1000,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCount, gotOffset, err := ValidatePaginationParams(tt.count, tt.offset, tt.maxCount)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePaginationParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.wantErrString != "" {
				if err.Error() != tt.wantErrString {
					t.Errorf("ValidatePaginationParams() error = %v, want %v", err.Error(), tt.wantErrString)
				}
				return
			}

			if !tt.wantErr {
				if gotCount != tt.wantCount {
					t.Errorf("ValidatePaginationParams() gotCount = %v, want %v", gotCount, tt.wantCount)
				}
				if gotOffset != tt.wantOffset {
					t.Errorf("ValidatePaginationParams() gotOffset = %v, want %v", gotOffset, tt.wantOffset)
				}
			}
		})
	}
}

func TestPaginateResults(t *testing.T) {
	// Create test data
	testData := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}

	tests := []struct {
		name       string
		items      []string
		limit      int
		offset     int
		want       []string
		wantTotal  int
		wantOffset int
		wantHasMore bool
	}{
		{
			name:        "first page",
			items:       testData,
			limit:       3,
			offset:      0,
			want:        []string{"a", "b", "c"},
			wantTotal:   10,
			wantOffset:  0,
			wantHasMore: true,
		},
		{
			name:        "middle page",
			items:       testData,
			limit:       3,
			offset:      3,
			want:        []string{"d", "e", "f"},
			wantTotal:   10,
			wantOffset:  3,
			wantHasMore: true,
		},
		{
			name:        "last page partial",
			items:       testData,
			limit:       3,
			offset:      9,
			want:        []string{"j"},
			wantTotal:   10,
			wantOffset:  9,
			wantHasMore: false,
		},
		{
			name:        "offset beyond data",
			items:       testData,
			limit:       3,
			offset:      20,
			want:        []string{},
			wantTotal:   10,
			wantOffset:  20,
			wantHasMore: false,
		},
		{
			name:        "limit larger than data",
			items:       testData,
			limit:       100,
			offset:      0,
			want:        testData,
			wantTotal:   10,
			wantOffset:  0,
			wantHasMore: false,
		},
		{
			name:        "empty data",
			items:       []string{},
			limit:       10,
			offset:      0,
			want:        []string{},
			wantTotal:   0,
			wantOffset:  0,
			wantHasMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PaginateResults(tt.items, tt.limit, tt.offset)

			if result.Pagination.Total != tt.wantTotal {
				t.Errorf("PaginateResults() Total = %v, want %v", result.Pagination.Total, tt.wantTotal)
			}

			if result.Pagination.Offset != tt.wantOffset {
				t.Errorf("PaginateResults() Offset = %v, want %v", result.Pagination.Offset, tt.wantOffset)
			}

			if result.Pagination.HasMore != tt.wantHasMore {
				t.Errorf("PaginateResults() HasMore = %v, want %v", result.Pagination.HasMore, tt.wantHasMore)
			}

			// Cast data to []string for comparison
			got, ok := result.Data.([]string)
			if !ok {
				t.Fatal("Failed to cast result.Data to []string")
			}

			if len(got) != len(tt.want) {
				t.Errorf("PaginateResults() got %d items, want %d", len(got), len(tt.want))
				return
			}

			for i, item := range got {
				if item != tt.want[i] {
					t.Errorf("PaginateResults() item[%d] = %v, want %v", i, item, tt.want[i])
				}
			}
		})
	}
}

func TestPaginateResultsWithStructs(t *testing.T) {
	type TestStruct struct {
		ID   int
		Name string
	}

	testData := []TestStruct{
		{1, "a"},
		{2, "b"},
		{3, "c"},
		{4, "d"},
		{5, "e"},
	}

	result := PaginateResults(testData, 2, 1)

	if result.Pagination.Total != 5 {
		t.Errorf("PaginateResults() Total = %v, want 5", result.Pagination.Total)
	}

	if result.Pagination.Offset != 1 {
		t.Errorf("PaginateResults() Offset = %v, want 1", result.Pagination.Offset)
	}

	got, ok := result.Data.([]TestStruct)
	if !ok {
		t.Fatal("Failed to cast result.Data to []TestStruct")
	}

	if len(got) != 2 {
		t.Errorf("PaginateResults() got %d items, want 2", len(got))
		return
	}

	if got[0].ID != 2 || got[1].ID != 3 {
		t.Errorf("PaginateResults() returned wrong items: %+v", got)
	}
}

// Benchmark tests

func BenchmarkValidatePaginationParams(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = ValidatePaginationParams(10, 0, 25)
	}
}

func BenchmarkPaginateResults(b *testing.B) {
	// Create test data
	testData := make([]int, 1000)
	for i := range testData {
		testData[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PaginateResults(testData, 10, 0)
	}
}

func BenchmarkPaginateResultsLargeOffset(b *testing.B) {
	testData := make([]int, 10000)
	for i := range testData {
		testData[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PaginateResults(testData, 10, 9000)
	}
}
