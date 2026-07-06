package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

// testPresignTTL is the fixed page-URL lifetime used across the service tests.
const testPresignTTL = time.Hour

func newMediaSvc(t *testing.T, baseURL string) (*service.MediaService, *mocks.MockMediaRepository, *mocks.MockObjectStore) {
	t.Helper()
	repo := mocks.NewMockMediaRepository(t)
	store := mocks.NewMockObjectStore(t)
	cq := mocks.NewMockCoverMirrorQueue(t)
	clq := mocks.NewMockCleanupQueue(t)
	return service.NewMediaService(repo, store, cq, clq, baseURL, testPresignTTL), repo, store
}

func TestMediaService_Popular_DelegatesToList(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "https://api.test/")
	ctx := context.Background()
	filter := domain.CatalogFilter{Sort: "title"}
	want := domain.MediaPage{Items: []domain.Media{{ID: "m1"}}, Page: 2}

	repo.EXPECT().List(ctx, "src", "popular", 2, 30, filter).Return(want, nil)

	got, err := svc.Popular(ctx, "src", 2, filter)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMediaService_Latest_DelegatesToList(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	want := domain.MediaPage{Page: 1}

	repo.EXPECT().List(ctx, "src", "latest", 1, 30, domain.CatalogFilter{}).Return(want, nil)

	got, err := svc.Latest(ctx, "src", 1, domain.CatalogFilter{})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMediaService_Search_DelegatesToSearch(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	want := domain.MediaPage{Items: []domain.Media{{ID: "hit"}}}

	repo.EXPECT().Search(ctx, "src", "naruto", 3, 30, domain.CatalogFilter{}).Return(want, nil)

	got, err := svc.Search(ctx, "src", "naruto", 3, domain.CatalogFilter{})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMediaService_Recommendations_DelegatesToRecommend(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	genres := []string{"Action", "Comedy"}
	exclude := []string{"seed1"}
	want := domain.MediaPage{Items: []domain.Media{{ID: "m1"}}, Page: 2}

	repo.EXPECT().Recommend(ctx, "src", genres, exclude, 2, 30).Return(want, nil)

	got, err := svc.Recommendations(ctx, "src", genres, exclude, 2)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMediaService_Recommendations_FallsBackToPopularWhenNoGenres(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	want := domain.MediaPage{Items: []domain.Media{{ID: "pop"}}, Page: 1}

	// No genres → the source's popular feed, so the endpoint always returns something.
	repo.EXPECT().List(ctx, "src", "popular", 1, 30, domain.CatalogFilter{}).Return(want, nil)

	got, err := svc.Recommendations(ctx, "src", nil, []string{"seed1"}, 1)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMediaService_GenresAndCategories(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	repo.EXPECT().Genres(ctx, "src").Return([]domain.Taxonomy{{Slug: "action", Name: "Action"}}, nil)
	repo.EXPECT().Categories(ctx, "src").Return([]domain.Taxonomy{{Slug: "webtoon", Name: "Webtoon"}}, nil)

	g, err := svc.Genres(ctx, "src")
	require.NoError(t, err)
	assert.Equal(t, "action", g[0].Slug)

	c, err := svc.Categories(ctx, "src")
	require.NoError(t, err)
	assert.Equal(t, "webtoon", c[0].Slug)
}

func TestMediaService_Details(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	want := domain.Media{ID: "m1", Title: "T", Type: domain.MediaManga}

	repo.EXPECT().Get(ctx, "m1").Return(want, nil)

	got, err := svc.Details(ctx, "m1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMediaService_Chapters(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	want := []domain.Chapter{{ID: "c1", MediaID: "m1"}}

	repo.EXPECT().Chapters(ctx, "m1").Return(want, nil)

	got, err := svc.Chapters(ctx, "m1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMediaService_Pages_PublicURL(t *testing.T) {
	svc, repo, store := newMediaSvc(t, "https://api.test")
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "pages/a.avif", Width: 800, Height: 1200},
	}, nil)
	store.EXPECT().PublicURL("pages/a.avif").Return("https://cdn.test/pages/a.avif")

	pages, err := svc.Pages(ctx, "c1")
	require.NoError(t, err)
	require.Len(t, pages, 1)
	assert.Equal(t, "https://cdn.test/pages/a.avif", pages[0].ImageURL)
}

func TestMediaService_Pages_PresignFallback(t *testing.T) {
	svc, repo, store := newMediaSvc(t, "https://api.test/")
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "pages/a.avif"},
	}, nil)
	store.EXPECT().PublicURL("pages/a.avif").Return("")
	store.EXPECT().PresignGet(ctx, "pages/a.avif", testPresignTTL).
		Return("https://acc.r2.cloudflarestorage.com/manga/pages/a.avif?X-Amz-Signature=abc", nil)

	pages, err := svc.Pages(ctx, "c1")
	require.NoError(t, err)
	require.Len(t, pages, 1)
	assert.Equal(t, "https://acc.r2.cloudflarestorage.com/manga/pages/a.avif?X-Amz-Signature=abc", pages[0].ImageURL)
}

