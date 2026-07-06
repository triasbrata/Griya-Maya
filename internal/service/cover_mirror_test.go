package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

func newMediaSvcWithQueue(t *testing.T) (*service.MediaService, *mocks.MockMediaRepository, *mocks.MockObjectStore, *mocks.MockCoverMirrorQueue) {
	t.Helper()
	repo := mocks.NewMockMediaRepository(t)
	store := mocks.NewMockObjectStore(t)
	cq := mocks.NewMockCoverMirrorQueue(t)
	clq := mocks.NewMockCleanupQueue(t)
	return service.NewMediaService(repo, store, cq, clq, "", testPresignTTL), repo, store, cq
}

func TestMediaService_CreateMedia_EnqueuesExternalCover(t *testing.T) {
	svc, repo, _, cq := newMediaSvcWithQueue(t)
	ctx := context.Background()

	repo.EXPECT().CreateMedia(ctx, mock.Anything).Return(nil)
	cq.EXPECT().Enqueue(ctx, mock.MatchedBy(func(j domain.CoverMirrorJob) bool {
		return j.SourceURL == "https://ext/cover.jpg" && j.MediaID != ""
	})).Return(nil)
	repo.EXPECT().Get(ctx, mock.Anything).
		Return(domain.Media{ID: "x", CoverURL: "https://ext/cover.jpg"}, nil)

	got, err := svc.CreateMedia(ctx, domain.MediaWriteRequest{
		SourceID: "s", Title: "T", CoverURL: "https://ext/cover.jpg",
	})
	require.NoError(t, err)
	// External URL is returned as-is until the async mirror completes.
	assert.Equal(t, "https://ext/cover.jpg", got.CoverURL)
}

func TestMediaService_CreateMedia_NoCoverNoEnqueue(t *testing.T) {
	svc, repo, _, _ := newMediaSvcWithQueue(t)
	ctx := context.Background()
	// No Enqueue expectation set → the mock fails if it is called.
	repo.EXPECT().CreateMedia(ctx, mock.Anything).Return(nil)
	repo.EXPECT().Get(ctx, mock.Anything).Return(domain.Media{ID: "x"}, nil)

	_, err := svc.CreateMedia(ctx, domain.MediaWriteRequest{SourceID: "s", Title: "T"})
	require.NoError(t, err)
}

func TestMediaService_Details_PresignsMirroredCover(t *testing.T) {
	svc, repo, store, _ := newMediaSvcWithQueue(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, "m1").Return(domain.Media{ID: "m1", CoverURL: "covers/m1.avif"}, nil)
	store.EXPECT().PublicURL("covers/m1.avif").Return("")
	store.EXPECT().PresignGet(ctx, "covers/m1.avif", mock.Anything).Return("https://r2/presigned", nil)

	got, err := svc.Details(ctx, "m1")
	require.NoError(t, err)
	assert.Equal(t, "https://r2/presigned", got.CoverURL)
}

func TestMediaService_Details_LeavesExternalCover(t *testing.T) {
	svc, repo, _, _ := newMediaSvcWithQueue(t)
	ctx := context.Background()
	// External cover (has a scheme) is returned untouched — no presign call.
	repo.EXPECT().Get(ctx, "m1").Return(domain.Media{ID: "m1", CoverURL: "https://ext/c.jpg"}, nil)

	got, err := svc.Details(ctx, "m1")
	require.NoError(t, err)
	assert.Equal(t, "https://ext/c.jpg", got.CoverURL)
}
