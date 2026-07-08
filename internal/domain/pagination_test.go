package domain

import "testing"

func TestOffsetPagination(t *testing.T) {
	p := OffsetPagination(2, CatalogPageSize, 90, true)
	if p.Kind != PaginationOffset {
		t.Errorf("Kind = %q, want %q", p.Kind, PaginationOffset)
	}
	if p.Page != 2 || p.PerPage != CatalogPageSize || p.Total != 90 || !p.HasNext {
		t.Errorf("unexpected pagination: %+v", p)
	}
	// A negative total signals "unknown / not counted" and is carried through.
	if got := OffsetPagination(1, 30, -1, false); got.Total != -1 || got.HasNext {
		t.Errorf("unknown-total case: %+v", got)
	}
}

func TestCursorPagination(t *testing.T) {
	p := CursorPagination(30, "next-cur", "prev-cur", true)
	if p.Kind != PaginationCursor {
		t.Errorf("Kind = %q, want %q", p.Kind, PaginationCursor)
	}
	if p.PerPage != 30 || p.NextCursor != "next-cur" || p.PrevCursor != "prev-cur" || !p.HasNext {
		t.Errorf("unexpected pagination: %+v", p)
	}
}
