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
)

func newSubTypeHandler(t *testing.T) (*handler.SubTypeHandler, *mocks.MockSubTypeService) {
	t.Helper()
	svc := mocks.NewMockSubTypeService(t)
	return handler.NewSubTypeHandler(svc), svc
}

func TestSubTypeHandler_List(t *testing.T) {
	h, svc := newSubTypeHandler(t)
	svc.EXPECT().List(mock.Anything).Return([]domain.SubType{
		{Slug: "manhwa", Type: domain.MediaManga, Name: "Manhwa"},
	}, nil)

	c := newCtx("GET", "/v1/admin/subtypes", nil, nil, "")
	h.List(context.Background(), c)

	assert.Equal(t, 200, c.Response.StatusCode())
	var got []domain.SubType
	decodeJSON(t, c, &got)
	require.Len(t, got, 1)
	assert.Equal(t, "manhwa", got[0].Slug)
	assert.Equal(t, domain.MediaManga, got[0].Type)
}

func TestSubTypeHandler_Create(t *testing.T) {
	h, svc := newSubTypeHandler(t)
	svc.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.SubTypeWriteRequest) bool {
		return r.Slug == "webtoon" && r.Type == domain.MediaManga && r.Name == "Webtoon"
	})).Return(domain.SubType{Slug: "webtoon", Type: domain.MediaManga, Name: "Webtoon"}, nil)

	c := newCtx("POST", "/v1/admin/subtypes", nil,
		[]byte(`{"slug":"webtoon","type":"manga","name":"Webtoon"}`), "application/json")
	h.Create(context.Background(), c)

	assert.Equal(t, 201, c.Response.StatusCode())
	var got domain.SubType
	decodeJSON(t, c, &got)
	assert.Equal(t, "webtoon", got.Slug)
}

func TestSubTypeHandler_Create_BadJSON(t *testing.T) {
	h, _ := newSubTypeHandler(t)
	c := newCtx("POST", "/v1/admin/subtypes", nil, []byte(`{bad`), "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
	assert.Equal(t, "invalid_input", decodeError(t, c).ErrorCode)
}

func TestSubTypeHandler_Create_ServiceError(t *testing.T) {
	h, svc := newSubTypeHandler(t)
	svc.EXPECT().Create(mock.Anything, mock.Anything).Return(domain.SubType{}, domain.ErrInvalidInput)
	c := newCtx("POST", "/v1/admin/subtypes", nil, []byte(`{"slug":"x","type":"bogus","name":"X"}`), "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
}

func TestSubTypeHandler_Update(t *testing.T) {
	h, svc := newSubTypeHandler(t)
	svc.EXPECT().Update(mock.Anything, "manhwa", mock.Anything).
		Return(domain.SubType{Slug: "manhwa", Type: domain.MediaManga, Name: "Manhwa KR"}, nil)

	c := newCtx("PUT", "/v1/admin/subtypes/manhwa", map[string]string{"slug": "manhwa"},
		[]byte(`{"type":"manga","name":"Manhwa KR"}`), "application/json")
	h.Update(context.Background(), c)

	assert.Equal(t, 200, c.Response.StatusCode())
	var got domain.SubType
	decodeJSON(t, c, &got)
	assert.Equal(t, "Manhwa KR", got.Name)
}

func TestSubTypeHandler_Update_BadJSON(t *testing.T) {
	h, _ := newSubTypeHandler(t)
	c := newCtx("PUT", "/v1/admin/subtypes/manhwa", map[string]string{"slug": "manhwa"}, []byte(`{bad`), "application/json")
	h.Update(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
}

func TestSubTypeHandler_Update_NotFound(t *testing.T) {
	h, svc := newSubTypeHandler(t)
	svc.EXPECT().Update(mock.Anything, "missing", mock.Anything).Return(domain.SubType{}, domain.ErrNotFound)
	c := newCtx("PUT", "/v1/admin/subtypes/missing", map[string]string{"slug": "missing"},
		[]byte(`{"type":"manga","name":"X"}`), "application/json")
	h.Update(context.Background(), c)
	assert.Equal(t, 404, c.Response.StatusCode())
}

func TestSubTypeHandler_Delete(t *testing.T) {
	h, svc := newSubTypeHandler(t)
	svc.EXPECT().Delete(mock.Anything, "manhwa").Return(nil)
	c := newCtx("DELETE", "/v1/admin/subtypes/manhwa", map[string]string{"slug": "manhwa"}, nil, "")
	h.Delete(context.Background(), c)
	assert.Equal(t, 204, c.Response.StatusCode())
}

func TestSubTypeHandler_Delete_Error(t *testing.T) {
	h, svc := newSubTypeHandler(t)
	svc.EXPECT().Delete(mock.Anything, "x").Return(domain.ErrInvalidInput)
	c := newCtx("DELETE", "/v1/admin/subtypes/x", map[string]string{"slug": "x"}, nil, "")
	h.Delete(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
}
