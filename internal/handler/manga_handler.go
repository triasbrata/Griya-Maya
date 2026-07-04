package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/service"
)

// MangaHandler exposes the catalog/reader endpoints.
type MangaHandler struct {
	svc   *service.MangaService
	store service.ObjectStore
}

// NewMangaHandler wires a MangaHandler.
func NewMangaHandler(svc *service.MangaService, store service.ObjectStore) *MangaHandler {
	return &MangaHandler{svc: svc, store: store}
}

// Popular godoc
// @Summary  Popular manga for a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId     path  string   true  "Source ID"
// @Param    page         query int      false "Page (1-based)"
// @Param    sort         query string   false "Sort key" Enums(popular, latest, updated, rating, title)
// @Param    order        query string   false "Sort direction" Enums(asc, desc)
// @Param    type         query []string false "Content-type tag (repeatable): manga|manhwa|manhua"
// @Param    genre        query []string false "Include genre slug (repeatable)"
// @Param    genreExclude query []string false "Exclude genre slug (repeatable)"
// @Param    genreMode    query string   false "Combine included genres" Enums(or, and)
// @Success  200 {object} domain.MangaPage
// @Router   /v1/sources/{sourceId}/popular [get]
func (h *MangaHandler) Popular(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Popular(ctx, c.Param("sourceId"), queryInt(c, "page", 1), parseCatalogFilter(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, res)
}

// Latest godoc
// @Summary  Latest-updated manga for a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId     path  string   true  "Source ID"
// @Param    page         query int      false "Page (1-based)"
// @Param    sort         query string   false "Sort key" Enums(popular, latest, updated, rating, title)
// @Param    order        query string   false "Sort direction" Enums(asc, desc)
// @Param    type         query []string false "Content-type tag (repeatable): manga|manhwa|manhua"
// @Param    genre        query []string false "Include genre slug (repeatable)"
// @Param    genreExclude query []string false "Exclude genre slug (repeatable)"
// @Param    genreMode    query string   false "Combine included genres" Enums(or, and)
// @Success  200 {object} domain.MangaPage
// @Router   /v1/sources/{sourceId}/latest [get]
func (h *MangaHandler) Latest(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Latest(ctx, c.Param("sourceId"), queryInt(c, "page", 1), parseCatalogFilter(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, res)
}

// Search godoc
// @Summary  Search manga within a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId     path  string   true  "Source ID"
// @Param    q            query string   true  "Query"
// @Param    page         query int      false "Page (1-based)"
// @Param    sort         query string   false "Sort key" Enums(popular, latest, updated, rating, title)
// @Param    order        query string   false "Sort direction" Enums(asc, desc)
// @Param    type         query []string false "Content-type tag (repeatable): manga|manhwa|manhua"
// @Param    genre        query []string false "Include genre slug (repeatable)"
// @Param    genreExclude query []string false "Exclude genre slug (repeatable)"
// @Param    genreMode    query string   false "Combine included genres" Enums(or, and)
// @Success  200 {object} domain.MangaPage
// @Router   /v1/sources/{sourceId}/search [get]
func (h *MangaHandler) Search(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Search(ctx, c.Param("sourceId"), c.Query("q"), queryInt(c, "page", 1), parseCatalogFilter(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, res)
}

// Genres godoc
// @Summary  Filterable genres for a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId path string true "Source ID"
// @Success  200 {array} domain.GenreTag
// @Router   /v1/sources/{sourceId}/genres [get]
func (h *MangaHandler) Genres(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Genres(ctx, c.Param("sourceId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, res)
}

// Details godoc
// @Summary  Manga details
// @Tags     catalog
// @Produce  json
// @Param    id path string true "Manga ID"
// @Success  200 {object} domain.Manga
// @Failure  404 {object} handler.ErrorResponse
// @Router   /v1/manga/{id} [get]
func (h *MangaHandler) Details(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Details(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, res)
}

// Chapters godoc
// @Summary  Chapter list for a manga
// @Tags     catalog
// @Produce  json
// @Param    id path string true "Manga ID"
// @Success  200 {array} domain.Chapter
// @Router   /v1/manga/{id}/chapters [get]
func (h *MangaHandler) Chapters(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Chapters(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, res)
}

// Pages godoc
// @Summary  Page list (AVIF image URLs) for a chapter
// @Tags     reader
// @Produce  json
// @Param    id path string true "Chapter ID"
// @Success  200 {array} domain.Page
// @Router   /v1/chapters/{id}/pages [get]
func (h *MangaHandler) Pages(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Pages(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, res)
}

// Image godoc
// @Summary  Proxy an AVIF page object from R2 (used when no public R2 domain).
// @Tags     reader
// @Produce  image/avif
// @Param    key query string true "R2 object key"
// @Success  200 {file} binary
// @Router   /v1/image [get]
func (h *MangaHandler) Image(ctx context.Context, c *app.RequestContext) {
	key := c.Query("key")
	if key == "" {
		c.JSON(consts.StatusBadRequest, ErrorResponse{Error: "invalid_input", Message: "key is required"})
		return
	}
	data, contentType, err := h.store.Get(ctx, key)
	if err != nil {
		writeError(c, err)
		return
	}
	if contentType == "" {
		contentType = "image/avif"
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Data(consts.StatusOK, contentType, data)
}
