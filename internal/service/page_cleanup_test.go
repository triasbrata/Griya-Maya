package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

// newMediaSvcCleanup wires a MediaService exposing the cleanup queue mock so
// tests can assert the R2 keys scheduled for deletion.
func newMediaSvcCleanup(t *testing.T) (*service.MediaService, *mocks.MockMediaRepository, *mocks.MockObjectStore, *mocks.MockCleanupQueue) {
	t.Helper()
	repo := mocks.NewMockMediaRepository(t)
	store := mocks.NewMockObjectStore(t)
	cover := mocks.NewMockCoverMirrorQueue(t)
	clq := mocks.NewMockCleanupQueue(t)
	return service.NewMediaService(repo, store, cover, clq, "", testPresignTTL), repo, store, clq
}

func TestMediaService_DeleteChapters_CollectsKeysAndEnqueuesOnce(t *testing.T) {
	svc, repo, _, clq := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "pages/a.avif"},
		{Index: 1, R2Key: ""}, // blank keys skipped
	}, nil)
	repo.EXPECT().DeleteChapter(ctx, "c1").Return(nil)
	repo.EXPECT().Pages(ctx, "c2").Return([]domain.StoredPage{
		{Index: 0, R2Key: "pages/b.avif"},
	}, nil)
	repo.EXPECT().DeleteChapter(ctx, "c2").Return(nil)

	// One job with all collected keys, in order.
	clq.EXPECT().Enqueue(ctx, []string{"pages/a.avif", "pages/b.avif"}).Return(nil)

	require.NoError(t, svc.DeleteChapters(ctx, []string{"c1", " ", "c2"}))
}

func TestMediaService_DeleteChapters_VideoBundleEnqueuesPrefix(t *testing.T) {
	svc, repo, _, clq := newMediaSvcCleanup(t)
	ctx := context.Background()

	// A video chapter records only its playlist key; the whole hls/{id}/ bundle
	// (init + segments) must be swept, so it's enqueued as a recursive prefix.
	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "hls/vid1/index.m3u8", Kind: domain.PageKindVideo},
	}, nil)
	repo.EXPECT().DeleteChapter(ctx, "c1").Return(nil)

	clq.EXPECT().EnqueuePrefixes(ctx, []string{"hls/vid1/"}).Return(nil)
	// No plain-key Enqueue for a video-only delete.

	require.NoError(t, svc.DeleteChapters(ctx, []string{"c1"}))
}

func TestMediaService_DeleteChapters_MixedKeysAndVideoPrefix(t *testing.T) {
	svc, repo, _, clq := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "pages/a.avif", Kind: domain.PageKindImage},
	}, nil)
	repo.EXPECT().DeleteChapter(ctx, "c1").Return(nil)
	repo.EXPECT().Pages(ctx, "c2").Return([]domain.StoredPage{
		{Index: 0, R2Key: "hls/vid2/index.m3u8", Kind: domain.PageKindVideo},
	}, nil)
	repo.EXPECT().DeleteChapter(ctx, "c2").Return(nil)

	// Plain keys and bundle prefixes are enqueued as separate jobs.
	clq.EXPECT().Enqueue(ctx, []string{"pages/a.avif"}).Return(nil)
	clq.EXPECT().EnqueuePrefixes(ctx, []string{"hls/vid2/"}).Return(nil)

	require.NoError(t, svc.DeleteChapters(ctx, []string{"c1", "c2"}))
}

func TestMediaService_DeleteChapters_NoKeysNoEnqueue(t *testing.T) {
	svc, repo, _, _ := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return(nil, nil)
	repo.EXPECT().DeleteChapter(ctx, "c1").Return(nil)
	// No Enqueue expectation → the mock fails if it is called.

	require.NoError(t, svc.DeleteChapters(ctx, []string{"c1"}))
}

func TestMediaService_DeleteChapters_PagesError(t *testing.T) {
	svc, repo, _, _ := newMediaSvcCleanup(t)
	ctx := context.Background()
	wantErr := errors.New("db down")

	repo.EXPECT().Pages(ctx, "c1").Return(nil, wantErr)

	err := svc.DeleteChapters(ctx, []string{"c1"})
	assert.ErrorIs(t, err, wantErr)
}

func TestMediaService_DeleteChapters_DeleteError(t *testing.T) {
	svc, repo, _, _ := newMediaSvcCleanup(t)
	ctx := context.Background()
	wantErr := errors.New("delete failed")

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{{R2Key: "k"}}, nil)
	repo.EXPECT().DeleteChapter(ctx, "c1").Return(wantErr)
	// Delete failed → no enqueue (never schedule a live object).

	err := svc.DeleteChapters(ctx, []string{"c1"})
	assert.ErrorIs(t, err, wantErr)
}

func TestMediaService_DeleteChapter_DelegatesAndEnqueues(t *testing.T) {
	svc, repo, _, clq := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{{Index: 0, R2Key: "pages/a.avif"}}, nil)
	repo.EXPECT().DeleteChapter(ctx, "c1").Return(nil)
	clq.EXPECT().Enqueue(ctx, []string{"pages/a.avif"}).Return(nil)

	require.NoError(t, svc.DeleteChapter(ctx, "c1"))
}

