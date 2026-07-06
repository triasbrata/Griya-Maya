package d1

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// MediaRepo is the D1-backed catalog repository for the unified `media` entity
// (manga | video | novel) and its normalized taxonomies.
type MediaRepo struct {
	db Querier
}

// NewMediaRepo wires a MediaRepo over a D1 client.
func NewMediaRepo(db *Client) *MediaRepo {
	return &MediaRepo{db: db}
}

// concatSep is the group_concat delimiter used to pack a media's taxonomy names
// into a single flat column. It is the ASCII unit separator (0x1F), which never
// appears in a genre/author/artist name, so names carrying commas survive the
// round-trip. Reassembled by splitConcat.
const concatSep = "\x1f"

// mediaColumns selects the media row plus its normalized taxonomies. The
// taxonomies are reassembled per-row via correlated group_concat subqueries (not
// JOINs) so the flat D1 REST result set keeps exactly one row per media and
// LIMIT/OFFSET pagination stays correct.
const mediaColumns = `media.id, media.source_id, media.type, media.url, media.title,
        media.cover_url, media.description, media.status, media.updated_at,
        (SELECT group_concat(g.name, char(31))  FROM media_genre mg    JOIN genre g     ON g.id  = mg.genre_id    WHERE mg.media_id = media.id) AS genres,
        (SELECT group_concat(c.name, char(31))  FROM media_category mc  JOIN category c  ON c.id  = mc.category_id WHERE mc.media_id = media.id) AS categories,
        (SELECT group_concat(a.name, char(31))  FROM media_author ma    JOIN author a    ON a.id  = ma.author_id   WHERE ma.media_id = media.id) AS authors,
        (SELECT group_concat(ar.name, char(31)) FROM media_artist mr    JOIN artist ar   ON ar.id = mr.artist_id   WHERE mr.media_id = media.id) AS artists`

// List returns a page of catalog entries for a feed ("popular" | "latest"),
// honoring the catalog filter (sort/direction, type + genre/category filters).
func (r *MediaRepo) List(ctx context.Context, sourceID, order string, page, perPage int, filter domain.CatalogFilter) (domain.MediaPage, error) {
	page, perPage = normPage(page, perPage)

	feedDefault := "updated"
	if order == "popular" {
		feedDefault = "popular"
	}

	qb := newQueryBuilder()
	where := "media.source_id = " + qb.bind(sourceID)
	where += filterClauses(qb, filter)

	sql := "SELECT " + mediaColumns + " FROM media WHERE " + where +
		" ORDER BY " + orderByClause(filter, feedDefault) +
		" LIMIT " + qb.bind(perPage+1) + " OFFSET " + qb.bind((page-1)*perPage)

	rows, err := r.db.Query(ctx, sql, qb.params...)
	if err != nil {
		return domain.MediaPage{}, err
	}
	return toMediaPage(rows, page, perPage), nil
}

// Search matches title (LIKE) within a source, honoring the catalog filter.
func (r *MediaRepo) Search(ctx context.Context, sourceID, query string, page, perPage int, filter domain.CatalogFilter) (domain.MediaPage, error) {
	page, perPage = normPage(page, perPage)

	qb := newQueryBuilder()
	where := "media.source_id = " + qb.bind(sourceID) + " AND media.title LIKE " + qb.bind("%"+query+"%")
	where += filterClauses(qb, filter)

	// Search sorts by title order unless the caller picked a sort.
	feedDefault := "title"
	sql := "SELECT " + mediaColumns + " FROM media WHERE " + where +
		" ORDER BY " + orderByClause(filter, feedDefault) +
		" LIMIT " + qb.bind(perPage+1) + " OFFSET " + qb.bind((page-1)*perPage)

	rows, err := r.db.Query(ctx, sql, qb.params...)
	if err != nil {
		return domain.MediaPage{}, err
	}
	return toMediaPage(rows, page, perPage), nil
}

