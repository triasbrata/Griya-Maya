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
	svcmocks "github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

func newMediaHandler(t *testing.T) (*handler.MediaHandler, *mocks.MockMediaService, *svcmocks.MockObjectStore) {
	t.Helper()
	svc := mocks.NewMockMediaService(t)
	store := svcmocks.NewMockObjectStore(t)
	return handler.NewMediaHandler(svc, store), svc, store
}

func TestMediaHandler_Popular_ParsesQueryAndFilter(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	wantFilter := domain.CatalogFilter{
		Sort:              "title",
		Ascending:         true,
		Types:             []string{"video"},
		IncludeGenres:     []string{"action"},
		ExcludeGenres:     []string{"ecchi"},
		IncludeCategories: []string{"webtoon"},
		GenreMode:         domain.GenreModeAnd,
	}
	page := domain.MediaPage{Items: []domain.Media{{ID: "m1"}}, Page: 2, HasNext: true}
	svc.EXPECT().Popular(mock.Anything, "src", 2, wantFilter).Return(page, nil)

	c := newCtx("GET",
		"/v1/sources/src/popular?page=2&sort=title&order=asc&type=video&genre=action&genreExclude=ecchi&category=webtoon&genreMode=and",
		map[string]string{"sourceId": "src"}, nil, "")
	h.Popular(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var got domain.MediaPage
	decodeJSON(t, c, &got)
	assert.Equal(t, page, got)
}

func TestMediaHandler_Popular_DefaultsAndError(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().
		Popular(mock.Anything, "src", 1, domain.CatalogFilter{GenreMode: domain.GenreModeOr}).
		Return(domain.MediaPage{}, domain.ErrNotFound)

	c := newCtx("GET", "/v1/sources/src/popular", map[string]string{"sourceId": "src"}, nil, "")
	h.Popular(context.Background(), c)

	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())
}