func TestMediaService_Pages_VideoAndNovel(t *testing.T) {
	svc, repo, store := newMediaSvc(t, "https://api.test")
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "hls/master.m3u8", Kind: domain.PageKindVideo},
		{Index: 1, R2Key: "novels/x.txt", Kind: domain.PageKindNovel},
	}, nil)
	store.EXPECT().PublicURL("hls/master.m3u8").Return("")
	store.EXPECT().Get(ctx, "novels/x.txt").Return([]byte("once upon a time"), "", nil)

	pages, err := svc.Pages(ctx, "c1")
	require.NoError(t, err)
	require.Len(t, pages, 2)
	assert.Equal(t, domain.PageKindVideo, pages[0].Type)
	assert.NotContains(t, pages[0].ImageURL, "?key=")
	assert.Equal(t, domain.PageKindNovel, pages[1].Type)
	assert.Equal(t, "once upon a time", pages[1].Body)
}

func TestMediaService_Pages_PropagatesError(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	wantErr := errors.New("db down")

	repo.EXPECT().Pages(ctx, "c1").Return(nil, wantErr)

	_, err := svc.Pages(ctx, "c1")
	assert.ErrorIs(t, err, wantErr)
}

func TestMediaService_CreateMedia_ValidatesAndReturnsStored(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	// Service generates the id, so match on any Media then echo a stored row.
	repo.EXPECT().CreateMedia(ctx, mock.MatchedBy(func(m domain.Media) bool {
		return m.ID != "" && m.SourceID == "src" && m.Type == domain.MediaVideo && m.Title == "T"
	})).Return(nil)
	repo.EXPECT().Get(ctx, mock.Anything).Return(domain.Media{ID: "generated", Title: "T"}, nil)

	got, err := svc.CreateMedia(ctx, domain.MediaWriteRequest{
		SourceID: "src", Type: domain.MediaVideo, URL: "u", Title: "T",
	})
	require.NoError(t, err)
	assert.Equal(t, "generated", got.ID)
}

func TestMediaService_CreateMedia_DefaultsTypeToManga(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	repo.EXPECT().CreateMedia(ctx, mock.MatchedBy(func(m domain.Media) bool {
		return m.Type == domain.MediaManga
	})).Return(nil)
	repo.EXPECT().Get(ctx, mock.Anything).Return(domain.Media{}, nil)

	_, err := svc.CreateMedia(ctx, domain.MediaWriteRequest{SourceID: "src", URL: "u", Title: "T"})
	require.NoError(t, err)
}

func TestMediaService_CreateMedia_DefaultsURLToID(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	// URL omitted → the service fills it with the generated id.
	repo.EXPECT().CreateMedia(ctx, mock.MatchedBy(func(m domain.Media) bool {
		return m.URL != "" && m.URL == m.ID
	})).Return(nil)
	repo.EXPECT().Get(ctx, mock.Anything).Return(domain.Media{}, nil)

	_, err := svc.CreateMedia(ctx, domain.MediaWriteRequest{SourceID: "src", Title: "T"})
	require.NoError(t, err)
}

func TestMediaService_CreateMedia_Validation(t *testing.T) {
	svc, _, _ := newMediaSvc(t, "")
	ctx := context.Background()

	cases := []domain.MediaWriteRequest{
		{URL: "u", Title: "T"},      // missing sourceId
		{SourceID: "src", URL: "u"}, // missing title
		{SourceID: "src", URL: "u", Title: "T", Type: domain.MediaType("x")}, // bad type
	}
	for _, req := range cases {
		_, err := svc.CreateMedia(ctx, req)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	}
}

func TestMediaService_UpdateMedia(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	repo.EXPECT().UpdateMedia(ctx, mock.MatchedBy(func(m domain.Media) bool {
		return m.ID == "m1" && m.Title == "T2"
	})).Return(nil)
	repo.EXPECT().Get(ctx, "m1").Return(domain.Media{ID: "m1", Title: "T2"}, nil)

	got, err := svc.UpdateMedia(ctx, "m1", domain.MediaWriteRequest{SourceID: "src", URL: "u", Title: "T2"})
	require.NoError(t, err)
	assert.Equal(t, "T2", got.Title)
}

