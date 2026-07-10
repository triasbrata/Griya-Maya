package domain

// CatalogPageSize is the default page size for the offset-paginated catalog
// endpoints (popular / latest / search) when no ?limit= override is supplied.
const CatalogPageSize = 30

// MaxCatalogPageSize caps a caller-supplied ?limit= page-size override so a
// single request can't demand an unbounded slice of the catalog.
const MaxCatalogPageSize = 100

// PaginationKind distinguishes the two supported pagination strategies.
type PaginationKind string

const (
	// PaginationOffset is skip/limit (1-based page) pagination.
	PaginationOffset PaginationKind = "offset"
	// PaginationCursor is opaque-cursor pagination.
	PaginationCursor PaginationKind = "cursor"
)

// Pagination is transport-agnostic pagination metadata. The HTTP layer emits it
// into response headers (see handler.writePagination). It models both offset
// (skip) and cursor pagination so one helper serves either strategy: read Kind,
// then the fields relevant to it.
type Pagination struct {
	Kind    PaginationKind
	HasNext bool
	PerPage int // page size / limit, for both kinds

	// Offset (skip) pagination.
	Page  int
	Total int // total matching rows; negative when unknown / not counted

	// Cursor pagination.
	NextCursor string
	PrevCursor string
}

// OffsetPagination builds offset/skip pagination metadata. Pass total < 0 when
// the total row count is unknown (not counted).
func OffsetPagination(page, perPage, total int, hasNext bool) Pagination {
	return Pagination{
		Kind:    PaginationOffset,
		HasNext: hasNext,
		PerPage: perPage,
		Page:    page,
		Total:   total,
	}
}

// CursorPagination builds cursor pagination metadata. Empty next/prev cursors
// are omitted from the response.
func CursorPagination(limit int, next, prev string, hasNext bool) Pagination {
	return Pagination{
		Kind:       PaginationCursor,
		HasNext:    hasNext,
		PerPage:    limit,
		NextCursor: next,
		PrevCursor: prev,
	}
}
