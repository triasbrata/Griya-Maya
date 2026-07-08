package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestVideoService_PresignUploads(t *testing.T) {
	t.Run("mints per-file slots under one prefix, derives content types, picks index.m3u8", func(t *testing.T) {
		svc, _, store := newVideoSvc(t, "")
		ctx := context.Background()

		// Echo the key back as the URL so we can assert per-item wiring.
		store.EXPECT().
			PresignPut(mock.Anything, mock.Anything, 30*time.Minute, mock.Anything).
			RunAndReturn(func(_ context.Context, key string, _ time.Duration, _ string) (string, error) {
				return "https://r2/" + key, nil
			}).Times(4)

		res, err := svc.PresignUploads(ctx, domain.VideoPresignRequest{
			Files: []domain.VideoPresignFile{
				{Name: "v720.m3u8"},
				{Name: "v720_init.mp4"},
				{Name: "v720_0.m4s"},
				{Name: "index.m3u8"},
			},
		})
		require.NoError(t, err)
		require.Len(t, res.Items, 4)
		assert.True(t, strings.HasPrefix(res.Prefix, "hls/"))
		assert.True(t, strings.HasSuffix(res.Prefix, "/"))
		// index.m3u8 wins over the earlier v720.m3u8 as the playlist key.
		assert.Equal(t, res.Prefix+"index.m3u8", res.PlaylistKey)

		byName := map[string]service.VideoPresignItem{}
		for _, it := range res.Items {
			byName[it.Name] = it
			assert.Equal(t, res.Prefix+it.Name, it.Key)
			assert.Equal(t, "https://r2/"+it.Key, it.URL)
		}
		assert.Equal(t, "application/vnd.apple.mpegurl", byName["v720.m3u8"].ContentType)
		assert.Equal(t, "video/mp4", byName["v720_init.mp4"].ContentType)
		assert.Equal(t, "video/mp4", byName["v720_0.m4s"].ContentType)
	})

	t.Run("honors a client-provided content type and a custom prefix", func(t *testing.T) {
		svc, _, store := newVideoSvc(t, "")
		store.EXPECT().
			PresignPut(mock.Anything, "hls/custom/index.m3u8", 30*time.Minute, "application/x-mpegURL").
			Return("https://r2/x", nil).Once()

		res, err := svc.PresignUploads(context.Background(), domain.VideoPresignRequest{
			Prefix: "hls/custom", // no trailing slash — normalized
			Files:  []domain.VideoPresignFile{{Name: "index.m3u8", ContentType: "application/x-mpegURL"}},
		})
		require.NoError(t, err)
		assert.Equal(t, "hls/custom/index.m3u8", res.PlaylistKey)
	})

	t.Run("rejects empty and oversized batches", func(t *testing.T) {
		svc, _, _ := newVideoSvc(t, "")
		_, err := svc.PresignUploads(context.Background(), domain.VideoPresignRequest{})
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		big := make([]domain.VideoPresignFile, 5001)
		_, err = svc.PresignUploads(context.Background(), domain.VideoPresignRequest{Files: big})
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("rejects a bundle with no playlist", func(t *testing.T) {
		svc, _, store := newVideoSvc(t, "")
		store.EXPECT().
			PresignPut(mock.Anything, mock.Anything, 30*time.Minute, mock.Anything).
			Return("https://r2/x", nil).Once()

		_, err := svc.PresignUploads(context.Background(), domain.VideoPresignRequest{
			Files: []domain.VideoPresignFile{{Name: "v720_0.m4s"}},
		})
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("rejects an empty file name", func(t *testing.T) {
		svc, _, _ := newVideoSvc(t, "")
		_, err := svc.PresignUploads(context.Background(), domain.VideoPresignRequest{
			Files: []domain.VideoPresignFile{{Name: "  "}},
		})
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("propagates presign error", func(t *testing.T) {
		svc, _, store := newVideoSvc(t, "")
		boom := errors.New("presign boom")
		store.EXPECT().PresignPut(mock.Anything, mock.Anything, 30*time.Minute, mock.Anything).
			Return("", boom).Once()

		_, err := svc.PresignUploads(context.Background(), domain.VideoPresignRequest{
			Files: []domain.VideoPresignFile{{Name: "index.m3u8"}},
		})
		assert.ErrorIs(t, err, boom)
	})
}
