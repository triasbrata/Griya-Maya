package d1

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/repository/d1/mocks"
)

// anyN returns n mock.Anything matchers for spreading into a variadic EXPECT.
func anyN(n int) []any {
	a := make([]any, n)
	for i := range a {
		a[i] = mock.Anything
	}
	return a
}

// mediaRow builds a flat media result row. sub_type is a first-class column;
// the remaining taxonomy columns are packed with the concatSep (0x1F) delimiter
// the repo reassembles.
func mediaRow(id, subType string) map[string]any {
	return map[string]any{
		"id": id, "source_id": "src", "type": "manga", "sub_type": subType,
		"url": "u/" + id, "title": "T-" + id,
		"cover_url": "", "description": "", "status": "ongoing", "updated_at": float64(0),
		"genres": "", "authors": "", "artists": "",
	}
}

func TestMediaRepo_List_MapsRowsAndPaginates(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var gotSQL string
	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(3)...).RunAndReturn(
		func(_ context.Context, sql string, _ ...any) ([]map[string]any, error) {
			gotSQL = sql
			return []map[string]any{mediaRow("m1", "manhwa"), mediaRow("m2", "manhua")}, nil
		})

	got, err := repo.List(context.Background(), "src", "popular", 1, 1, domain.CatalogFilter{})
	require.NoError(t, err)
	assert.True(t, got.HasNext)
	require.Len(t, got.Items, 1) // trimmed to perPage
	assert.Equal(t, "m1", got.Items[0].ID)
	assert.Equal(t, domain.MediaManga, got.Items[0].Type)
	assert.Equal(t, "manhwa", got.Items[0].SubType)
	assert.Contains(t, gotSQL, "FROM media")
	assert.Contains(t, gotSQL, "ORDER BY popularity DESC")
}

func TestMediaRepo_List_PropagatesError(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	wantErr := errors.New("d1 boom")

	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(3)...).Return(nil, wantErr)

	_, err := repo.List(context.Background(), "src", "latest", 1, 1, domain.CatalogFilter{})
	assert.ErrorIs(t, err, wantErr)
}

func TestMediaRepo_Recommend_RanksBySubType(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var gotSQL string
	var params []any
	// source(1) + sub_type IN[manhwa, manhua](2) + exclude(1) + limit + offset = 6.
	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(6)...).RunAndReturn(
		func(_ context.Context, sql string, p ...any) ([]map[string]any, error) {
			gotSQL, params = sql, p
			return []map[string]any{mediaRow("m1", "manhwa"), mediaRow("m2", "manhua")}, nil
		})

	got, err := repo.Recommend(context.Background(), "src",
		[]string{"manhwa", "manhua"}, []string{"seed1"}, 1, 1)
	require.NoError(t, err)
	assert.True(t, got.HasNext)
	require.Len(t, got.Items, 1) // trimmed to perPage
	assert.Equal(t, "m1", got.Items[0].ID)

	// Sub-type membership gate on the first-class column (no join/overlap count).
	assert.Contains(t, gotSQL, "media.sub_type IN (")
	assert.NotContains(t, gotSQL, "COUNT(DISTINCT")
	// Tie-break is the popular order.
	assert.Contains(t, gotSQL, "ORDER BY popularity DESC, title ASC")
	// Excluded ids are dropped.
	assert.Contains(t, gotSQL, "media.id NOT IN (")
	// Sub-types are bound verbatim (canonical slugs); the seed id is bound verbatim.
	assertContains(t, params, "manhwa")
	assertContains(t, params, "manhua")
	assertContains(t, params, "src")
	assertContains(t, params, "seed1")
}

func TestMediaRepo_Recommend_NoExclude(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var gotSQL string
	// source(1) + sub_type(1) + limit + offset = 4, no exclude.
	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(4)...).RunAndReturn(
		func(_ context.Context, sql string, _ ...any) ([]map[string]any, error) {
			gotSQL = sql
			return []map[string]any{mediaRow("m1", "manhwa")}, nil
		})

	got, err := repo.Recommend(context.Background(), "src", []string{"manhwa"}, nil, 1, 30)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.NotContains(t, gotSQL, "NOT IN")
}

func TestMediaRepo_Recommend_PropagatesError(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	wantErr := errors.New("d1 boom")

	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(4)...).Return(nil, wantErr)

	_, err := repo.Recommend(context.Background(), "src", []string{"manhwa"}, nil, 1, 30)
	assert.ErrorIs(t, err, wantErr)
}

func TestMediaRepo_Search_BindsQueryLike(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var params []any
	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(4)...).RunAndReturn(
		func(_ context.Context, _ string, p ...any) ([]map[string]any, error) {
			params = p
			return []map[string]any{mediaRow("hit", "")}, nil
		})

	got, err := repo.Search(context.Background(), "src", "naruto", 1, 30, domain.CatalogFilter{})
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Contains(t, params, "%naruto%")
}

