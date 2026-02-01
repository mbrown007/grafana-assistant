package server

import "fmt"

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Count  int
	Offset int
}

// PaginationResult represents paginated result metadata
type PaginationResult struct {
	Total          int  `json:"total"`
	Offset         int  `json:"offset"`
	Count          int  `json:"count"`
	RequestedCount int  `json:"requested_count"`
	HasMore        bool `json:"has_more"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       any              `json:"data"`
	Pagination PaginationResult `json:"pagination"`
}

// ValidatePaginationParams validates and normalizes pagination parameters
func ValidatePaginationParams(count, offset, maxCount int) (int, int, error) {
	// Validate count parameter
	if count < 1 {
		return 0, 0, fmt.Errorf("count parameter (%d) must be at least 1", count)
	}
	if count > maxCount {
		return 0, 0, fmt.Errorf("count parameter (%d) exceeds maximum allowed value (%d). Please use count <= %d and paginate through results using the offset parameter", count, maxCount, maxCount)
	}

	// Validate offset parameter
	if offset < 0 {
		return 0, 0, fmt.Errorf("offset parameter (%d) must be non-negative (>= 0)", offset)
	}

	return count, offset, nil
}

// PaginateResults applies pagination to a slice and generates metadata
func PaginateResults[T any](items []T, count, offset int) PaginatedResponse {
	total := len(items)
	endIndex := offset + count

	// Handle bounds
	if offset >= total {
		return PaginatedResponse{
			Data: []T{},
			Pagination: PaginationResult{
				Total:          total,
				Offset:         offset,
				Count:          0,
				RequestedCount: count,
				HasMore:        false,
			},
		}
	}

	if endIndex > total {
		endIndex = total
	}

	paginatedItems := items[offset:endIndex]
	hasMore := endIndex < total

	return PaginatedResponse{
		Data: paginatedItems,
		Pagination: PaginationResult{
			Total:          total,
			Offset:         offset,
			Count:          len(paginatedItems),
			RequestedCount: count,
			HasMore:        hasMore,
		},
	}
}
