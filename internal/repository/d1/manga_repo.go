package d1

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// MangaRepo is the D1-backed catalog repository.
type MangaRepo struct {
	db *Client
}

// NewMangaRepo wires a MangaRepo over a D1 client.
func NewMangaRepo(db *Client) *MangaRepo {
	return &MangaRepo{db: db}
}

const mangaColumns = `id, source_id, url, title, cover_url, author, artist, description,
	        genres, status, updated_at`

// List returns a page of catalog entries for a feed ("popular" | "latest"),
// honoring the catalog filter (sort/direction, type + genre include/exclude).
func (r *MangaRepo) List(ctx context.Context, sourceID, order string, page, perPage int, filter domain.CatalogFilter) (domain.MangaPage, error) {
	page, perPage = normPage(page, perPage)

	feedDefault := "updated"
	if order == "popular" {
		feedDefault = "popular"
	}

	qb := newQueryBuilder()
	where := "source_id = " + qb.bind(sourceID)
	where += filterClauses(qb, filter)

	sql := "SELECT " + mangaColumns + " FROM manga WHERE " + where +
		" ORDER BY " + orderByClause(filter, feedDefault) +
		" LIMIT " + qb.bind(perPage+1) + " OFFSET " + qb.bind((page-1)*perPage)

	rows, err := r.db.Query(ctx, sql, qb.params...)
	if err != nil {
		return domain.MangaPage{}, err
	}
	return toMangaPage(rows, page, perPage), nil
}

// Search matches title (LIKE) within a source, honoring the catalog filter.
func (r *MangaRepo) Search(ctx context.Context, sourceID, query string, page, perPage int, filter domain.CatalogFilter) (domain.MangaPage, error) {
	page, perPage = normPage(page, perPage)

	qb := newQueryBuilder()
	where := "source_id = " + qb.bind(sourceID) + " AND title LIKE " + qb.bind("%"+query+"%")
	where += filterClauses(qb, filter)

	// Search sorts by relevance-ish title order unless the caller picked a sort.
	feedDefault := "title"
	sql := "SELECT " + mangaColumns + " FROM manga WHERE " + where +
		" ORDER BY " + orderByClause(filter, feedDefault) +
		" LIMIT " + qb.bind(perPage+1) + " OFFSET " + qb.bind((page-1)*perPage)

	rows, err := r.db.Query(ctx, sql, qb.params...)
	if err != nil {
		return domain.MangaPage{}, err
	}
	return toMangaPage(rows, page, perPage), nil
}

