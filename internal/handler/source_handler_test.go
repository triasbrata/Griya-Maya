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

func TestSourceHandler_List_EnabledOnly(t *testing.T) {
	svc := mocks.NewMockSourceService(t)
	svc.EXPECT().List(mock.Anything, true).Return([]domain.Source{{ID: "griyamedia", Name: "GriyaMedia"}}, nil)
	h := handler.NewSourceHandler(svc)

	c := newCtx("GET", "/v1/sources", nil, nil, "")
	h.List(context.Background(), c)

	var got []domain.Source
	decodeJSON(t, c, &got)
	require.Len(t, got, 1)
	assert.Equal(t, "griyamedia", got[0].ID)
}

func TestSourceHandler_AdminList_All(t *testing.T) {
	svc := mocks.NewMockSourceService(t)
	svc.EXPECT().List(mock.Anything, false).Return([]domain.Source{{ID: "a"}, {ID: "b"}}, nil)
	h := handler.NewSourceHandler(svc)

	c := newCtx("GET", "/v1/admin/sources", nil, nil, "")
	h.AdminList(context.Background(), c)

	var got []domain.Source
	decodeJSON(t, c, &got)
	assert.Len(t, got, 2)
}

func TestSourceHandler_Create(t *testing.T) {
	svc := mocks.NewMockSourceService(t)
	svc.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.SourceWriteRequest) bool {
		return r.ID == "s1" && r.Name == "S1"
	})).Return(domain.Source{ID: "s1", Name: "S1"}, nil)
	h := handler.NewSourceHandler(svc)

	c := newCtx("POST", "/v1/admin/sources", nil, []byte(`{"id":"s1","name":"S1"}`), "application/json")
	h.Create(context.Background(), c)

	assert.Equal(t, 201, c.Response.StatusCode())
	var got domain.Source
	decodeJSON(t, c, &got)
	assert.Equal(t, "s1", got.ID)
}

func TestSourceHandler_Create_BadJSON(t *testing.T) {
	svc := mocks.NewMockSourceService(t)
	h := handler.NewSourceHandler(svc)
	c := newCtx("POST", "/v1/admin/sources", nil, []byte(`{bad`), "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
	assert.Equal(t, "invalid_input", decodeError(t, c).ErrorCode)
}

func TestSourceHandler_Get_NotFound(t *testing.T) {
	svc := mocks.NewMockSourceService(t)
	svc.EXPECT().Get(mock.Anything, "missing").Return(domain.Source{}, domain.ErrNotFound)
	h := handler.NewSourceHandler(svc)
	c := newCtx("GET", "/v1/admin/sources/missing", map[string]string{"id": "missing"}, nil, "")
	h.Get(context.Background(), c)
	assert.Equal(t, 404, c.Response.StatusCode())
}

func TestSourceHandler_Update(t *testing.T) {
	svc := mocks.NewMockSourceService(t)
	svc.EXPECT().Update(mock.Anything, "s1", mock.Anything).Return(domain.Source{ID: "s1", Name: "New"}, nil)
	h := handler.NewSourceHandler(svc)
	c := newCtx("PUT", "/v1/admin/sources/s1", map[string]string{"id": "s1"}, []byte(`{"name":"New"}`), "application/json")
	h.Update(context.Background(), c)
	var got domain.Source
	decodeJSON(t, c, &got)
	assert.Equal(t, "New", got.Name)
}

func TestSourceHandler_Delete(t *testing.T) {
	svc := mocks.NewMockSourceService(t)
	svc.EXPECT().Delete(mock.Anything, "s1").Return(nil)
	h := handler.NewSourceHandler(svc)
	c := newCtx("DELETE", "/v1/admin/sources/s1", map[string]string{"id": "s1"}, nil, "")
	h.Delete(context.Background(), c)
	assert.Equal(t, 204, c.Response.StatusCode())
}

func TestSourceHandler_Delete_Refused(t *testing.T) {
	svc := mocks.NewMockSourceService(t)
	svc.EXPECT().Delete(mock.Anything, "s1").Return(domain.ErrInvalidInput)
	h := handler.NewSourceHandler(svc)
	c := newCtx("DELETE", "/v1/admin/sources/s1", map[string]string{"id": "s1"}, nil, "")
	h.Delete(context.Background(), c)
	assert.Equal(t, 400, c.Response.StatusCode())
}
