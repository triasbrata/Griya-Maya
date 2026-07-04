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

func newTaxonomyHandler(t *testing.T) (*handler.TaxonomyHandler, *mocks.MockTaxonomyService) {
	t.Helper()
	svc := mocks.NewMockTaxonomyService(t)
	return handler.NewTaxonomyHandler(svc), svc
}

func TestTaxonomyHandler_List(t *testing.T) {
	h, svc := newTaxonomyHandler(t)

	svc.EXPECT().List(mock.Anything, domain.TaxonomyGenre).
		Return([]domain.Taxonomy{{ID: "g1", Slug: "action", Name: "Action"}}, nil)

	c := newCtx("GET", "/v1/taxonomies/genres", map[string]string{"kind": "genres"}, nil, "")
	h.List(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var got []domain.Taxonomy
	decodeJSON(t, c, &got)
	require.Len(t, got, 1)
	assert.Equal(t, "Action", got[0].Name)
}

func TestTaxonomyHandler_List_UnknownKind(t *testing.T) {
	h, _ := newTaxonomyHandler(t)

	c := newCtx("GET", "/v1/taxonomies/bogus", map[string]string{"kind": "bogus"}, nil, "")
	h.List(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())
}

func TestTaxonomyHandler_Create(t *testing.T) {
	h, svc := newTaxonomyHandler(t)

	svc.EXPECT().Create(mock.Anything, domain.TaxonomyAuthor, "Oda").
		Return(domain.Taxonomy{ID: "a1", Name: "Oda"}, nil)

	body := []byte(`{"name":"Oda"}`)
	c := newCtx("POST", "/v1/taxonomies/authors", map[string]string{"kind": "authors"}, body, "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, consts.StatusCreated, c.Response.StatusCode())
}

func TestTaxonomyHandler_Create_BadInput(t *testing.T) {
	h, svc := newTaxonomyHandler(t)

	svc.EXPECT().Create(mock.Anything, domain.TaxonomyGenre, "").
		Return(domain.Taxonomy{}, domain.ErrInvalidInput)

	body := []byte(`{"name":""}`)
	c := newCtx("POST", "/v1/taxonomies/genres", map[string]string{"kind": "genres"}, body, "application/json")
	h.Create(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestTaxonomyHandler_Update(t *testing.T) {
	h, svc := newTaxonomyHandler(t)

	svc.EXPECT().Update(mock.Anything, domain.TaxonomyCategory, "c1", "Webtoon").
		Return(domain.Taxonomy{ID: "c1", Slug: "webtoon", Name: "Webtoon"}, nil)

	body := []byte(`{"name":"Webtoon"}`)
	c := newCtx("PUT", "/v1/taxonomies/categories/c1",
		map[string]string{"kind": "categories", "id": "c1"}, body, "application/json")
	h.Update(context.Background(), c)
	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
}

func TestTaxonomyHandler_Delete(t *testing.T) {
	h, svc := newTaxonomyHandler(t)

	svc.EXPECT().Delete(mock.Anything, domain.TaxonomyArtist, "x1").Return(nil)

	c := newCtx("DELETE", "/v1/taxonomies/artists/x1",
		map[string]string{"kind": "artists", "id": "x1"}, nil, "")
	h.Delete(context.Background(), c)
	assert.Equal(t, consts.StatusNoContent, c.Response.StatusCode())
}