// Recommend ranks a source's media by how many of the requested genre slugs each
// entry shares (descending), breaking ties by the catalog's default popular
// ordering (popularity, then title). Media that share no requested genre, and any
// id in exclude, are omitted. genres must be non-empty — the service falls back to
// the popular feed otherwise, so an empty genre set never reaches here.
//
// The ranking runs entirely in SQL over the same normalized genre join tables the
// genre filter uses (media_genre → genre), reusing the OR-mode EXISTS clause for
// the "at least one shared genre" gate.
func (r *MediaRepo) Recommend(ctx context.Context, sourceID string, genres, exclude []string, page, perPage int) (domain.MediaPage, error) {
	page, perPage = normPage(page, perPage)

	genres = nonEmpty(genres)
	genreTT, _ := taxTableFor(domain.TaxonomyGenre)

	qb := newQueryBuilder()

	// overlap = how many requested genre slugs this media carries; drives ranking.
	overlap := overlapCount(qb, genreTT, genres)

	where := "media.source_id = " + qb.bind(sourceID)
	// Only rank media sharing at least one requested genre (zero-overlap dropped).
	where += includeClause(qb, genreTT, genres, domain.GenreModeOr)
	// Drop already-read / seed ids.
	if exc := nonEmpty(exclude); len(exc) > 0 {
		ph := make([]string, len(exc))
		for i, id := range exc {
			ph[i] = qb.bind(id)
		}
		where += " AND media.id NOT IN (" + strings.Join(ph, ", ") + ")"
	}

	sql := "SELECT " + mediaColumns + ", " + overlap + " AS overlap FROM media WHERE " + where +
		" ORDER BY overlap DESC, popularity DESC, title ASC" +
		" LIMIT " + qb.bind(perPage+1) + " OFFSET " + qb.bind((page-1)*perPage)

	rows, err := r.db.Query(ctx, sql, qb.params...)
	if err != nil {
		return domain.MediaPage{}, err
	}
	return toMediaPage(rows, page, perPage), nil
}

// overlapCount renders a correlated subquery counting how many of slugs a media
// carries in the genre taxonomy. Slugs are normalized with genreSlug so they line
// up with the stored slugs, exactly like includeClause/excludeClause.
func overlapCount(qb *queryBuilder, tt taxTable, slugs []string) string {
	ph := make([]string, len(slugs))
	for i, s := range slugs {
		ph[i] = qb.bind(genreSlug(s))
	}
	return "(SELECT COUNT(DISTINCT t.slug) FROM " + tt.join + " j JOIN " + tt.table +
		" t ON t.id = j." + tt.fk + " WHERE j.media_id = media.id AND t.slug IN (" + strings.Join(ph, ", ") + "))"
}

// Genres returns the distinct genres attached to a source's catalog, as
// filterable tags (slug + name), sorted alphabetically.
func (r *MediaRepo) Genres(ctx context.Context, sourceID string) ([]domain.Taxonomy, error) {
	return r.sourceTaxonomy(ctx, domain.TaxonomyGenre, sourceID)
}

// Categories returns the distinct categories attached to a source's catalog.
func (r *MediaRepo) Categories(ctx context.Context, sourceID string) ([]domain.Taxonomy, error) {
	return r.sourceTaxonomy(ctx, domain.TaxonomyCategory, sourceID)
}