func TestMediaService_DeleteMedia(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	// No page keys and a non-mirrored cover → nothing enqueued for cleanup.
	repo.EXPECT().PageKeysForMedia(ctx, "m1").Return(nil, nil)
	repo.EXPECT().Get(ctx, "m1").Return(domain.Media{ID: "m1"}, nil)
	repo.EXPECT().DeleteMedia(ctx, "m1").Return(nil)
	require.NoError(t, svc.DeleteMedia(ctx, "m1"))

	_, e := svc.CreateChapter(ctx, domain.ChapterWriteRequest{}) // missing mediaId
	assert.ErrorIs(t, e, domain.ErrInvalidInput)
}

func TestMediaService_CreateChapter(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	repo.EXPECT().CreateChapter(ctx, mock.MatchedBy(func(c domain.Chapter) bool {
		return c.ID != "" && c.MediaID == "m1" && c.Name == "Ch 1"
	})).Return(nil)

	got, err := svc.CreateChapter(ctx, domain.ChapterWriteRequest{MediaID: "m1", Name: "Ch 1"})
	require.NoError(t, err)
	assert.NotEmpty(t, got.ID)
}

func TestMediaService_UpdateChapter(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	repo.EXPECT().UpdateChapter(ctx, mock.MatchedBy(func(c domain.Chapter) bool {
		return c.ID == "c1" && c.MediaID == "m1" && c.Name == "Ch 2"
	})).Return(nil)

	got, err := svc.UpdateChapter(ctx, "c1", domain.ChapterWriteRequest{MediaID: "m1", Name: "Ch 2"})
	require.NoError(t, err)
	assert.Equal(t, "c1", got.ID)

	// Validation: missing name.
	_, err = svc.UpdateChapter(ctx, "c1", domain.ChapterWriteRequest{MediaID: "m1"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
	// Validation: missing id.
	_, err = svc.UpdateChapter(ctx, "", domain.ChapterWriteRequest{MediaID: "m1", Name: "x"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestMediaService_DeleteChapter(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()

	// DeleteChapter delegates to DeleteChapters: read pages, delete rows. No
	// page keys here → no cleanup enqueue.
	repo.EXPECT().Pages(ctx, "c1").Return(nil, nil)
	repo.EXPECT().DeleteChapter(ctx, "c1").Return(nil)
	require.NoError(t, svc.DeleteChapter(ctx, "c1"))

	err := svc.DeleteChapter(ctx, "")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestMediaService_Pages_NovelBodyError(t *testing.T) {
	svc, repo, store := newMediaSvc(t, "")
	ctx := context.Background()
	wantErr := errors.New("r2 down")

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "novels/x.txt", Kind: domain.PageKindNovel},
	}, nil)
	store.EXPECT().Get(ctx, "novels/x.txt").Return(nil, "", wantErr)

	_, err := svc.Pages(ctx, "c1")
	assert.ErrorIs(t, err, wantErr)
}

// --- taxonomy service ---

func TestTaxonomyService_CRUD(t *testing.T) {
	repo := mocks.NewMockMediaRepository(t)
	svc := service.NewTaxonomyService(repo)
	ctx := context.Background()

	repo.EXPECT().ListTaxonomy(ctx, domain.TaxonomyGenre).
		Return([]domain.Taxonomy{{Name: "Action"}}, nil)
	list, err := svc.List(ctx, domain.TaxonomyGenre)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	repo.EXPECT().CreateTaxonomy(ctx, domain.TaxonomyAuthor, "Oda").
		Return(domain.Taxonomy{ID: "a1", Name: "Oda"}, nil)
	created, err := svc.Create(ctx, domain.TaxonomyAuthor, "  Oda  ")
	require.NoError(t, err)
	assert.Equal(t, "a1", created.ID)

	repo.EXPECT().UpdateTaxonomy(ctx, domain.TaxonomyGenre, "g1", "Comedy").
		Return(domain.Taxonomy{ID: "g1", Slug: "comedy", Name: "Comedy"}, nil)
	_, err = svc.Update(ctx, domain.TaxonomyGenre, "g1", "Comedy")
	require.NoError(t, err)

	repo.EXPECT().DeleteTaxonomy(ctx, domain.TaxonomyArtist, "x1").Return(nil)
	require.NoError(t, svc.Delete(ctx, domain.TaxonomyArtist, "x1"))
}

func TestTaxonomyService_Validation(t *testing.T) {
	repo := mocks.NewMockMediaRepository(t)
	svc := service.NewTaxonomyService(repo)
	ctx := context.Background()

	_, err := svc.List(ctx, domain.TaxonomyKind("bogus"))
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = svc.Create(ctx, domain.TaxonomyGenre, "   ")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = svc.Update(ctx, domain.TaxonomyGenre, "", "x")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	err = svc.Delete(ctx, domain.TaxonomyGenre, "")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}
