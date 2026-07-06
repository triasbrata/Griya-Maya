package service_test

import (
	"context"
	"errors"
	"fmt"
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

func newConvertSvc(t *testing.T) (*service.ConvertService, *mocks.MockJobRepository, *mocks.MockObjectStore) {
	t.Helper()
	jobs := mocks.NewMockJobRepository(t)
	store := mocks.NewMockObjectStore(t)
	return service.NewConvertService(jobs, store), jobs, store
}

func TestConvertService_PresignUploads(t *testing.T) {
	t.Run("mints count items with distinct keys under one prefix", func(t *testing.T) {
		svc, _, store := newConvertSvc(t)
		ctx := context.Background()

		// Echo the key back as the URL so we can assert per-item wiring.
		store.EXPECT().
			PresignPut(mock.Anything, mock.Anything, 30*time.Minute, "image/avif").
			RunAndReturn(func(_ context.Context, key string, _ time.Duration, _ string) (string, error) {
				return "https://r2/" + key, nil
			}).Times(3)

		res, err := svc.PresignUploads(ctx, 3, "")
		require.NoError(t, err)
		require.Len(t, res.Items, 3)
		assert.True(t, strings.HasPrefix(res.Prefix, "pages/"))
		assert.True(t, strings.HasSuffix(res.Prefix, "/"))

		seen := map[string]bool{}
		for i, it := range res.Items {
			assert.Equal(t, fmt.Sprintf("%spage-%04d.avif", res.Prefix, i), it.Key)
			assert.Equal(t, "https://r2/"+it.Key, it.URL)
			assert.False(t, seen[it.Key], "duplicate key %s", it.Key)
			seen[it.Key] = true
		}
	})

	t.Run("passes through custom content type", func(t *testing.T) {
		svc, _, store := newConvertSvc(t)
		store.EXPECT().
			PresignPut(mock.Anything, mock.Anything, 30*time.Minute, "image/webp").
			Return("https://r2/x", nil).Once()

		res, err := svc.PresignUploads(context.Background(), 1, "image/webp")
		require.NoError(t, err)
		require.Len(t, res.Items, 1)
	})

	t.Run("rejects invalid count", func(t *testing.T) {
		for _, count := range []int{0, -1, 5001} {
			svc, _, _ := newConvertSvc(t)
			_, err := svc.PresignUploads(context.Background(), count, "")
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
		}
	})

	t.Run("propagates presign error", func(t *testing.T) {
		svc, _, store := newConvertSvc(t)
		boom := errors.New("presign boom")
		store.EXPECT().PresignPut(mock.Anything, mock.Anything, 30*time.Minute, "image/avif").
			Return("", boom).Once()

		_, err := svc.PresignUploads(context.Background(), 2, "")
		assert.ErrorIs(t, err, boom)
	})
}

func TestConvertService_RegisterPages(t *testing.T) {
	t.Run("maps and replaces pages, returns resolved pages", func(t *testing.T) {
		svc, jobs, store := newConvertSvc(t)
		ctx := context.Background()
		in := []domain.StoredPage{
			{Index: 0, R2Key: "pages/x/page-0000.avif", Width: 800, Height: 1200},
			{Index: 1, R2Key: "pages/x/page-0001.avif", Width: 801, Height: 1201},
		}

		jobs.EXPECT().ReplacePages(ctx, "ch1", in).Return(nil)
		store.EXPECT().PublicURL("pages/x/page-0000.avif").Return("")
		store.EXPECT().PublicURL("pages/x/page-0001.avif").Return("https://cdn/pages/x/page-0001.avif")

		out, err := svc.RegisterPages(ctx, "ch1", in)
		require.NoError(t, err)
		require.Len(t, out, 2)
		assert.Equal(t, "/v1/image?key=pages/x/page-0000.avif", out[0].ImageURL)
		assert.Equal(t, "https://cdn/pages/x/page-0001.avif", out[1].ImageURL)
		assert.Equal(t, 800, out[0].Width)
	})

	t.Run("rejects empty pages", func(t *testing.T) {
		svc, _, _ := newConvertSvc(t)
		_, err := svc.RegisterPages(context.Background(), "ch1", nil)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("rejects page missing r2Key", func(t *testing.T) {
		svc, _, _ := newConvertSvc(t)
		_, err := svc.RegisterPages(context.Background(), "ch1", []domain.StoredPage{
			{Index: 0, R2Key: "ok"},
			{Index: 1, R2Key: "  "},
		})
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("propagates ReplacePages error", func(t *testing.T) {
		svc, jobs, _ := newConvertSvc(t)
		boom := errors.New("db boom")
		jobs.EXPECT().ReplacePages(mock.Anything, "ch1", mock.Anything).Return(boom)

		_, err := svc.RegisterPages(context.Background(), "ch1", []domain.StoredPage{{Index: 0, R2Key: "k"}})
		assert.ErrorIs(t, err, boom)
	})
}