// sourceTaxonomy lists the distinct tags of a kind linked to any media of a
// source. SQL does the dedup + sort.
func (r *MediaRepo) sourceTaxonomy(ctx context.Context, kind domain.TaxonomyKind, sourceID string) ([]domain.Taxonomy, error) {
	tt, _ := taxTableFor(kind)
	sel := "t.id AS id, t.name AS name"
	if tt.hasSlug {
		sel = "t.id AS id, t.slug AS slug, t.name AS name"
	}
	sql := "SELECT DISTINCT " + sel +
		" FROM " + tt.table + " t" +
		" JOIN " + tt.join + " j ON j." + tt.fk + " = t.id" +
		" JOIN media m ON m.id = j.media_id" +
		" WHERE m.source_id = ?1 ORDER BY t.name ASC"
	rows, err := r.db.Query(ctx, sql, sourceID)
	if err != nil {
		return nil, err
	}
	return taxonomyRows(kind, rows), nil
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

// filterClauses appends type/genre/category constraints to a WHERE clause.
func filterClauses(qb *queryBuilder, f domain.CatalogFilter) string {
	var b strings.Builder

	// TYPE is a first-class column: media.type IN (...).
	if types := nonEmpty(f.Types); len(types) > 0 {
		ph := make([]string, len(types))
		for i, t := range types {
			ph[i] = qb.bind(t)
		}
		b.WriteString(" AND media.type IN (" + strings.Join(ph, ", ") + ")")
	}

	genreTT, _ := taxTableFor(domain.TaxonomyGenre)
	categoryTT, _ := taxTableFor(domain.TaxonomyCategory)

	if inc := nonEmpty(f.IncludeGenres); len(inc) > 0 {
		b.WriteString(includeClause(qb, genreTT, inc, f.GenreMode))
	}
	if exc := nonEmpty(f.ExcludeGenres); len(exc) > 0 {
		b.WriteString(excludeClause(qb, genreTT, exc))
	}
	if inc := nonEmpty(f.IncludeCategories); len(inc) > 0 {
		b.WriteString(includeClause(qb, categoryTT, inc, f.GenreMode))
	}
	if exc := nonEmpty(f.ExcludeCategories); len(exc) > 0 {
		b.WriteString(excludeClause(qb, categoryTT, exc))
	}

	return b.String()
}

// includeClause requires membership in a taxonomy by slug. OR mode: the media
// carries ANY of the slugs (EXISTS). AND mode: it carries ALL of them
// (COUNT(DISTINCT matched) == requested count).
func includeClause(qb *queryBuilder, tt taxTable, slugs []string, mode domain.GenreMode) string {
	ph := make([]string, len(slugs))
	for i, s := range slugs {
		ph[i] = qb.bind(genreSlug(s))
	}
	in := strings.Join(ph, ", ")
	if mode == domain.GenreModeAnd {
		return " AND (SELECT COUNT(DISTINCT t.slug) FROM " + tt.join + " j JOIN " + tt.table +
			" t ON t.id = j." + tt.fk + " WHERE j.media_id = media.id AND t.slug IN (" + in + ")) = " +
			qb.bind(len(slugs))
	}
	return " AND EXISTS (SELECT 1 FROM " + tt.join + " j JOIN " + tt.table +
		" t ON t.id = j." + tt.fk + " WHERE j.media_id = media.id AND t.slug IN (" + in + "))"
}

// excludeClause forbids membership in ANY of the given slugs.
func excludeClause(qb *queryBuilder, tt taxTable, slugs []string) string {
	ph := make([]string, len(slugs))
	for i, s := range slugs {
		ph[i] = qb.bind(genreSlug(s))
	}
	return " AND NOT EXISTS (SELECT 1 FROM " + tt.join + " j JOIN " + tt.table +
		" t ON t.id = j." + tt.fk + " WHERE j.media_id = media.id AND t.slug IN (" + strings.Join(ph, ", ") + "))"
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

// genreSlug lowercases a name and hyphenates whitespace, matching the app's slug
// convention (and the migration's back-fill rule) so include/exclude filters
// line up on both sides.
func genreSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Join(strings.Fields(s), "-")
	return s
}

// Get returns a single media entry or domain.ErrNotFound.
func (r *MediaRepo) Get(ctx context.Context, id string) (domain.Media, error) {
	rows, err := r.db.Query(ctx,
		"SELECT "+mediaColumns+" FROM media WHERE id = ?1 LIMIT 1", id)
	if err != nil {
		return domain.Media{}, err
	}
	if len(rows) == 0 {
		return domain.Media{}, domain.ErrNotFound
	}
	return mediaFromRow(rows[0]), nil
}

// Chapters returns chapters for a media entry ordered by number.
func (r *MediaRepo) Chapters(ctx context.Context, mediaID string) ([]domain.Chapter, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, media_id, url, name, number, scanlator, date_upload, format
		 FROM chapter WHERE media_id = ?1 ORDER BY number ASC`, mediaID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Chapter, 0, len(rows))
	for _, row := range rows {
		out = append(out, chapterFromRow(row))
	}
	return out, nil
}

// ChapterByID returns a single chapter or domain.ErrNotFound.
func (r *MediaRepo) ChapterByID(ctx context.Context, id string) (domain.Chapter, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, media_id, url, name, number, scanlator, date_upload, format
		 FROM chapter WHERE id = ?1 LIMIT 1`, id)
	if err != nil {
		return domain.Chapter{}, err
	}
	if len(rows) == 0 {
		return domain.Chapter{}, domain.ErrNotFound
	}
	return chapterFromRow(rows[0]), nil
}

