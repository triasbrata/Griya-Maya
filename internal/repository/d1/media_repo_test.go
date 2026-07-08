package d1

import (
	"strings"
	"testing"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

func TestGenreSlug(t *testing.T) {
	cases := map[string]string{
		"Martial Arts": "martial-arts",
		"  Sci-Fi  ":   "sci-fi",
		"Action":       "action",
	}
	for in, want := range cases {
		if got := genreSlug(in); got != want {
			t.Errorf("genreSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilterClausesTypesSubTypes(t *testing.T) {
	qb := newQueryBuilder()
	f := domain.CatalogFilter{
		Types:    []string{"manga", ""},        // empty dropped
		SubTypes: []string{"manhwa", "manhua"}, // first-class column filter
	}
	clause := filterClauses(qb, f)

	// Type is a first-class column filter.
	if !strings.Contains(clause, "media.type IN (") {
		t.Errorf("expected media.type IN clause, got %q", clause)
	}
	// Sub-type is likewise a first-class column filter (no join).
	if !strings.Contains(clause, "media.sub_type IN (") {
		t.Errorf("expected media.sub_type IN clause, got %q", clause)
	}
	// Category filtering was removed entirely — no taxonomy joins here.
	if strings.Contains(clause, "media_category") || strings.Contains(clause, "EXISTS") {
		t.Errorf("category filtering must be gone, got %q", clause)
	}
	// Params: type(1) + sub_type[manhwa, manhua](2) = 3.
	if len(qb.params) != 3 {
		t.Fatalf("expected 3 bound params, got %d (%v)", len(qb.params), qb.params)
	}
	if qb.params[0] != "manga" {
		t.Errorf("first param should be the type, got %v", qb.params[0])
	}
	// Sub-types are bound verbatim (already canonical slugs).
	assertContains(t, qb.params, "manhwa")
	assertContains(t, qb.params, "manhua")
}

func assertContains(t *testing.T, params []any, want any) {
	t.Helper()
	for _, p := range params {
		if p == want {
			return
		}
	}
	t.Errorf("params %v missing %v", params, want)
}

func TestFilterClausesEmpty(t *testing.T) {
	qb := newQueryBuilder()
	if clause := filterClauses(qb, domain.CatalogFilter{}); clause != "" {
		t.Errorf("empty filter should add no clauses, got %q", clause)
	}
	if len(qb.params) != 0 {
		t.Errorf("empty filter should bind nothing, got %d", len(qb.params))
	}
}

func TestOrderByClause(t *testing.T) {
	cases := []struct {
		name        string
		filter      domain.CatalogFilter
		feedDefault string
		want        string
	}{
		{"feed default popular", domain.CatalogFilter{}, "popular", "popularity DESC, title ASC"},
		{"feed default latest", domain.CatalogFilter{}, "updated", "updated_at DESC, title ASC"},
		{"explicit title asc", domain.CatalogFilter{Sort: "title", Ascending: true}, "popular", "title ASC"},
		{"rating falls back to popularity", domain.CatalogFilter{Sort: "rating"}, "updated", "popularity DESC, title ASC"},
		{"ascending flips direction", domain.CatalogFilter{Sort: "updated", Ascending: true}, "popular", "updated_at ASC, title ASC"},
	}
	for _, tc := range cases {
		if got := orderByClause(tc.filter, tc.feedDefault); got != tc.want {
			t.Errorf("%s: orderByClause = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestBindIsPositional(t *testing.T) {
	qb := newQueryBuilder()
	if p := qb.bind("a"); p != "?1" {
		t.Errorf("first bind = %q, want ?1", p)
	}
	if p := qb.bind("b"); p != "?2" {
		t.Errorf("second bind = %q, want ?2", p)
	}
}

func TestTaxTableFor(t *testing.T) {
	for _, kind := range []domain.TaxonomyKind{
		domain.TaxonomyGenre, domain.TaxonomyAuthor, domain.TaxonomyArtist,
	} {
		tt, ok := taxTableFor(kind)
		if !ok || tt.table == "" || tt.join == "" || tt.fk == "" {
			t.Errorf("taxTableFor(%q) = %+v, ok=%v", kind, tt, ok)
		}
	}
	// genre is re-introduced; it must resolve to the genre table.
	if tt, ok := taxTableFor(domain.TaxonomyGenre); !ok || tt.table != "genre" || tt.join != "media_genre" {
		t.Errorf("genre kind should resolve to genre table, got %+v ok=%v", tt, ok)
	}
	// category was retired; it must no longer resolve.
	if tt, ok := taxTableFor(domain.TaxonomyKind("category")); ok {
		t.Errorf("retired category kind should not resolve, got %+v", tt)
	}
	if tt, ok := taxTableFor(domain.TaxonomyKind("bogus")); ok {
		t.Errorf("unknown kind should not resolve, got %+v", tt)
	}
	if !mustTax(t, domain.TaxonomyGenre).hasSlug {
		t.Error("genre should have slug")
	}
	if mustTax(t, domain.TaxonomyAuthor).hasSlug {
		t.Error("author should not have slug")
	}
}

func mustTax(t *testing.T, kind domain.TaxonomyKind) taxTable {
	t.Helper()
	tt, ok := taxTableFor(kind)
	if !ok {
		t.Fatalf("taxTableFor(%q) not found", kind)
	}
	return tt
}
