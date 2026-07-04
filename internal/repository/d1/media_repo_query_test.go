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

// mediaRow builds a flat media result row with the group_concat taxonomy columns
// packed using the concatSep (0x1F) delimiter the repo reassembles.
func mediaRow(id, genres string) map[string]any {
	return map[string]any{
		"id": id, "source_id": "src", "type": "manga", "url": "u/" + id, "title": "T-" + id,
		"cover_url": "", "description": "", "status": "ongoing", "updated_at": float64(0),
		"genres": genres, "categories": "", "authors": "", "artists": "",
	}
}

func TestMediaRepo_List_MapsRowsAndPaginates(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var gotSQL string
	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(3)...).RunAndReturn(
		func(_ context.Context, sql string, _ ...any) ([]map[string]any, error) {
			gotSQL = sql
			return []map[string]any{mediaRow("m1", "Action"), mediaRow("m2", "Comedy")}, nil
		})

	got, err := repo.List(context.Background(), "src", "popular", 1, 1, domain.CatalogFilter{})
	require.NoError(t, err)
	assert.True(t, got.HasNext)
	require.Len(t, got.Items, 1) // trimmed to perPage
	assert.Equal(t, "m1", got.Items[0].ID)
	assert.Equal(t, domain.MediaManga, got.Items[0].Type)
	assert.Equal(t, []string{"Action"}, got.Items[0].Genres)
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

func TestMediaRepo_Recommend_RanksByOverlap(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var gotSQL string
	var params []any
	// genres(2 overlap) + source(1) + include-genres(2) + exclude(1) + limit + offset = 8.
	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(8)...).RunAndReturn(
		func(_ context.Context, sql string, p ...any) ([]map[string]any, error) {
			gotSQL, params = sql, p
			return []map[string]any{mediaRow("m1", "Action"), mediaRow("m2", "Comedy")}, nil
		})

	got, err := repo.Recommend(context.Background(), "src",
		[]string{"Action", "Sci Fi"}, []string{"seed1"}, 1, 1)
	require.NoError(t, err)
	assert.True(t, got.HasNext)
	require.Len(t, got.Items, 1) // trimmed to perPage
	assert.Equal(t, "m1", got.Items[0].ID)

	// Ranking is overlap-count first, then the popular tie-break.
	assert.Contains(t, gotSQL, "COUNT(DISTINCT t.slug)")
	assert.Contains(t, gotSQL, "AS overlap")
	assert.Contains(t, gotSQL, "ORDER BY overlap DESC, popularity DESC, title ASC")
	// Zero-overlap gate reuses the OR-mode EXISTS clause on the genre join tables.
	assert.Contains(t, gotSQL, "EXISTS (SELECT 1 FROM media_genre")
	// Excluded ids are dropped.
	assert.Contains(t, gotSQL, "media.id NOT IN (")
	// Genre names are bound as normalized slugs; the seed id is bound verbatim.
	assertContains(t, params, "action")
	assertContains(t, params, "sci-fi")
	assertContains(t, params, "src")
	assertContains(t, params, "seed1")
}

func TestMediaRepo_Recommend_NoExclude(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	var gotSQL string
	// genre(1 overlap) + source(1) + include-genre(1) + limit + offset = 5, no exclude.
	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(5)...).RunAndReturn(
		func(_ context.Context, sql string, _ ...any) ([]map[string]any, error) {
			gotSQL = sql
			return []map[string]any{mediaRow("m1", "Action")}, nil
		})

	got, err := repo.Recommend(context.Background(), "src", []string{"Action"}, nil, 1, 30)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.NotContains(t, gotSQL, "NOT IN")
}

func TestMediaRepo_Recommend_PropagatesError(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	wantErr := errors.New("d1 boom")

	q.EXPECT().Query(mock.Anything, mock.Anything, anyN(5)...).Return(nil, wantErr)

	_, err := repo.Recommend(context.Background(), "src", []string{"Action"}, nil, 1, 30)
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
	}), "m1").Return([]map[string]any{mediaRow("m1", "Action"+concatSep+"Drama")}, nil).Once()

	got, err := repo.Get(context.Background(), "m1")
	require.NoError(t, err)
	assert.Equal(t, "m1", got.ID)
	assert.Equal(t, []string{"Action", "Drama"}, got.Genres)

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

func TestMediaRepo_Genres_FromNormalizedTables(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	// The normalized query already dedups + sorts; the repo just maps rows.
	q.EXPECT().Query(mock.Anything, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "FROM genre t") && strings.Contains(s, "media_genre")
	}), "src").Return([]map[string]any{
		{"id": "g1", "slug": "action", "name": "Action"},
		{"id": "g2", "slug": "comedy", "name": "Comedy"},
	}, nil)

	got, err := repo.Genres(context.Background(), "src")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "action", got[0].Slug)
	assert.Equal(t, "Comedy", got[1].Name)
}

func TestMediaRepo_Categories_FromNormalizedTables(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Query(mock.Anything, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "FROM category t")
	}), "src").Return([]map[string]any{{"id": "c1", "slug": "webtoon", "name": "Webtoon"}}, nil)

	got, err := repo.Categories(context.Background(), "src")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "webtoon", got[0].Slug)
}

func TestMediaRepo_CreateMedia_InsertsAndSyncs(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	// INSERT the media row.
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO media"), anyN(9)...).Return(nil).Once()
	// Each taxonomy sync starts by clearing the join rows.
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM media_genre"), "m1").Return(nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM media_category"), "m1").Return(nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM media_author"), "m1").Return(nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM media_artist"), "m1").Return(nil).Once()
	// One genre "Action": upsert (INSERT OR IGNORE into genre), then look up its id, then link.
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT OR IGNORE INTO genre"), anyN(3)...).Return(nil).Once()
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT id FROM genre"), "action").
		Return([]map[string]any{{"id": "g1"}}, nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT OR IGNORE INTO media_genre"), "m1", "g1").Return(nil).Once()

	m := domain.Media{ID: "m1", SourceID: "src", Type: domain.MediaVideo, URL: "u", Title: "T", Genres: []string{"Action"}}
	require.NoError(t, repo.CreateMedia(context.Background(), m))
}

func TestMediaRepo_DeleteMedia_CascadesExplicitly(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	for _, prefix := range []string{
		"DELETE FROM page", "DELETE FROM chapter", "DELETE FROM media_genre",
		"DELETE FROM media_category", "DELETE FROM media_author", "DELETE FROM media_artist",
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

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE media SET"), anyN(8)...).Return(nil).Once()
	// No taxonomies → each sync just clears its join rows.
	for _, p := range []string{"DELETE FROM media_genre", "DELETE FROM media_category", "DELETE FROM media_author", "DELETE FROM media_artist"} {
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

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO media"), anyN(9)...).Return(wantErr).Once()

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