func TestMediaService_DeleteMedia_EnqueuesPagesAndMirroredCover(t *testing.T) {
	svc, repo, _, clq := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().PageKeysForMedia(ctx, "m1").Return([]string{"pages/a.avif", "pages/b.avif"}, nil)
	// Mirrored cover (an R2 key, no scheme) is added to the cleanup set.
	repo.EXPECT().Get(ctx, "m1").Return(domain.Media{ID: "m1", CoverURL: "covers/m1.avif"}, nil)
	repo.EXPECT().DeleteMedia(ctx, "m1").Return(nil)
	clq.EXPECT().Enqueue(ctx, []string{"pages/a.avif", "pages/b.avif", "covers/m1.avif"}).Return(nil)

	require.NoError(t, svc.DeleteMedia(ctx, "m1"))
}

func TestMediaService_DeleteMedia_ExternalCoverNotEnqueued(t *testing.T) {
	svc, repo, _, clq := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().PageKeysForMedia(ctx, "m1").Return([]string{"pages/a.avif"}, nil)
	// External cover (has a scheme) carries nothing of ours — only pages cleaned.
	repo.EXPECT().Get(ctx, "m1").Return(domain.Media{ID: "m1", CoverURL: "https://ext/c.jpg"}, nil)
	repo.EXPECT().DeleteMedia(ctx, "m1").Return(nil)
	clq.EXPECT().Enqueue(ctx, []string{"pages/a.avif"}).Return(nil)

	require.NoError(t, svc.DeleteMedia(ctx, "m1"))
}

func TestMediaService_DeleteMedia_PageKeysError(t *testing.T) {
	svc, repo, _, _ := newMediaSvcCleanup(t)
	ctx := context.Background()
	wantErr := errors.New("db down")

	repo.EXPECT().PageKeysForMedia(ctx, "m1").Return(nil, wantErr)

	assert.ErrorIs(t, svc.DeleteMedia(ctx, "m1"), wantErr)
}

func TestMediaService_ChapterPagesAdmin_ExposesRawKeyAndPresignedURL(t *testing.T) {
	svc, repo, store, _ := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "pages/a.avif", Width: 800, Height: 1200, Kind: domain.PageKindImage},
	}, nil)
	store.EXPECT().PublicURL("pages/a.avif").Return("")
	store.EXPECT().PresignGet(ctx, "pages/a.avif", testPresignTTL).Return("https://r2/presigned", nil)

	pages, err := svc.ChapterPagesAdmin(ctx, "c1")
	require.NoError(t, err)
	require.Len(t, pages, 1)
	assert.Equal(t, "pages/a.avif", pages[0].R2Key)
	assert.Equal(t, "https://r2/presigned", pages[0].ImageURL)
	assert.Equal(t, 800, pages[0].Width)
	assert.Equal(t, domain.PageKindImage, pages[0].Kind)
}

func TestMediaService_ChapterPagesAdmin_PagesError(t *testing.T) {
	svc, repo, _, _ := newMediaSvcCleanup(t)
	ctx := context.Background()
	wantErr := errors.New("db down")

	repo.EXPECT().Pages(ctx, "c1").Return(nil, wantErr)

	_, err := svc.ChapterPagesAdmin(ctx, "c1")
	assert.ErrorIs(t, err, wantErr)
}

func TestMediaService_DeleteChapterPage_DeletesAndEnqueues(t *testing.T) {
	svc, repo, _, clq := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{
		{Index: 0, R2Key: "pages/a.avif"},
		{Index: 1, R2Key: "pages/b.avif"},
	}, nil)
	repo.EXPECT().DeletePage(ctx, "c1", 1).Return(nil)
	clq.EXPECT().Enqueue(ctx, []string{"pages/b.avif"}).Return(nil)

	require.NoError(t, svc.DeleteChapterPage(ctx, "c1", 1))
}

func TestMediaService_DeleteChapterPage_NotFound(t *testing.T) {
	svc, repo, _, _ := newMediaSvcCleanup(t)
	ctx := context.Background()

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{{Index: 0, R2Key: "pages/a.avif"}}, nil)

	err := svc.DeleteChapterPage(ctx, "c1", 5)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMediaService_DeleteChapterPage_Validation(t *testing.T) {
	svc, _, _, _ := newMediaSvcCleanup(t)
	err := svc.DeleteChapterPage(context.Background(), "  ", 0)
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestMediaService_DeleteChapterPage_DeleteError(t *testing.T) {
	svc, repo, _, _ := newMediaSvcCleanup(t)
	ctx := context.Background()
	wantErr := errors.New("delete failed")

	repo.EXPECT().Pages(ctx, "c1").Return([]domain.StoredPage{{Index: 0, R2Key: "pages/a.avif"}}, nil)
	repo.EXPECT().DeletePage(ctx, "c1", 0).Return(wantErr)

	assert.ErrorIs(t, svc.DeleteChapterPage(ctx, "c1", 0), wantErr)
}
