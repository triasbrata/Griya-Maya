package handler_test

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/handler"
	"github.com/triasbrata/mihon-manga-server/internal/handler/mocks"
	"github.com/triasbrata/mihon-manga-server/internal/service"
)

func TestConvertHandler_Presign_OK(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	h := handler.NewConvertHandler(svc)

	want := service.PresignResult{
		Prefix: "pages/abc/",
		Items:  []service.PresignItem{{Key: "pages/abc/page-0000.avif", URL: "https://r2/put"}},
	}
	svc.EXPECT().PresignUploads(mock.Anything, 1, "").Return(want, nil)

	c := newCtx("POST", "/v1/convert/presign", nil, []byte(`{"count":1}`), "application/json")
	h.Presign(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var out service.PresignResult
	decodeJSON(t, c, &out)
	assert.Equal(t, "pages/abc/", out.Prefix)
	assert.Len(t, out.Items, 1)
}

func TestConvertHandler_Presign_BadJSON(t *testing.T) {
	h := handler.NewConvertHandler(mocks.NewMockConvertService(t))
	c := newCtx("POST", "/v1/convert/presign", nil, []byte(`{bad`), "application/json")
	h.Presign(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConvertHandler_Presign_ServiceError(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	h := handler.NewConvertHandler(svc)
	svc.EXPECT().PresignUploads(mock.Anything, 0, "").Return(service.PresignResult{}, domain.ErrInvalidInput)

	c := newCtx("POST", "/v1/convert/presign", nil, []byte(`{"count":0}`), "application/json")
	h.Presign(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConvertHandler_RegisterPages_OK(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	h := handler.NewConvertHandler(svc)

	svc.EXPECT().RegisterPages(mock.Anything, "ch1", []domain.StoredPage{
		{Index: 0, R2Key: "pages/x/page-0000.avif", Width: 800, Height: 1200},
	}).Return([]domain.Page{{Index: 0, ImageURL: "/v1/image?key=pages/x/page-0000.avif", Width: 800, Height: 1200}}, nil)

	body := []byte(`{"pages":[{"index":0,"r2Key":"pages/x/page-0000.avif","width":800,"height":1200}]}`)
	c := newCtx("POST", "/v1/chapters/ch1/pages", map[string]string{"id": "ch1"}, body, "application/json")
	h.RegisterPages(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var out []domain.Page
	decodeJSON(t, c, &out)
	require.Len(t, out, 1)
	assert.Equal(t, "/v1/image?key=pages/x/page-0000.avif", out[0].ImageURL)
}

func TestConvertHandler_RegisterPages_BadJSON(t *testing.T) {
	h := handler.NewConvertHandler(mocks.NewMockConvertService(t))
	c := newCtx("POST", "/v1/chapters/ch1/pages", map[string]string{"id": "ch1"}, []byte(`{bad`), "application/json")
	h.RegisterPages(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConvertHandler_RegisterPages_ServiceError(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	h := handler.NewConvertHandler(svc)
	svc.EXPECT().RegisterPages(mock.Anything, "ch1", mock.Anything).Return(nil, domain.ErrInvalidInput)

	c := newCtx("POST", "/v1/chapters/ch1/pages", map[string]string{"id": "ch1"}, []byte(`{"pages":[]}`), "application/json")
	h.RegisterPages(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}