// Pages returns stored page rows (R2 keys) for a chapter, ordered by index.
func (r *MediaRepo) Pages(ctx context.Context, chapterID string) ([]domain.StoredPage, error) {
	rows, err := r.db.Query(ctx,
		`SELECT idx, r2_key, width, height, kind FROM page
		 WHERE chapter_id = ?1 ORDER BY idx ASC`, chapterID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StoredPage, 0, len(rows))
	for _, row := range rows {
		kind := strVal(row["kind"])
		if kind == "" {
			kind = domain.PageKindImage
		}
		out = append(out, domain.StoredPage{
			Index:  intVal(row["idx"]),
			R2Key:  strVal(row["r2_key"]),
			Width:  intVal(row["width"]),
			Height: intVal(row["height"]),
			Kind:   kind,
		})
	}
	return out, nil
}

// --- media write path ---

// CreateMedia inserts a media row (m.ID must be set) and its taxonomy links.
func (r *MediaRepo) CreateMedia(ctx context.Context, m domain.Media) error {
	mtype := m.Type
	if mtype == "" {
		mtype = domain.MediaManga
	}
	updated := m.UpdatedAt
	if updated.IsZero() {
		updated = time.Now()
	}
	if err := r.db.Exec(ctx,
		`INSERT INTO media (id, source_id, type, url, title, cover_url, description,
		    status, popularity, updated_at)
		 VALUES (?1,?2,?3,?4,?5,?6,?7,?8,0,?9)`,
		m.ID, m.SourceID, string(mtype), m.URL, m.Title, m.CoverURL, m.Description,
		string(m.Status), updated.Unix(),
	); err != nil {
		return err
	}
	return r.syncAllTaxonomies(ctx, m)
}

// UpdateMedia rewrites a media row and re-links its taxonomies.
func (r *MediaRepo) UpdateMedia(ctx context.Context, m domain.Media) error {
	if err := r.db.Exec(ctx,
		`UPDATE media SET type=?2, url=?3, title=?4, cover_url=?5, description=?6,
		    status=?7, updated_at=?8 WHERE id=?1`,
		m.ID, string(m.Type), m.URL, m.Title, m.CoverURL, m.Description,
		string(m.Status), time.Now().Unix(),
	); err != nil {
		return err
	}
	return r.syncAllTaxonomies(ctx, m)
}

// SetMediaCover rewrites only the cover_url column (the async cover mirror swaps
// an external URL for the stored R2 key without touching taxonomies).
func (r *MediaRepo) SetMediaCover(ctx context.Context, mediaID, coverURL string) error {
	return r.db.Exec(ctx,
		`UPDATE media SET cover_url=?2, updated_at=?3 WHERE id=?1`,
		mediaID, coverURL, time.Now().Unix())
}

