package handler_test

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/handler"
	"github.com/triasbrata/mihon-manga-server/internal/handler/mocks"
)

func TestHealthHandler_Health(t *testing.T) {
	h := handler.NewHealthHandler()
	c := newCtx("GET", "/healthz", nil, nil, "")
	h.Health(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var body map[string]string
	decodeJSON(t, c, &body)
	assert.Equal(t, "ok", body["status"])
}

func TestNovelHandler_Register(t *testing.T) {
	svc := mocks.NewMockNovelService(t)
	h := handler.NewNovelHandler(svc)

	svc.EXPECT().Register(mock.Anything, domain.NovelRegisterRequest{ChapterID: "c1", Text: "hi"}).
		Return(domain.Page{Type: domain.PageKindNovel, Body: "hi"}, nil)

	body := []byte(`{"chapterId":"c1","text":"hi"}`)
	c := newCtx("POST", "/v1/novel", nil, body, "application/json")
	h.Register(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestNovelHandler_Register_BadJSON(t *testing.T) {
	h := handler.NewNovelHandler(mocks.NewMockNovelService(t))
	c := newCtx("POST", "/v1/novel", nil, []byte("{bad"), "application/json")
	h.Register(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

// Error-path coverage: a service error surfaces through writeError with the
// mapped status.
func TestMediaHandler_ErrorPaths(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().Latest(mock.Anything, "src", 1, mock.Anything).
		Return(domain.MediaPage{}, domain.ErrInvalidInput)
	c := newCtx("GET", "/v1/sources/src/latest", map[string]string{"sourceId": "src"}, nil, "")
	h.Latest(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())

	svc.EXPECT().Search(mock.Anything, "src", "", 1, mock.Anything).
		Return(domain.MediaPage{}, domain.ErrNotFound)
	c = newCtx("GET", "/v1/sources/src/search", map[string]string{"sourceId": "src"}, nil, "")
	h.Search(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())

	svc.EXPECT().Genres(mock.Anything, "src").Return(nil, domain.ErrNotFound)
	c = newCtx("GET", "/v1/sources/src/genres", map[string]string{"sourceId": "src"}, nil, "")
	h.Genres(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())

	svc.EXPECT().Categories(mock.Anything, "src").Return(nil, domain.ErrNotFound)
	c = newCtx("GET", "/v1/sources/src/categories", map[string]string{"sourceId": "src"}, nil, "")
	h.Categories(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())

	svc.EXPECT().Chapters(mock.Anything, "m1").Return(nil, domain.ErrNotFound)
	c = newCtx("GET", "/v1/media/m1/chapters", map[string]string{"id": "m1"}, nil, "")
	h.Chapters(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())

	svc.EXPECT().Pages(mock.Anything, "c1").Return(nil, domain.ErrNotFound)
	c = newCtx("GET", "/v1/chapters/c1/pages", map[string]string{"id": "c1"}, nil, "")
	h.Pages(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())

	svc.EXPECT().UpdateMedia(mock.Anything, "m1", mock.Anything).
		Return(domain.Media{}, domain.ErrInvalidInput)
	c = newCtx("PUT", "/v1/media/m1", map[string]string{"id": "m1"}, []byte(`{}`), "application/json")
	h.UpdateMedia(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())

	svc.EXPECT().DeleteMedia(mock.Anything, "m1").Return(domain.ErrInvalidInput)
	c = newCtx("DELETE", "/v1/media/m1", map[string]string{"id": "m1"}, nil, "")
	h.DeleteMedia(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())

	svc.EXPECT().DeleteChapter(mock.Anything, "c1").Return(domain.ErrInvalidInput)
	c = newCtx("DELETE", "/v1/chapters/c1", map[string]string{"id": "c1"}, nil, "")
	h.DeleteChapter(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestMediaHandler_WriteBadJSON(t *testing.T) {
	h, _, _ := newMediaHandler(t)

	c := newCtx("PUT", "/v1/media/m1", map[string]string{"id": "m1"}, []byte("{bad"), "application/json")
	h.UpdateMedia(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())

	c = newCtx("POST", "/v1/media/m1/chapters", map[string]string{"id": "m1"}, []byte("{bad"), "application/json")
	h.CreateChapter(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())

	c = newCtx("PUT", "/v1/chapters/c1", map[string]string{"id": "c1"}, []byte("{bad"), "application/json")
	h.UpdateChapter(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestTaxonomyHandler_ErrorPaths(t *testing.T) {
	h, svc := newTaxonomyHandler(t)

	svc.EXPECT().List(mock.Anything, domain.TaxonomyGenre).Return(nil, domain.ErrInvalidInput)
	c := newCtx("GET", "/v1/taxonomies/genres", map[string]string{"kind": "genres"}, nil, "")
	h.List(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())

	// Bad JSON on create.
	c = newCtx("POST", "/v1/taxonomies/genres", map[string]string{"kind": "genres"}, []byte("{bad"), "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())

	// Unknown kind on create/update/delete → 404 before hitting the service.
	c = newCtx("POST", "/v1/taxonomies/bogus", map[string]string{"kind": "bogus"}, []byte(`{"name":"x"}`), "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())

	c = newCtx("PUT", "/v1/taxonomies/bogus/x", map[string]string{"kind": "bogus", "id": "x"}, []byte(`{"name":"y"}`), "application/json")
	h.Update(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())

	c = newCtx("DELETE", "/v1/taxonomies/bogus/x", map[string]string{"kind": "bogus", "id": "x"}, nil, "")
	h.Delete(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())
}
