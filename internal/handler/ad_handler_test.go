package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/handler"
	"github.com/triasbrata/mihon-manga-server/internal/handler/mocks"
	"github.com/triasbrata/mihon-manga-server/internal/service"
)

func TestAdHandler_List_ReaderPlacement(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().ListActive(mock.Anything, "reader_interstitial").
		Return([]domain.Ad{{ID: "a1", ImageURL: "https://r2/s", ClickURL: "https://x"}}, nil)
	h := handler.NewAdHandler(svc)

	c := newCtx("GET", "/v1/ads?placement=reader_interstitial", nil, nil, "")
	h.List(context.Background(), c)

	var got []domain.Ad
	decodeJSON(t, c, &got)
	require.Len(t, got, 1)
	assert.Equal(t, "a1", got[0].ID)
	assert.Equal(t, "https://r2/s", got[0].ImageURL)
}

func TestAdHandler_AdminList_All(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().List(mock.Anything).Return([]domain.StoredAd{{ID: "a"}, {ID: "b"}}, nil)
	h := handler.NewAdHandler(svc)

	c := newCtx("GET", "/v1/admin/ads", nil, nil, "")
	h.AdminList(context.Background(), c)

	var got []domain.StoredAd
	decodeJSON(t, c, &got)
	assert.Len(t, got, 2)
}

func TestAdHandler_Get_NotFound(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().Get(mock.Anything, "missing").Return(domain.StoredAd{}, domain.ErrNotFound)
	h := handler.NewAdHandler(svc)
	c := newCtx("GET", "/v1/admin/ads/missing", map[string]string{"id": "missing"}, nil, "")
	h.Get(context.Background(), c)
	assert.Equal(t, 404, c.Response.StatusCode())
}

func TestAdHandler_Create(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.AdWriteRequest) bool {
		return r.R2Key == "ads/k1" && r.Placement == "reader_interstitial"
	})).Return(domain.StoredAd{ID: "a1", R2Key: "ads/k1"}, nil)
	h := handler.NewAdHandler(svc)

	c := newCtx("POST", "/v1/admin/ads", nil, []byte(`{"r2Key":"ads/k1","placement":"reader_interstitial"}`), "application/json")
	h.Create(context.Background(), c)

	assert.Equal(t, 201, c.Response.StatusCode())
	var got domain.StoredAd
	decodeJSON(t, c, &got)
	assert.Equal(t, "a1", got.ID)
}

func TestAdHandler_Create_BadJSON(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	h := handler.NewAdHandler(svc)
	c := newCtx("POST", "/v1/admin/ads", nil, []byte(`{bad`), "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
	assert.Equal(t, "invalid_input", decodeError(t, c).ErrorCode)
}

func TestAdHandler_Create_ValidationError(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().Create(mock.Anything, mock.Anything).Return(domain.StoredAd{}, domain.ErrInvalidInput)
	h := handler.NewAdHandler(svc)
	c := newCtx("POST", "/v1/admin/ads", nil, []byte(`{}`), "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
}

func TestAdHandler_Update(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().Update(mock.Anything, "a1", mock.Anything).Return(domain.StoredAd{ID: "a1", R2Key: "ads/k2"}, nil)
	h := handler.NewAdHandler(svc)
	c := newCtx("PUT", "/v1/admin/ads/a1", map[string]string{"id": "a1"}, []byte(`{"r2Key":"ads/k2"}`), "application/json")
	h.Update(context.Background(), c)
	var got domain.StoredAd
	decodeJSON(t, c, &got)
	assert.Equal(t, "ads/k2", got.R2Key)
}

func TestAdHandler_Update_BadJSON(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	h := handler.NewAdHandler(svc)
	c := newCtx("PUT", "/v1/admin/ads/a1", map[string]string{"id": "a1"}, []byte(`{bad`), "application/json")
	h.Update(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
}

func TestAdHandler_Delete(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().Delete(mock.Anything, "a1").Return(nil)
	h := handler.NewAdHandler(svc)
	c := newCtx("DELETE", "/v1/admin/ads/a1", map[string]string{"id": "a1"}, nil, "")
	h.Delete(context.Background(), c)
	assert.Equal(t, 204, c.Response.StatusCode())
}

func TestAdHandler_Presign(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().PresignUpload(mock.Anything, "image/png").
		Return(service.PresignItem{Key: "ads/uuid", URL: "https://r2/put"}, nil)
	h := handler.NewAdHandler(svc)
	c := newCtx("POST", "/v1/admin/ads/presign", nil, []byte(`{"contentType":"image/png"}`), "application/json")
	h.Presign(context.Background(), c)

	var got service.PresignItem
	decodeJSON(t, c, &got)
	assert.Equal(t, "ads/uuid", got.Key)
	assert.Equal(t, "https://r2/put", got.URL)
}

func TestAdHandler_Presign_EmptyBody(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	svc.EXPECT().PresignUpload(mock.Anything, "").
		Return(service.PresignItem{Key: "ads/uuid", URL: "https://r2/put"}, nil)
	h := handler.NewAdHandler(svc)
	c := newCtx("POST", "/v1/admin/ads/presign", nil, nil, "")
	h.Presign(context.Background(), c)
	assert.Equal(t, 200, c.Response.StatusCode())
}

func TestAdHandler_Presign_BadJSON(t *testing.T) {
	svc := mocks.NewMockAdService(t)
	h := handler.NewAdHandler(svc)
	c := newCtx("POST", "/v1/admin/ads/presign", nil, []byte(`{bad`), "application/json")
	h.Presign(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
}