// DeleteMedia removes a media entry and everything hanging off it (chapters,
// their pages, and all taxonomy links). Children are deleted explicitly rather
// than relying on ON DELETE CASCADE, which requires PRAGMA foreign_keys=ON.
func (r *MediaRepo) DeleteMedia(ctx context.Context, id string) error {
	stmts := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM page WHERE chapter_id IN (SELECT id FROM chapter WHERE media_id=?1)`, []any{id}},
		{`DELETE FROM chapter WHERE media_id=?1`, []any{id}},
		{`DELETE FROM media_genre WHERE media_id=?1`, []any{id}},
		{`DELETE FROM media_category WHERE media_id=?1`, []any{id}},
		{`DELETE FROM media_author WHERE media_id=?1`, []any{id}},
		{`DELETE FROM media_artist WHERE media_id=?1`, []any{id}},
		{`DELETE FROM media WHERE id=?1`, []any{id}},
	}
	for _, s := range stmts {
		if err := r.db.Exec(ctx, s.sql, s.args...); err != nil {
			return err
		}
	}
	return nil
}

// syncAllTaxonomies re-links a media's genres/categories/authors/artists from
// the display-name arrays on m (upserting tags as needed).
func (r *MediaRepo) syncAllTaxonomies(ctx context.Context, m domain.Media) error {
	if err := r.syncTaxonomy(ctx, m.ID, domain.TaxonomyGenre, m.Genres); err != nil {
		return err
	}
	if err := r.syncTaxonomy(ctx, m.ID, domain.TaxonomyCategory, m.Categories); err != nil {
		return err
	}
	if err := r.syncTaxonomy(ctx, m.ID, domain.TaxonomyAuthor, m.Authors); err != nil {
		return err
	}
	return r.syncTaxonomy(ctx, m.ID, domain.TaxonomyArtist, m.Artists)
}

// syncTaxonomy rewrites a media's links for one taxonomy kind: it clears the
// existing join rows and re-inserts one per (upserted) name.
func (r *MediaRepo) syncTaxonomy(ctx context.Context, mediaID string, kind domain.TaxonomyKind, names []string) error {
	tt, _ := taxTableFor(kind)
	if err := r.db.Exec(ctx, "DELETE FROM "+tt.join+" WHERE media_id=?1", mediaID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		id, err := r.upsertTag(ctx, kind, name)
		if err != nil {
			return err
		}
		if err := r.db.Exec(ctx,
			"INSERT OR IGNORE INTO "+tt.join+" (media_id, "+tt.fk+") VALUES (?1,?2)",
			mediaID, id); err != nil {
			return err
		}
	}
	return nil
}

// upsertTag ensures a tag row exists for name and returns its id.
func (r *MediaRepo) upsertTag(ctx context.Context, kind domain.TaxonomyKind, name string) (string, error) {
	tt, _ := taxTableFor(kind)
	if tt.hasSlug {
		slug := genreSlug(name)
		if err := r.db.Exec(ctx,
			"INSERT OR IGNORE INTO "+tt.table+" (id, slug, name) VALUES (?1,?2,?3)",
			uuid.NewString(), slug, name); err != nil {
			return "", err
		}
		return r.scalarID(ctx, "SELECT id FROM "+tt.table+" WHERE slug=?1", slug)
	}
	if err := r.db.Exec(ctx,
		"INSERT OR IGNORE INTO "+tt.table+" (id, name) VALUES (?1,?2)",
		uuid.NewString(), name); err != nil {
		return "", err
	}
	return r.scalarID(ctx, "SELECT id FROM "+tt.table+" WHERE name=?1", name)
}

func (r *MediaRepo) scalarID(ctx context.Context, sql string, arg any) (string, error) {
	rows, err := r.db.Query(ctx, sql, arg)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", domain.ErrNotFound
	}
	return strVal(rows[0]["id"]), nil
}

// --- chapter write path ---

// CreateChapter inserts a chapter row (c.ID must be set).
func (r *MediaRepo) CreateChapter(ctx context.Context, c domain.Chapter) error {
	return r.db.Exec(ctx,
		`INSERT INTO chapter (id, media_id, url, name, number, scanlator, date_upload, format)
		 VALUES (?1,?2,?3,?4,?5,?6,?7,?8)`,
		c.ID, c.MediaID, c.URL, c.Name, c.Number, c.Scanlator, c.DateUpload.Unix(), c.Format,
	)
}

// UpdateChapter rewrites a chapter's mutable fields (media_id is fixed).
func (r *MediaRepo) UpdateChapter(ctx context.Context, c domain.Chapter) error {
	return r.db.Exec(ctx,
		`UPDATE chapter SET url=?2, name=?3, number=?4, scanlator=?5, date_upload=?6, format=?7
		 WHERE id=?1`,
		c.ID, c.URL, c.Name, c.Number, c.Scanlator, c.DateUpload.Unix(), c.Format,
	)
}

// DeleteChapter removes a chapter and its pages.
func (r *MediaRepo) DeleteChapter(ctx context.Context, id string) error {
	if err := r.db.Exec(ctx, `DELETE FROM page WHERE chapter_id=?1`, id); err != nil {
		return err
	}
	return r.db.Exec(ctx, `DELETE FROM chapter WHERE id=?1`, id)
}

// DeletePage removes a single page row identified by its chapter and index.
func (r *MediaRepo) DeletePage(ctx context.Context, chapterID string, idx int) error {
	return r.db.Exec(ctx, `DELETE FROM page WHERE chapter_id=?1 AND idx=?2`, chapterID, idx)
}

// PageKeysForMedia returns the R2 keys of every page across all of a media
// entry's chapters (used to schedule R2 cleanup when the media is deleted).
func (r *MediaRepo) PageKeysForMedia(ctx context.Context, mediaID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT r2_key FROM page
		 WHERE chapter_id IN (SELECT id FROM chapter WHERE media_id=?1)`, mediaID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if k := strVal(row["r2_key"]); k != "" {
			out = append(out, k)
		}
	}
	return out, nil
}

