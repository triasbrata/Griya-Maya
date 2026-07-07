package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

func newAdSvc(t *testing.T) (*AdService, *mocks.MockAdRepository, *mocks.MockObjectStore) {
	t.Helper()
	repo := mocks.NewMockAdRepository(t)
	store := mocks.NewMockObjectStore(t)
	return NewAdService(repo, store, time.Hour), repo, store
}

func TestAdService_ListActive_PresignsAndMaps(t *testing.T) {
	svc, repo, store := newAdSvc(t)
	repo.EXPECT().List(mock.Anything, true, "reader_interstitial").Return([]domain.StoredAd{
		{ID: "a1", R2Key: "ads/k1", ClickURL: "https://x", Weight: 5, Placement: "reader_interstitial", Width: 600, Height: 300, Active: true},
	}, nil)
	store.EXPECT().PresignGet(mock.Anything, "ads/k1", time.Hour).Return("https://r2/signed", nil)

	got, err := svc.ListActive(context.Background(), "reader_interstitial")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "https://r2/signed", got[0].ImageURL)
	assert.Equal(t, "https://x", got[0].ClickURL)
	assert.Equal(t, 2.0, got[0].AspectRatio) // 600/300
	assert.Equal(t, 5, got[0].Weight)
}

func TestAdService_ListActive_TrimsPlacementAndZeroAspect(t *testing.T) {
	svc, repo, store := newAdSvc(t)
	repo.EXPECT().List(mock.Anything, true, "").Return([]domain.StoredAd{
		{ID: "a1", R2Key: "ads/k1"}, // no dimensions -> aspect omitted
	}, nil)
	store.EXPECT().PresignGet(mock.Anything, "ads/k1", time.Hour).Return("https://r2/s", nil)

	got, err := svc.ListActive(context.Background(), "   ")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Zero(t, got[0].AspectRatio)
}

func TestAdService_ListActive_PresignError(t *testing.T) {
	svc, repo, store := newAdSvc(t)
	repo.EXPECT().List(mock.Anything, true, "").Return([]domain.StoredAd{{ID: "a1", R2Key: "ads/k1"}}, nil)
	store.EXPECT().PresignGet(mock.Anything, "ads/k1", time.Hour).Return("", errors.New("boom"))
	_, err := svc.ListActive(context.Background(), "")
	require.Error(t, err)
}

func TestAdService_List_Admin(t *testing.T) {
	svc, repo, _ := newAdSvc(t)
	repo.EXPECT().List(mock.Anything, false, "").Return([]domain.StoredAd{{ID: "a"}, {ID: "b"}}, nil)
	got, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestAdService_Get_Validation(t *testing.T) {
	svc, _, _ := newAdSvc(t)
	_, err := svc.Get(context.Background(), "  ")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestAdService_Get(t *testing.T) {
	svc, repo, _ := newAdSvc(t)
	repo.EXPECT().Get(mock.Anything, "a1").Return(domain.StoredAd{ID: "a1"}, nil)
	got, err := svc.Get(context.Background(), "a1")
	require.NoError(t, err)
	assert.Equal(t, "a1", got.ID)
}

func TestAdService_Create_DefaultsAndPersists(t *testing.T) {
	svc, repo, _ := newAdSvc(t)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(a domain.StoredAd) bool {
		return a.R2Key == "ads/k1" && a.Weight == 1 && a.Active // defaults applied
	})).Return(nil)
	repo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.StoredAd{R2Key: "ads/k1", Weight: 1, Active: true}, nil)

	got, err := svc.Create(context.Background(), domain.AdWriteRequest{R2Key: "ads/k1"})
	require.NoError(t, err)
	assert.Equal(t, 1, got.Weight)
	assert.True(t, got.Active)
}

func TestAdService_Create_ActiveFalseHonored(t *testing.T) {
	svc, repo, _ := newAdSvc(t)
	f := false
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(a domain.StoredAd) bool { return !a.Active && a.Weight == 3 })).Return(nil)
	repo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.StoredAd{}, nil)
	_, err := svc.Create(context.Background(), domain.AdWriteRequest{R2Key: "ads/k1", Weight: 3, Active: &f})
	require.NoError(t, err)
}

func TestAdService_Create_Validation(t *testing.T) {
	svc, _, _ := newAdSvc(t)
	_, err := svc.Create(context.Background(), domain.AdWriteRequest{}) // missing r2Key
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestAdService_Update(t *testing.T) {
	svc, repo, _ := newAdSvc(t)
	repo.EXPECT().Get(mock.Anything, "a1").Return(domain.StoredAd{ID: "a1"}, nil).Once()
	repo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(a domain.StoredAd) bool {
		return a.ID == "a1" && a.R2Key == "ads/k2"
	})).Return(nil)
	repo.EXPECT().Get(mock.Anything, "a1").Return(domain.StoredAd{ID: "a1", R2Key: "ads/k2"}, nil).Once()

	got, err := svc.Update(context.Background(), "a1", domain.AdWriteRequest{R2Key: "ads/k2"})
	require.NoError(t, err)
	assert.Equal(t, "ads/k2", got.R2Key)
}

func TestAdService_Update_NotFound(t *testing.T) {
	svc, repo, _ := newAdSvc(t)
	repo.EXPECT().Get(mock.Anything, "missing").Return(domain.StoredAd{}, domain.ErrNotFound)
	_, err := svc.Update(context.Background(), "missing", domain.AdWriteRequest{R2Key: "ads/k"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAdService_Update_Validation(t *testing.T) {
	svc, _, _ := newAdSvc(t)
	_, err := svc.Update(context.Background(), "  ", domain.AdWriteRequest{R2Key: "x"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = svc.Update(context.Background(), "a1", domain.AdWriteRequest{}) // missing r2Key
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestAdService_Delete(t *testing.T) {
	svc, repo, _ := newAdSvc(t)
	repo.EXPECT().Delete(mock.Anything, "a1").Return(nil)
	require.NoError(t, svc.Delete(context.Background(), "a1"))
}

func TestAdService_Delete_Validation(t *testing.T) {
	svc, _, _ := newAdSvc(t)
	assert.ErrorIs(t, svc.Delete(context.Background(), "  "), domain.ErrInvalidInput)
}

func TestAdService_PresignUpload_Defaults(t *testing.T) {
	svc, _, store := newAdSvc(t)
	store.EXPECT().PresignPut(mock.Anything, mock.MatchedBy(func(k string) bool {
		return len(k) > 4 && k[:4] == "ads/"
	}), adPresignPutTTL, "image/avif").Return("https://r2/put", nil)
	item, err := svc.PresignUpload(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "https://r2/put", item.URL)
	assert.Contains(t, item.Key, "ads/")
}

func TestAdService_PresignUpload_CustomContentType(t *testing.T) {
	svc, _, store := newAdSvc(t)
	store.EXPECT().PresignPut(mock.Anything, mock.Anything, adPresignPutTTL, "image/png").Return("https://r2/put", nil)
	_, err := svc.PresignUpload(context.Background(), "image/png")
	require.NoError(t, err)
}

func TestAdService_PresignUpload_Error(t *testing.T) {
	svc, _, store := newAdSvc(t)
	store.EXPECT().PresignPut(mock.Anything, mock.Anything, adPresignPutTTL, "image/avif").Return("", errors.New("boom"))
	_, err := svc.PresignUpload(context.Background(), "")
	require.Error(t, err)
}
