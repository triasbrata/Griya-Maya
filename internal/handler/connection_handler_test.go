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
)

func newConnectionHandler(t *testing.T) (*handler.ConnectionHandler, *mocks.MockConnectionService) {
	t.Helper()
	svc := mocks.NewMockConnectionService(t)
	return handler.NewConnectionHandler(svc), svc
}

func TestConnectionHandler_Create(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Create(mock.Anything, mock.MatchedBy(func(r domain.ConnectionWriteRequest) bool {
		return r.Provider == domain.ProviderMyAnimeList && r.ClientID == "abc123"
	})).Return(domain.Connection{
		ID: "c1", Provider: domain.ProviderMyAnimeList, ClientID: "abc123",
		ClientSecret: "should-not-serialize", Status: domain.ConnectionDisconnected,
	}, nil)

	body := []byte(`{"provider":"myanimelist","label":"My MAL","client_id":"abc123","client_secret":"topsecret"}`)
	c := newCtx("POST", "/v1/connections", nil, body, "application/json")
	h.Create(context.Background(), c)

	assert.Equal(t, consts.StatusCreated, c.Response.StatusCode())
	// The secret must not leak in the response body.
	assert.NotContains(t, string(c.Response.Body()), "should-not-serialize")
	assert.NotContains(t, string(c.Response.Body()), "client_secret")
	var got domain.Connection
	decodeJSON(t, c, &got)
	assert.Equal(t, "c1", got.ID)
	assert.Equal(t, "abc123", got.ClientID)
}

func TestConnectionHandler_Create_BindError(t *testing.T) {
	h, _ := newConnectionHandler(t)
	c := newCtx("POST", "/v1/connections", nil, []byte(`{bad json`), "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConnectionHandler_Create_ServiceError(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Create(mock.Anything, mock.Anything).Return(domain.Connection{}, domain.ErrInvalidInput)
	body := []byte(`{"provider":"imdb"}`)
	c := newCtx("POST", "/v1/connections", nil, body, "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConnectionHandler_List(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().List(mock.Anything).Return([]domain.Connection{{ID: "c1"}, {ID: "c2"}}, nil)
	c := newCtx("GET", "/v1/connections", nil, nil, "")
	h.List(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var got []domain.Connection
	decodeJSON(t, c, &got)
	require.Len(t, got, 2)
}

func TestConnectionHandler_Get(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Get(mock.Anything, "c1").Return(domain.Connection{ID: "c1"}, nil)
	c := newCtx("GET", "/v1/connections/c1", map[string]string{"id": "c1"}, nil, "")
	h.Get(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestConnectionHandler_Get_NotFound(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Get(mock.Anything, "missing").Return(domain.Connection{}, domain.ErrNotFound)
	c := newCtx("GET", "/v1/connections/missing", map[string]string{"id": "missing"}, nil, "")
	h.Get(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())
}

func TestConnectionHandler_Update(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Update(mock.Anything, "c1", mock.Anything).Return(domain.Connection{ID: "c1", Label: "New"}, nil)
	body := []byte(`{"label":"New"}`)
	c := newCtx("PUT", "/v1/connections/c1", map[string]string{"id": "c1"}, body, "application/json")
	h.Update(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestConnectionHandler_Update_BindError(t *testing.T) {
	h, _ := newConnectionHandler(t)
	c := newCtx("PUT", "/v1/connections/c1", map[string]string{"id": "c1"}, []byte(`{bad`), "application/json")
	h.Update(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConnectionHandler_Delete(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Delete(mock.Anything, "c1").Return(nil)
	c := newCtx("DELETE", "/v1/connections/c1", map[string]string{"id": "c1"}, nil, "")
	h.Delete(context.Background(), c)
	assert.Equal(t, consts.StatusNoContent, c.Response.StatusCode())
}

func TestConnectionHandler_Authorize(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Authorize(mock.Anything, "c1", "https://cb").
		Return("https://myanimelist.net/v1/oauth2/authorize?state=x", nil)
	body := []byte(`{"redirect_uri":"https://cb"}`)
	c := newCtx("POST", "/v1/connections/c1/authorize", map[string]string{"id": "c1"}, body, "application/json")
	h.Authorize(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var got map[string]string
	decodeJSON(t, c, &got)
	assert.Contains(t, got["authorize_url"], "myanimelist.net")
}

func TestConnectionHandler_Authorize_BindError(t *testing.T) {
	h, _ := newConnectionHandler(t)
	c := newCtx("POST", "/v1/connections/c1/authorize", map[string]string{"id": "c1"}, []byte(`{bad`), "application/json")
	h.Authorize(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConnectionHandler_Authorize_ServiceError(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Authorize(mock.Anything, "c1", "").Return("", domain.ErrInvalidInput)
	body := []byte(`{"redirect_uri":""}`)
	c := newCtx("POST", "/v1/connections/c1/authorize", map[string]string{"id": "c1"}, body, "application/json")
	h.Authorize(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConnectionHandler_Callback(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Callback(mock.Anything, "code-1", "st1").
		Return(domain.Connection{ID: "c1", Status: domain.ConnectionConnected, AccessToken: "leak"}, nil)
	body := []byte(`{"code":"code-1","state":"st1"}`)
	c := newCtx("POST", "/v1/connections/callback", nil, body, "application/json")
	h.Callback(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	assert.NotContains(t, string(c.Response.Body()), "leak") // tokens redacted
	var got domain.Connection
	decodeJSON(t, c, &got)
	assert.Equal(t, domain.ConnectionConnected, got.Status)
}

func TestConnectionHandler_Callback_BindError(t *testing.T) {
	h, _ := newConnectionHandler(t)
	c := newCtx("POST", "/v1/connections/callback", nil, []byte(`{bad`), "application/json")
	h.Callback(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConnectionHandler_Refresh(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Refresh(mock.Anything, "c1").Return(domain.Connection{ID: "c1", Status: domain.ConnectionConnected}, nil)
	c := newCtx("POST", "/v1/connections/c1/refresh", map[string]string{"id": "c1"}, nil, "")
	h.Refresh(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestConnectionHandler_Refresh_Error(t *testing.T) {
	h, svc := newConnectionHandler(t)
	svc.EXPECT().Refresh(mock.Anything, "c1").Return(domain.Connection{}, domain.ErrInvalidInput)
	c := newCtx("POST", "/v1/connections/c1/refresh", map[string]string{"id": "c1"}, nil, "")
	h.Refresh(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}