func TestMediaRepo_Get_FoundAndNotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Query(mock.Anything, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "WHERE id = ?1")
	}), "m1").Return([]map[string]any{mediaRow("m1", "manhwa")}, nil).Once()

	got, err := repo.Get(context.Background(), "m1")
	require.NoError(t, err)
	assert.Equal(t, "m1", got.ID)
	assert.Equal(t, "manhwa", got.SubType)

	q.EXPECT().Query(mock.Anything, mock.Anything, "missing").Return(nil, nil).Once()
	_, err = repo.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMediaRepo_Chapters(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Query(mock.Anything, mock.Anything, "m1").Return([]map[string]any{
		{"id": "c1", "media_id": "m1", "url": "u", "name": "Ch 1", "number": float64(1), "scanlator": "", "date_upload": float64(0), "format": "cbz"},
	}, nil)

	got, err := repo.Chapters(context.Background(), "m1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c1", got[0].ID)
	assert.Equal(t, "m1", got[0].MediaID)
	assert.Equal(t, "cbz", got[0].Format)
}

func TestMediaRepo_Pages(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Query(mock.Anything, mock.Anything, "c1").Return([]map[string]any{
		{"idx": float64(0), "r2_key": "pages/a.avif", "width": float64(800), "height": float64(1200)},
	}, nil)

	got, err := repo.Pages(context.Background(), "c1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "pages/a.avif", got[0].R2Key)
	assert.Equal(t, 800, got[0].Width)
	assert.Equal(t, domain.PageKindImage, got[0].Kind)
}

func TestMediaRepo_SubTypes_FromMediaColumn(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	// Distinct sub-types come off the media column, LEFT JOIN sub_type for the
	// display name, sorted.
	q.EXPECT().Query(mock.Anything, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "FROM media m LEFT JOIN sub_type st") &&
			strings.Contains(s, "m.sub_type != ''")
	}), "src").Return([]map[string]any{
		{"slug": "manhua", "name": "Manhua"},
		{"slug": "manhwa", "name": "Manhwa"},
		{"slug": "", "name": ""}, // defensively skipped
	}, nil)

	got, err := repo.SubTypes(context.Background(), "src")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "manhua", got[0].Slug)
	assert.Equal(t, "Manhua", got[0].Name) // display name resolved from the sub_type table
	assert.Equal(t, "manhwa", got[1].Slug)
}

func TestMediaRepo_SubTypeVocab(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT slug, type, name FROM sub_type")).
		Return([]map[string]any{
			{"slug": "manga", "type": "manga", "name": "Manga"},
			{"slug": "manhwa", "type": "manga", "name": "Manhwa"},
			{"slug": "web_novel", "type": "novel", "name": "Web Novel"},
		}, nil)

	got, err := repo.SubTypeVocab(context.Background())
	require.NoError(t, err)
	require.Len(t, got[domain.MediaManga], 2)
	require.Len(t, got[domain.MediaNovel], 1)
	assert.Equal(t, "manga", got[domain.MediaManga][0].Slug)
}

