package d1

import (
	"strings"
	"testing"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

func TestGenreToken(t *testing.T) {
	cases := map[string]string{
		"Martial Arts": "martialarts",
		"martial-arts": "martialarts",
		"  Sci-Fi  ":   "scifi",
		"Action":       "action",
	}
	for in, want := range cases {
		if got := genreToken(in); got != want {
			t.Errorf("genreToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenreSlug(t *testing.T) {
	if got := genreSlug("Martial Arts"); got != "martial-arts" {
		t.Errorf("genreSlug = %q, want martial-arts", got)
	}
}

func TestFilterClausesTypesAndGenres(t *testing.T) {
	qb := newQueryBuilder()
	f := domain.CatalogFilter{
		Types:         []string{"manhwa", ""},
		IncludeGenres: []string{"Action", "martial-arts"},
		ExcludeGenres: []string{"Ecchi"},
		GenreMode:     domain.GenreModeAnd,
	}
	clause := filterClauses(qb, f)

	// Types → one OR group; empty type dropped.
	if !strings.Contains(clause, "AND (") {
		t.Fatalf("expected a grouped clause, got %q", clause)
	}
	// AND mode joins included genres with AND.
	if !strings.Contains(clause, " AND ") {
		t.Errorf("expected AND joiner in include group, got %q", clause)
	}
	// Exclusion emits NOT.
	if !strings.Contains(clause, "AND NOT ") {
		t.Errorf("expected NOT for excluded genre, got %q", clause)
	}
	// Params: 1 type + 2 include + 1 exclude = 4 bound values.
	if len(qb.params) != 4 {
		t.Errorf("expected 4 bound params, got %d (%v)", len(qb.params), qb.params)
	}
	// Bound values are the wrapped, tokenized membership patterns.
	if qb.params[1] != "%,action,%" {
		t.Errorf("expected include token pattern, got %v", qb.params[1])
	}
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
