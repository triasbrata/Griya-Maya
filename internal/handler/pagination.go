package handler

import (
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// Pagination response-header names. Metadata lives in headers (not the body) so
// the response payload stays purely the resource. Both offset and cursor
// strategies share Kind + Has-Next; the rest are strategy-specific.
const (
	HdrPaginationKind       = "X-Pagination-Kind"        // "offset" | "cursor"
	HdrPaginationHasNext    = "X-Pagination-Has-Next"    // "true" | "false"
	HdrPaginationPerPage    = "X-Pagination-Per-Page"    // page size / limit
	HdrPaginationPage       = "X-Pagination-Page"        // offset: 1-based page
	HdrPaginationTotal      = "X-Pagination-Total"       // offset: total rows (if known)
	HdrPaginationNextCursor = "X-Pagination-Next-Cursor" // cursor: next page cursor
	HdrPaginationPrevCursor = "X-Pagination-Prev-Cursor" // cursor: prev page cursor
)

// PaginationHeaders is the full set of pagination headers. The server lists it
// in the CORS Access-Control-Expose-Headers so cross-origin browser clients
// (the admin panel) can actually read these off the response.
var PaginationHeaders = []string{
	HdrPaginationKind,
	HdrPaginationHasNext,
	HdrPaginationPerPage,
	HdrPaginationPage,
	HdrPaginationTotal,
	HdrPaginationNextCursor,
	HdrPaginationPrevCursor,
}

// writePagination emits pagination metadata into the response headers for either
// strategy. Absent/unknown values (empty cursor, negative total) are omitted.
func writePagination(c *app.RequestContext, p domain.Pagination) {
	c.Header(HdrPaginationKind, string(p.Kind))
	c.Header(HdrPaginationHasNext, strconv.FormatBool(p.HasNext))
	if p.PerPage > 0 {
		c.Header(HdrPaginationPerPage, strconv.Itoa(p.PerPage))
	}
	switch p.Kind {
	case domain.PaginationOffset:
		if p.Page > 0 {
			c.Header(HdrPaginationPage, strconv.Itoa(p.Page))
		}
		if p.Total >= 0 {
			c.Header(HdrPaginationTotal, strconv.Itoa(p.Total))
		}
	case domain.PaginationCursor:
		if p.NextCursor != "" {
			c.Header(HdrPaginationNextCursor, p.NextCursor)
		}
		if p.PrevCursor != "" {
			c.Header(HdrPaginationPrevCursor, p.PrevCursor)
		}
	}
}

// queryCursor reads the opaque cursor query param for cursor-paginated
// endpoints (empty means "first page"). Offset endpoints use queryInt("page").
func queryCursor(c *app.RequestContext) string {
	return c.Query("cursor")
}