// --- taxonomy management (genre/category/author/artist) ---

// ListTaxonomy returns all tags of a kind, ordered by name.
func (r *MediaRepo) ListTaxonomy(ctx context.Context, kind domain.TaxonomyKind) ([]domain.Taxonomy, error) {
	tt, ok := taxTableFor(kind)
	if !ok {
		return nil, domain.ErrInvalidInput
	}
	cols := "id, name"
	if tt.hasSlug {
		cols = "id, slug, name"
	}
	rows, err := r.db.Query(ctx, "SELECT "+cols+" FROM "+tt.table+" ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	return taxonomyRows(kind, rows), nil
}

// CreateTaxonomy inserts a tag (idempotent by slug/name) and returns it.
func (r *MediaRepo) CreateTaxonomy(ctx context.Context, kind domain.TaxonomyKind, name string) (domain.Taxonomy, error) {
	if _, ok := taxTableFor(kind); !ok {
		return domain.Taxonomy{}, domain.ErrInvalidInput
	}
	id, err := r.upsertTag(ctx, kind, name)
	if err != nil {
		return domain.Taxonomy{}, err
	}
	return domain.Taxonomy{ID: id, Slug: slugFor(kind, name), Name: name, Kind: kind}, nil
}

// UpdateTaxonomy renames a tag (and re-slugs genre/category) by id.
func (r *MediaRepo) UpdateTaxonomy(ctx context.Context, kind domain.TaxonomyKind, id, name string) (domain.Taxonomy, error) {
	tt, ok := taxTableFor(kind)
	if !ok {
		return domain.Taxonomy{}, domain.ErrInvalidInput
	}
	if tt.hasSlug {
		if err := r.db.Exec(ctx,
			"UPDATE "+tt.table+" SET name=?2, slug=?3 WHERE id=?1", id, name, genreSlug(name)); err != nil {
			return domain.Taxonomy{}, err
		}
	} else if err := r.db.Exec(ctx,
		"UPDATE "+tt.table+" SET name=?2 WHERE id=?1", id, name); err != nil {
		return domain.Taxonomy{}, err
	}
	return domain.Taxonomy{ID: id, Slug: slugFor(kind, name), Name: name, Kind: kind}, nil
}

// DeleteTaxonomy removes a tag and its media links.
func (r *MediaRepo) DeleteTaxonomy(ctx context.Context, kind domain.TaxonomyKind, id string) error {
	tt, ok := taxTableFor(kind)
	if !ok {
		return domain.ErrInvalidInput
	}
	if err := r.db.Exec(ctx, "DELETE FROM "+tt.join+" WHERE "+tt.fk+"=?1", id); err != nil {
		return err
	}
	return r.db.Exec(ctx, "DELETE FROM "+tt.table+" WHERE id=?1", id)
}

// --- taxonomy table routing (fixed allowlist; safe to interpolate) ---

type taxTable struct {
	table   string
	join    string
	fk      string
	hasSlug bool
}

func taxTableFor(kind domain.TaxonomyKind) (taxTable, bool) {
	switch kind {
	case domain.TaxonomyGenre:
		return taxTable{"genre", "media_genre", "genre_id", true}, true
	case domain.TaxonomyCategory:
		return taxTable{"category", "media_category", "category_id", true}, true
	case domain.TaxonomyAuthor:
		return taxTable{"author", "media_author", "author_id", false}, true
	case domain.TaxonomyArtist:
		return taxTable{"artist", "media_artist", "artist_id", false}, true
	}
	return taxTable{}, false
}

func slugFor(kind domain.TaxonomyKind, name string) string {
	if kind.HasSlug() {
		return genreSlug(name)
	}
	return ""
}

func taxonomyRows(kind domain.TaxonomyKind, rows []map[string]any) []domain.Taxonomy {
	out := make([]domain.Taxonomy, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Taxonomy{
			ID:   strVal(row["id"]),
			Slug: strVal(row["slug"]),
			Name: strVal(row["name"]),
			Kind: kind,
		})
	}
	return out
}