func TestMediaRepo_ValidSubType(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	// Empty slug never hits the DB.
	ok, err := repo.ValidSubType(context.Background(), domain.MediaManga, "")
	require.NoError(t, err)
	assert.True(t, ok)

	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT slug FROM sub_type"), "manhwa", "manga").
		Return([]map[string]any{{"slug": "manhwa"}}, nil).Once()
	ok, err = repo.ValidSubType(context.Background(), domain.MediaManga, "manhwa")
	require.NoError(t, err)
	assert.True(t, ok)

	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT slug FROM sub_type"), "web_novel", "manga").
		Return(nil, nil).Once()
	ok, err = repo.ValidSubType(context.Background(), domain.MediaManga, "web_novel")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestMediaRepo_SubTypeCRUD(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO sub_type"), "webtoon", "manga", "Webtoon").Return(nil).Once()
	require.NoError(t, repo.CreateSubType(context.Background(),
		domain.SubType{Slug: "webtoon", Type: domain.MediaManga, Name: "Webtoon"}))

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE sub_type SET"), "webtoon", "manga", "Web Toon").Return(nil).Once()
	require.NoError(t, repo.UpdateSubType(context.Background(), "webtoon",
		domain.SubType{Slug: "webtoon", Type: domain.MediaManga, Name: "Web Toon"}))

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM sub_type"), "webtoon").Return(nil).Once()
	require.NoError(t, repo.DeleteSubType(context.Background(), "webtoon"))
}

func TestMediaRepo_CreateMedia_InsertsAndSyncs(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	// INSERT the media row (id, source_id, type, sub_type, url, title, cover_url,
	// description, status, updated_at = 10 bound params).
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO media"), anyN(10)...).Return(nil).Once()
	// Each taxonomy sync starts by clearing the join rows (genre re-introduced).
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM media_genre"), "m1").Return(nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM media_author"), "m1").Return(nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM media_artist"), "m1").Return(nil).Once()
	// One genre "Action": upsert (INSERT OR IGNORE into genre), then look up its id, then link.
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT OR IGNORE INTO genre"), anyN(3)...).Return(nil).Once()
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT id FROM genre"), "action").
		Return([]map[string]any{{"id": "g1"}}, nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT OR IGNORE INTO media_genre"), "m1", "g1").Return(nil).Once()

	m := domain.Media{ID: "m1", SourceID: "src", Type: domain.MediaManga, SubType: "manhwa", URL: "u", Title: "T", Genres: []string{"Action"}}
	require.NoError(t, repo.CreateMedia(context.Background(), m))
}

func TestMediaRepo_DeleteMedia_CascadesExplicitly(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	for _, prefix := range []string{
		"DELETE FROM page", "DELETE FROM chapter",
		"DELETE FROM media_genre", "DELETE FROM media_author", "DELETE FROM media_artist",
		"DELETE FROM media WHERE",
	} {
		q.EXPECT().Exec(mock.Anything, sqlHasPrefix(prefix), "m1").Return(nil).Once()
	}

	require.NoError(t, repo.DeleteMedia(context.Background(), "m1"))
}

func TestMediaRepo_CreateChapter(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var params []any
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO chapter"), anyN(8)...).
		RunAndReturn(func(_ context.Context, _ string, p ...any) error {
			params = p
			return nil
		})

	err := repo.CreateChapter(context.Background(), domain.Chapter{ID: "c1", MediaID: "m1", Name: "Ch 1"})
	require.NoError(t, err)
	assert.Equal(t, "c1", params[0])
	assert.Equal(t, "m1", params[1])
}

func TestMediaRepo_UpdateMedia(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	// UPDATE binds id, type, sub_type, url, title, cover_url, description, status,
	// updated_at = 9 params.
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE media SET"), anyN(9)...).Return(nil).Once()
	// No taxonomies → each sync just clears its join rows (genre re-introduced).
	for _, p := range []string{"DELETE FROM media_genre", "DELETE FROM media_author", "DELETE FROM media_artist"} {
		q.EXPECT().Exec(mock.Anything, sqlHasPrefix(p), "m1").Return(nil).Once()
	}

	err := repo.UpdateMedia(context.Background(), domain.Media{ID: "m1", Title: "T2"})
	require.NoError(t, err)
}

func TestMediaRepo_UpdateChapter(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var params []any
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE chapter SET"), anyN(7)...).
		RunAndReturn(func(_ context.Context, _ string, p ...any) error {
			params = p
			return nil
		})

	err := repo.UpdateChapter(context.Background(), domain.Chapter{ID: "c1", Name: "Ch 2"})
	require.NoError(t, err)
	assert.Equal(t, "c1", params[0])
}

func TestMediaRepo_CreateMedia_InsertError(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	wantErr := errors.New("insert boom")

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO media"), anyN(10)...).Return(wantErr).Once()

	err := repo.CreateMedia(context.Background(), domain.Media{ID: "m1", Title: "T"})
	assert.ErrorIs(t, err, wantErr)
}

func TestMediaRepo_DeleteChapter(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM page"), "c1").Return(nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM chapter"), "c1").Return(nil).Once()

	require.NoError(t, repo.DeleteChapter(context.Background(), "c1"))
}

func TestMediaRepo_ListTaxonomy(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT id, name FROM author")).
		Return([]map[string]any{{"id": "a1", "name": "Oda"}}, nil)

	got, err := repo.ListTaxonomy(context.Background(), domain.TaxonomyAuthor)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Oda", got[0].Name)
	assert.Empty(t, got[0].Slug) // author has no slug
}

func TestMediaRepo_CreateTaxonomy_Genre(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT OR IGNORE INTO genre"), anyN(3)...).Return(nil).Once()
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT id FROM genre"), "sci-fi").
		Return([]map[string]any{{"id": "g9"}}, nil).Once()

	got, err := repo.CreateTaxonomy(context.Background(), domain.TaxonomyGenre, "Sci Fi")
	require.NoError(t, err)
	assert.Equal(t, "g9", got.ID)
	assert.Equal(t, "sci-fi", got.Slug)
}

func TestMediaRepo_UpdateTaxonomy_ReslugsGenre(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var params []any
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE genre"), anyN(3)...).
		RunAndReturn(func(_ context.Context, _ string, p ...any) error {
			params = p
			return nil
		})

	got, err := repo.UpdateTaxonomy(context.Background(), domain.TaxonomyGenre, "g1", "Slice Of Life")
	require.NoError(t, err)
	assert.Equal(t, "slice-of-life", got.Slug)
	assert.Equal(t, "slice-of-life", params[2]) // re-slugged
}

func TestMediaRepo_DeleteTaxonomy(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM media_genre"), "g1").Return(nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM genre"), "g1").Return(nil).Once()

	require.NoError(t, repo.DeleteTaxonomy(context.Background(), domain.TaxonomyGenre, "g1"))
}