func TestMediaHandler_Latest(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().Latest(mock.Anything, "src", 1, mock.Anything).Return(domain.MediaPage{Page: 1}, nil)

	c := newCtx("GET", "/v1/sources/src/latest", map[string]string{"sourceId": "src"}, nil, "")
	h.Latest(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestMediaHandler_Search(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().
		Search(mock.Anything, "src", "naruto", 1, mock.Anything).
		Return(domain.MediaPage{Items: []domain.Media{{ID: "hit"}}}, nil)

	c := newCtx("GET", "/v1/sources/src/search?q=naruto", map[string]string{"sourceId": "src"}, nil, "")
	h.Search(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestMediaHandler_Genres(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().Genres(mock.Anything, "src").
		Return([]domain.Taxonomy{{Slug: "action", Name: "Action"}}, nil)

	c := newCtx("GET", "/v1/sources/src/genres", map[string]string{"sourceId": "src"}, nil, "")
	h.Genres(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var got []domain.Taxonomy
	decodeJSON(t, c, &got)
	require.Len(t, got, 1)
	assert.Equal(t, "action", got[0].Slug)
}

func TestMediaHandler_Categories(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().Categories(mock.Anything, "src").
		Return([]domain.Taxonomy{{Slug: "webtoon", Name: "Webtoon"}}, nil)

	c := newCtx("GET", "/v1/sources/src/categories", map[string]string{"sourceId": "src"}, nil, "")
	h.Categories(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestMediaHandler_Details_NotFound(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().Details(mock.Anything, "missing").Return(domain.Media{}, domain.ErrNotFound)

	c := newCtx("GET", "/v1/media/missing", map[string]string{"id": "missing"}, nil, "")
	h.Details(context.Background(), c)

	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())
	assert.Equal(t, "not_found", decodeError(t, c).ErrorCode)
}

func TestMediaHandler_Chapters(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().Chapters(mock.Anything, "m1").Return([]domain.Chapter{{ID: "c1", MediaID: "m1"}}, nil)

	c := newCtx("GET", "/v1/media/m1/chapters", map[string]string{"id": "m1"}, nil, "")
	h.Chapters(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestMediaHandler_Pages(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().Pages(mock.Anything, "c1").
		Return([]domain.Page{{Index: 0, ImageURL: "https://cdn/x.avif"}}, nil)

	c := newCtx("GET", "/v1/chapters/c1/pages", map[string]string{"id": "c1"}, nil, "")
	h.Pages(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestMediaHandler_CreateMedia(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	wantReq := domain.MediaWriteRequest{
		SourceID: "src", Type: domain.MediaVideo, URL: "u", Title: "T", Genres: []string{"Action"},
	}
	created := domain.Media{ID: "new", SourceID: "src", Type: domain.MediaVideo, Title: "T"}
	svc.EXPECT().CreateMedia(mock.Anything, wantReq).Return(created, nil)

	body := []byte(`{"sourceId":"src","type":"video","url":"u","title":"T","genres":["Action"]}`)
	c := newCtx("POST", "/v1/media", nil, body, "application/json")
	h.CreateMedia(context.Background(), c)

	assert.Equal(t, consts.StatusCreated, c.Response.StatusCode())
	var got domain.Media
	decodeJSON(t, c, &got)
	assert.Equal(t, "new", got.ID)
}

func TestMediaHandler_CreateMedia_BadJSON(t *testing.T) {
	h, _, _ := newMediaHandler(t)

	c := newCtx("POST", "/v1/media", nil, []byte("{not json"), "application/json")
	h.CreateMedia(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestMediaHandler_UpdateMedia(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	wantReq := domain.MediaWriteRequest{SourceID: "src", URL: "u", Title: "T2"}
	svc.EXPECT().UpdateMedia(mock.Anything, "m1", wantReq).Return(domain.Media{ID: "m1", Title: "T2"}, nil)

	body := []byte(`{"sourceId":"src","url":"u","title":"T2"}`)
	c := newCtx("PUT", "/v1/media/m1", map[string]string{"id": "m1"}, body, "application/json")
	h.UpdateMedia(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestMediaHandler_DeleteMedia(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().DeleteMedia(mock.Anything, "m1").Return(nil)

	c := newCtx("DELETE", "/v1/media/m1", map[string]string{"id": "m1"}, nil, "")
	h.DeleteMedia(context.Background(), c)
	assert.Equal(t, consts.StatusNoContent, c.Response.StatusCode())
}

func TestMediaHandler_CreateChapter_UsesPathMediaID(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	// The path media id overrides any body mediaId.
	wantReq := domain.ChapterWriteRequest{MediaID: "m1", Name: "Ch 1", URL: "u", Number: 1}
	svc.EXPECT().CreateChapter(mock.Anything, wantReq).Return(domain.Chapter{ID: "c1", MediaID: "m1"}, nil)

	body := []byte(`{"mediaId":"ignored","name":"Ch 1","url":"u","number":1}`)
	c := newCtx("POST", "/v1/media/m1/chapters", map[string]string{"id": "m1"}, body, "application/json")
	h.CreateChapter(context.Background(), c)
	assert.Equal(t, consts.StatusCreated, c.Response.StatusCode())
}

func TestMediaHandler_UpdateChapter(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	wantReq := domain.ChapterWriteRequest{MediaID: "m1", Name: "Ch 2"}
	svc.EXPECT().UpdateChapter(mock.Anything, "c1", wantReq).Return(domain.Chapter{ID: "c1"}, nil)

	body := []byte(`{"mediaId":"m1","name":"Ch 2"}`)
	c := newCtx("PUT", "/v1/chapters/c1", map[string]string{"id": "c1"}, body, "application/json")
	h.UpdateChapter(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestMediaHandler_DeleteChapter(t *testing.T) {
	h, svc, _ := newMediaHandler(t)

	svc.EXPECT().DeleteChapter(mock.Anything, "c1").Return(nil)

	c := newCtx("DELETE", "/v1/chapters/c1", map[string]string{"id": "c1"}, nil, "")
	h.DeleteChapter(context.Background(), c)
	assert.Equal(t, consts.StatusNoContent, c.Response.StatusCode())
}

func TestMediaHandler_Image_MissingKey(t *testing.T) {
	h, _, _ := newMediaHandler(t)

	c := newCtx("GET", "/v1/image", nil, nil, "")
	h.Image(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestMediaHandler_Image_ServesBytes(t *testing.T) {
	h, _, store := newMediaHandler(t)

	store.EXPECT().Get(mock.Anything, "pages/a.avif").Return([]byte("AVIF-BYTES"), "", nil)

	c := newCtx("GET", "/v1/image?key=pages/a.avif", nil, nil, "")
	h.Image(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	assert.Equal(t, []byte("AVIF-BYTES"), c.Response.Body())
	assert.Contains(t, string(c.Response.Header.ContentType()), "image/avif")
	assert.NotEmpty(t, string(c.Response.Header.Peek("Cache-Control")))
}

func TestMediaHandler_Image_StoreError(t *testing.T) {
	h, _, store := newMediaHandler(t)

	store.EXPECT().Get(mock.Anything, "bad").Return(nil, "", domain.ErrNotFound)

	c := newCtx("GET", "/v1/image?key=bad", nil, nil, "")
	h.Image(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())
}