// --- row mapping ---

func normPage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 30
	}
	return page, perPage
}

func toMediaPage(rows []map[string]any, page, perPage int) domain.MediaPage {
	hasNext := len(rows) > perPage
	if hasNext {
		rows = rows[:perPage]
	}
	items := make([]domain.Media, 0, len(rows))
	for _, row := range rows {
		items = append(items, mediaFromRow(row))
	}
	return domain.MediaPage{Items: items, HasNext: hasNext, Page: page}
}

func mediaFromRow(row map[string]any) domain.Media {
	mtype := domain.MediaType(strVal(row["type"]))
	if mtype == "" {
		mtype = domain.MediaManga
	}
	m := domain.Media{
		ID:          strVal(row["id"]),
		SourceID:    strVal(row["source_id"]),
		Type:        mtype,
		URL:         strVal(row["url"]),
		Title:       strVal(row["title"]),
		CoverURL:    strVal(row["cover_url"]),
		Description: strVal(row["description"]),
		Status:      domain.MediaStatus(strVal(row["status"])),
		Genres:      splitConcat(strVal(row["genres"])),
		Categories:  splitConcat(strVal(row["categories"])),
		Authors:     splitConcat(strVal(row["authors"])),
		Artists:     splitConcat(strVal(row["artists"])),
	}
	m.UpdatedAt = timeVal(row["updated_at"])
	return m
}

func chapterFromRow(row map[string]any) domain.Chapter {
	return domain.Chapter{
		ID:         strVal(row["id"]),
		MediaID:    strVal(row["media_id"]),
		URL:        strVal(row["url"]),
		Name:       strVal(row["name"]),
		Number:     floatVal(row["number"]),
		Scanlator:  strVal(row["scanlator"]),
		DateUpload: timeVal(row["date_upload"]),
		Format:     strVal(row["format"]),
	}
}

// splitConcat unpacks a group_concat(name, char(31)) column into a trimmed,
// non-empty slice. Returns nil for an empty/absent column.
func splitConcat(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, concatSep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
