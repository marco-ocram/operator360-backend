// Package pageparam centralizes page/page_size query-param parsing and
// in-memory slice pagination, both duplicated across many handlers.
package pageparam

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// JSONFields builds the standard pagination metadata block
// ({"page", "page_size", "total_records", "total_pages", "has_next",
// "has_previous"}) returned alongside paginated data.
func jsonFields(page, size, total, totalPages int) gin.H {
	return gin.H{
		"page":          page,
		"page_size":     size,
		"total_records": total,
		"total_pages":   totalPages,
		"has_next":      page < totalPages,
		"has_previous":  page > 1,
	}
}

// Parse reads "page" and "page_size" query params, falling back to 1 and
// defaultSize when absent or invalid, and clamping page_size to [1, maxSize].
func Parse(c *gin.Context, defaultSize, maxSize int) (page, size int) {
	page = 1
	size = defaultSize

	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(c.Query("page_size")); err == nil && ps > 0 && ps <= maxSize {
		size = ps
	}

	return page, size
}

// Result describes one page of a larger slice.
type Result[T any] struct {
	Items      []T
	Page       int
	PageSize   int
	Total      int
	TotalPages int
}

// HasNext reports whether a next page exists.
func (r Result[T]) HasNext() bool { return r.Page < r.TotalPages }

// HasPrevious reports whether a previous page exists.
func (r Result[T]) HasPrevious() bool { return r.Page > 1 }

// JSON returns the standard pagination metadata block for embedding in a
// response under a "pagination" key.
func (r Result[T]) JSON() gin.H {
	return jsonFields(r.Page, r.PageSize, r.Total, r.TotalPages)
}

// Slice returns the requested page of items, clamping page to the last
// available page when it overshoots, and returning an empty slice (never
// nil) when the requested range is out of bounds.
func Slice[T any](items []T, page, size int) Result[T] {
	total := len(items)
	totalPages := (total + size - 1) / size

	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	start := (page - 1) * size
	end := start + size
	if start >= total {
		start, end = 0, 0
	} else if end > total {
		end = total
	}

	paged := []T{}
	if start < end {
		paged = items[start:end]
	}

	return Result[T]{
		Items:      paged,
		Page:       page,
		PageSize:   size,
		Total:      total,
		TotalPages: totalPages,
	}
}
