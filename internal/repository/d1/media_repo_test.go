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

func TestFilterClausesTypesGenresCategories(t *testing.T) {
	qb := newQueryBuilder()
	f := domain.CatalogFilter{
		Types:             []string{"video", ""}, // empty dropped
		IncludeGenres:     []string{"Action", "martial-arts"},
		ExcludeGenres:     []string{"Ecchi"},
		IncludeCategories: []string{"Webtoon"},
		GenreMode:         domain.GenreModeAnd,
	}
	clause := filterClauses(qb, f)

	// Type is a first-class column filter.
	if !strings.Contains(clause, "media.type IN (") {
		t.Errorf("expected media.type IN clause, got %q", clause)
	}
	// AND mode uses a COUNT(DISTINCT ...) = N check for included genres.
	if !strings.Contains(clause, "COUNT(DISTINCT t.slug)") {
		t.Errorf("expected COUNT(DISTINCT) for AND mode, got %q", clause)
	}
	// Category include joins the category taxonomy.
	if !strings.Contains(clause, "media_category") {
		t.Errorf("expected media_category join, got %q", clause)
	}
	// Exclusion emits NOT EXISTS.
	if !strings.Contains(clause, "NOT EXISTS") {
		t.Errorf("expected NOT EXISTS for excluded genre, got %q", clause)
	}
	// Params: type(1) + genre-include AND [action, martial-arts, count] (3)
	// + genre-exclude [ecchi] (1) + category-include AND [webtoon, count] (2) = 7.
	if len(qb.params) != 7 {
		t.Fatalf("expected 7 bound params, got %d (%v)", len(qb.params), qb.params)
	}
	if qb.params[0] != "video" {
		t.Errorf("first param should be the type, got %v", qb.params[0])
	}
	// Genre values are bound as normalized slugs.
	assertContains(t, qb.params, "action")
	assertContains(t, qb.params, "martial-arts")
	assertContains(t, qb.params, "ecchi")
	assertContains(t, qb.params, "webtoon")
	assertContains(t, qb.params, 2) // AND-mode count
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

func TestFilterClausesOrModeUsesExists(t *testing.T) {
	qb := newQueryBuilder()
	clause := filterClauses(qb, domain.CatalogFilter{
		IncludeGenres: []string{"action"},
		GenreMode:     domain.GenreModeOr,
	})
	if !strings.Contains(clause, "EXISTS (SELECT 1 FROM media_genre") {
		t.Errorf("OR mode should use EXISTS, got %q", clause)
	}
	if strings.Contains(clause, "COUNT(DISTINCT") {
		t.Errorf("OR mode should not use COUNT, got %q", clause)
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
		domain.TaxonomyGenre, domain.TaxonomyCategory, domain.TaxonomyAuthor, domain.TaxonomyArtist,
	} {
		tt, ok := taxTableFor(kind)
		if !ok || tt.table == "" || tt.join == "" || tt.fk == "" {
			t.Errorf("taxTableFor(%q) = %+v, ok=%v", kind, tt, ok)
		}
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