// Genres returns the distinct genres seen across a source's catalog, as
// filterable tags (slug + display name), sorted alphabetically.
func (r *MangaRepo) Genres(ctx context.Context, sourceID string) ([]domain.GenreTag, error) {
	rows, err := r.db.Query(ctx,
		`SELECT genres FROM manga WHERE source_id = ?1 AND genres IS NOT NULL AND genres <> ''`,
		sourceID)
	if err != nil {
		return nil, err
	}
	seen := map[string]domain.GenreTag{}
	for _, row := range rows {
		for _, name := range strings.Split(strVal(row["genres"]), ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			slug := genreSlug(name)
			if _, ok := seen[slug]; !ok {
				seen[slug] = domain.GenreTag{Slug: slug, Name: name}
			}
		}
	}
	out := make([]domain.GenreTag, 0, len(seen))
	for _, g := range seen {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// --- filter → SQL helpers ---

// queryBuilder accumulates positional params so dynamic WHERE fragments can bind
// values as ?1..?N in the order they are appended.
type queryBuilder struct {
	params []any
}

func newQueryBuilder() *queryBuilder { return &queryBuilder{} }

// bind records a value and returns its positional placeholder (e.g. "?3").
func (qb *queryBuilder) bind(v any) string {
	qb.params = append(qb.params, v)
	return "?" + strconv.Itoa(len(qb.params))
}

// filterClauses appends the type/genre constraints to a WHERE clause. Genres are
// stored comma-separated, so membership is matched with LIKE on a normalized
// ",a,b,c," wrapping (guards against substring false-positives like "art"⊂"martial").
func filterClauses(qb *queryBuilder, f domain.CatalogFilter) string {
	var b strings.Builder

	// Type tags have no dedicated column; match them against genres (OR).
	if types := nonEmpty(f.Types); len(types) > 0 {
		b.WriteString(" AND (")
		for i, t := range types {
			if i > 0 {
				b.WriteString(" OR ")
			}
			b.WriteString(genreLike(qb, t))
		}
		b.WriteString(")")
	}

	if inc := nonEmpty(f.IncludeGenres); len(inc) > 0 {
		joiner := " OR "
		if f.GenreMode == domain.GenreModeAnd {
			joiner = " AND "
		}
		b.WriteString(" AND (")
		for i, g := range inc {
			if i > 0 {
				b.WriteString(joiner)
			}
			b.WriteString(genreLike(qb, g))
		}
		b.WriteString(")")
	}

	for _, g := range nonEmpty(f.ExcludeGenres) {
		b.WriteString(" AND NOT ")
		b.WriteString(genreLike(qb, g))
	}

	return b.String()
}

// genreLike builds a case-insensitive membership test against the comma-separated
// genres column. Both the stored genres and the query value are collapsed to a
// space/hyphen-free token so a slug ("martial-arts") and a display name
// ("Martial Arts") compare equal. Commas wrap the column to avoid substring
// false-positives (e.g. "art" ⊄ "martialarts").
func genreLike(qb *queryBuilder, value string) string {
	token := genreToken(value)
	col := "LOWER(REPLACE(REPLACE(',' || genres || ',', ' ', ''), '-', ''))"
	return col + " LIKE " + qb.bind("%,"+token+",%")
}

// genreToken normalizes a genre name or slug to a comparable token: lowercase
// with spaces and hyphens removed.
func genreToken(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

// orderByClause renders a safe ORDER BY (column names are from a fixed allowlist,
// never bound params). Ties break by title for stable pagination.
func orderByClause(f domain.CatalogFilter, feedDefault string) string {
	col := f.SortColumn(feedDefault)
	dir := "DESC"
	if f.Ascending {
		dir = "ASC"
	}
	if col == "title" {
		return "title " + dir
	}
	return col + " " + dir + ", title ASC"
}

func nonEmpty(values []string) []string {
	out := values[:0:0]
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

// genreSlug lowercases a genre name and hyphenates whitespace, matching the app's
// slug convention so include/exclude filters line up on both sides.
func genreSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Join(strings.Fields(s), "-")
	return s
}

// Get returns a single manga or domain.ErrNotFound.
func (r *MangaRepo) Get(ctx context.Context, id string) (domain.Manga, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, source_id, url, title, cover_url, author, artist, description,
		        genres, status, updated_at
		 FROM manga WHERE id = ?1 LIMIT 1`, id)
	if err != nil {
		return domain.Manga{}, err
	}
	if len(rows) == 0 {
		return domain.Manga{}, domain.ErrNotFound
	}
	return mangaFromRow(rows[0]), nil
}

// Chapters returns chapters for a manga ordered by number.
func (r *MangaRepo) Chapters(ctx context.Context, mangaID string) ([]domain.Chapter, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, manga_id, url, name, number, scanlator, date_upload, format
		 FROM chapter WHERE manga_id = ?1 ORDER BY number ASC`, mangaID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Chapter, 0, len(rows))
	for _, row := range rows {
		out = append(out, chapterFromRow(row))
	}
	return out, nil
}

// Pages returns stored page rows (R2 keys) for a chapter, ordered by index.
func (r *MangaRepo) Pages(ctx context.Context, chapterID string) ([]domain.StoredPage, error) {
	rows, err := r.db.Query(ctx,
		`SELECT idx, r2_key, width, height FROM page
		 WHERE chapter_id = ?1 ORDER BY idx ASC`, chapterID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StoredPage, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.StoredPage{
			Index:  intVal(row["idx"]),
			R2Key:  strVal(row["r2_key"]),
			Width:  intVal(row["width"]),
			Height: intVal(row["height"]),
		})
	}
	return out, nil
}

func normPage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 30
	}
	return page, perPage
}

func toMangaPage(rows []map[string]any, page, perPage int) domain.MangaPage {
	hasNext := len(rows) > perPage
	if hasNext {
		rows = rows[:perPage]
	}
	items := make([]domain.Manga, 0, len(rows))
	for _, row := range rows {
		items = append(items, mangaFromRow(row))
	}
	return domain.MangaPage{Items: items, HasNext: hasNext, Page: page}
}

func mangaFromRow(row map[string]any) domain.Manga {
	m := domain.Manga{
		ID:          strVal(row["id"]),
		SourceID:    strVal(row["source_id"]),
		URL:         strVal(row["url"]),
		Title:       strVal(row["title"]),
		CoverURL:    strVal(row["cover_url"]),
		Author:      strVal(row["author"]),
		Artist:      strVal(row["artist"]),
		Description: strVal(row["description"]),
		Status:      domain.MangaStatus(strVal(row["status"])),
	}
	if g := strVal(row["genres"]); g != "" {
		m.Genres = strings.Split(g, ",")
	}
	m.UpdatedAt = timeVal(row["updated_at"])
	return m
}

func chapterFromRow(row map[string]any) domain.Chapter {
	return domain.Chapter{
		ID:         strVal(row["id"]),
		MangaID:    strVal(row["manga_id"]),
		URL:        strVal(row["url"]),
		Name:       strVal(row["name"]),
		Number:     floatVal(row["number"]),
		Scanlator:  strVal(row["scanlator"]),
		DateUpload: timeVal(row["date_upload"]),
		Format:     strVal(row["format"]),
	}
}

// --- D1 JSON value helpers (values arrive as string / float64 / nil) ---

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func floatVal(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func intVal(v any) int {
	return int(floatVal(v))
}

// timeVal parses an integer unix-seconds column or an RFC3339 string.
func timeVal(v any) time.Time {
	switch t := v.(type) {
	case float64:
		if t == 0 {
			return time.Time{}
		}
		return time.Unix(int64(t), 0).UTC()
	case string:
		if t == "" {
			return time.Time{}
		}
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
