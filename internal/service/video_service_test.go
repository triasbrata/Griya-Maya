package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

func newVideoSvc(t *testing.T, baseURL string) (*service.VideoService, *mocks.MockJobRepository, *mocks.MockObjectStore) {
	t.Helper()
	pages := mocks.NewMockJobRepository(t)
	store := mocks.NewMockObjectStore(t)
	return service.NewVideoService(pages, store, baseURL), pages, store
}

func TestVideoService_Register_Validation(t *testing.T) {
	cases := []struct {
		name string
		req  domain.VideoRegisterRequest
	}{
		{"missing chapter", domain.VideoRegisterRequest{PlaylistKey: "hls/a/index.m3u8"}},
		{"missing playlist", domain.VideoRegisterRequest{ChapterID: "ch1"}},
		{"not an m3u8", domain.VideoRegisterRequest{ChapterID: "ch1", PlaylistKey: "hls/a/seg0.ts"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := newVideoSvc(t, "")
			_, err := svc.Register(context.Background(), tc.req)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
}

func TestVideoService_Register_PersistsSingleVideoPage(t *testing.T) {
	svc, pages, store := newVideoSvc(t, "https://api.test")
	ctx := context.Background()
	req := domain.VideoRegisterRequest{
		ChapterID:   "ch1",
		PlaylistKey: "/hls/a/index.m3u8", // leading slash is trimmed
		Width:       1920,
		Height:      1080,
	}

	pages.EXPECT().ReplacePages(ctx, "ch1", []domain.StoredPage{
		{Index: 0, R2Key: "hls/a/index.m3u8", Width: 1920, Height: 1080, Kind: domain.PageKindVideo},
	}).Return(nil)
	// No public R2 domain -> path-based stream proxy (not the ?key= image proxy).
	store.EXPECT().PublicURL("hls/a/index.m3u8").Return("")

	page, err := svc.Register(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, domain.PageKindVideo, page.Type)
	assert.Equal(t, 1920, page.Width)
	assert.NotContains(t, page.ImageURL, "?key=")
	assert.Contains(t, page.ImageURL, "hls/a/index.m3u8")
}

func TestVideoService_Register_PersistError(t *testing.T) {
	svc, pages, _ := newVideoSvc(t, "")
	ctx := context.Background()
	wantErr := errors.New("d1 down")

	pages.EXPECT().ReplacePages(mock.Anything, "ch1", mock.Anything).Return(wantErr)

	_, err := svc.Register(ctx, domain.VideoRegisterRequest{ChapterID: "ch1", PlaylistKey: "hls/a/index.m3u8"})
	assert.ErrorIs(t, err, wantErr)
}
