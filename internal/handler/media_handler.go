package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
)

// MediaHandler exposes the catalog/reader endpoints and media/chapter management.
type MediaHandler struct {
	svc   MediaService
	store service.ObjectStore
}

// NewMediaHandler wires a MediaHandler.
func NewMediaHandler(svc MediaService, store service.ObjectStore) *MediaHandler {
	return &MediaHandler{svc: svc, store: store}
}

// Popular godoc
// @Summary  Popular media for a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId        path  string   true  "Source ID"
// @Param    page            query int      false "Page (1-based)"
// @Param    sort            query string   false "Sort key" Enums(popular, latest, updated, rating, title)
// @Param    order           query string   false "Sort direction" Enums(asc, desc)
// @Param    type            query []string false "Media type (repeatable): manga|video|novel"
// @Param    genre           query []string false "Include genre slug (repeatable)"
// @Param    genreExclude    query []string false "Exclude genre slug (repeatable)"
// @Param    category        query []string false "Include category slug (repeatable)"
// @Param    categoryExclude query []string false "Exclude category slug (repeatable)"
// @Param    genreMode       query string   false "Combine included genres/categories" Enums(or, and)
// @Success  200 {object} domain.MediaPage
// @Router   /v1/sources/{sourceId}/popular [get]
func (h *MediaHandler) Popular(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Popular(ctx, c.Param("sourceId"), queryInt(c, "page", 1), parseCatalogFilter(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writePagination(c, domain.OffsetPagination(res.Page, domain.CatalogPageSize, -1, res.HasNext))
	writeOK(c, consts.StatusOK, res)
}

// Latest godoc
// @Summary  Latest-updated media for a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId        path  string   true  "Source ID"
// @Param    page            query int      false "Page (1-based)"
// @Param    sort            query string   false "Sort key" Enums(popular, latest, updated, rating, title)
// @Param    order           query string   false "Sort direction" Enums(asc, desc)
// @Param    type            query []string false "Media type (repeatable): manga|video|novel"
// @Param    genre           query []string false "Include genre slug (repeatable)"
// @Param    genreExclude    query []string false "Exclude genre slug (repeatable)"
// @Param    category        query []string false "Include category slug (repeatable)"
// @Param    categoryExclude query []string false "Exclude category slug (repeatable)"
// @Param    genreMode       query string   false "Combine included genres/categories" Enums(or, and)
// @Success  200 {object} domain.MediaPage
// @Router   /v1/sources/{sourceId}/latest [get]
func (h *MediaHandler) Latest(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Latest(ctx, c.Param("sourceId"), queryInt(c, "page", 1), parseCatalogFilter(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writePagination(c, domain.OffsetPagination(res.Page, domain.CatalogPageSize, -1, res.HasNext))
	writeOK(c, consts.StatusOK, res)
}

// Search godoc
// @Summary  Search media within a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId        path  string   true  "Source ID"
// @Param    q               query string   true  "Query"
// @Param    page            query int      false "Page (1-based)"
// @Param    sort            query string   false "Sort key" Enums(popular, latest, updated, rating, title)
// @Param    order           query string   false "Sort direction" Enums(asc, desc)
// @Param    type            query []string false "Media type (repeatable): manga|video|novel"
// @Param    genre           query []string false "Include genre slug (repeatable)"
// @Param    genreExclude    query []string false "Exclude genre slug (repeatable)"
// @Param    category        query []string false "Include category slug (repeatable)"
// @Param    categoryExclude query []string false "Exclude category slug (repeatable)"
// @Param    genreMode       query string   false "Combine included genres/categories" Enums(or, and)
// @Success  200 {object} domain.MediaPage
// @Router   /v1/sources/{sourceId}/search [get]
func (h *MediaHandler) Search(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Search(ctx, c.Param("sourceId"), c.Query("q"), queryInt(c, "page", 1), parseCatalogFilter(c))
	if err != nil {
		writeError(c, err)
		return
	}
	writePagination(c, domain.OffsetPagination(res.Page, domain.CatalogPageSize, -1, res.HasNext))
	writeOK(c, consts.StatusOK, res)
}

// Genres godoc
// @Summary  Filterable genres for a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId path string true "Source ID"
// @Success  200 {array} domain.Taxonomy
// @Router   /v1/sources/{sourceId}/genres [get]
func (h *MediaHandler) Genres(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Genres(ctx, c.Param("sourceId"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Categories godoc
// @Summary  Filterable categories for a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId path string true "Source ID"
// @Success  200 {array} domain.Taxonomy
// @Router   /v1/sources/{sourceId}/categories [get]
func (h *MediaHandler) Categories(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Categories(ctx, c.Param("sourceId"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Details godoc
// @Summary  Media details
// @Tags     catalog
// @Produce  json
// @Param    id path string true "Media ID"
// @Success  200 {object} domain.Media
// @Failure  404 {object} handler.ErrorResponse
// @Router   /v1/media/{id} [get]
func (h *MediaHandler) Details(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Details(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Chapters godoc
// @Summary  Chapter list for a media entry
// @Tags     catalog
// @Produce  json
// @Param    id path string true "Media ID"
// @Success  200 {array} domain.Chapter
// @Router   /v1/media/{id}/chapters [get]
func (h *MediaHandler) Chapters(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Chapters(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Pages godoc
// @Summary  Page list (presigned URLs) for a chapter
// @Description Returns short-lived presigned R2 URLs the client fetches directly. Requires a manga.read access token.
// @Tags     reader
// @Produce  json
// @Param    id path string true "Chapter ID"
// @Success  200 {array} domain.Page
// @Failure  401 {object} handler.ErrorResponse
// @Failure  403 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/chapters/{id}/pages [get]
func (h *MediaHandler) Pages(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Pages(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// CreateMedia godoc
// @Summary  Create a media entry (manga | video | novel).
// @Tags     media
// @Accept   json
// @Produce  json
// @Param    request body domain.MediaWriteRequest true "Media"
// @Success  201 {object} domain.Media
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/media [post]
func (h *MediaHandler) CreateMedia(ctx context.Context, c *app.RequestContext) {
	var req domain.MediaWriteRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.CreateMedia(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusCreated, res)
}

// UpdateMedia godoc
// @Summary  Update a media entry.
// @Tags     media
// @Accept   json
// @Produce  json
// @Param    id      path string                  true "Media ID"
// @Param    request body domain.MediaWriteRequest true "Media"
// @Success  200 {object} domain.Media
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/media/{id} [put]
func (h *MediaHandler) UpdateMedia(ctx context.Context, c *app.RequestContext) {
	var req domain.MediaWriteRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.UpdateMedia(ctx, c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// DeleteMedia godoc
// @Summary  Delete a media entry (cascades chapters/pages/links).
// @Tags     media
// @Param    id path string true "Media ID"
// @Success  204
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/media/{id} [delete]
func (h *MediaHandler) DeleteMedia(ctx context.Context, c *app.RequestContext) {
	if err := h.svc.DeleteMedia(ctx, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(consts.StatusNoContent)
}

// CreateChapter godoc
// @Summary  Create a chapter under a media entry.
// @Tags     media
// @Accept   json
// @Produce  json
// @Param    id      path string                    true "Media ID"
// @Param    request body domain.ChapterWriteRequest true "Chapter"
// @Success  201 {object} domain.Chapter
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/media/{id}/chapters [post]
func (h *MediaHandler) CreateChapter(ctx context.Context, c *app.RequestContext) {
	var req domain.ChapterWriteRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	// The path media id is authoritative.
	req.MediaID = c.Param("id")
	res, err := h.svc.CreateChapter(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusCreated, res)
}

// UpdateChapter godoc
// @Summary  Update a chapter.
// @Tags     media
// @Accept   json
// @Produce  json
// @Param    id      path string                    true "Chapter ID"
// @Param    request body domain.ChapterWriteRequest true "Chapter"
// @Success  200 {object} domain.Chapter
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/chapters/{id} [put]
func (h *MediaHandler) UpdateChapter(ctx context.Context, c *app.RequestContext) {
	var req domain.ChapterWriteRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.UpdateChapter(ctx, c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// DeleteChapter godoc
// @Summary  Delete a chapter (and its pages).
// @Tags     media
// @Param    id path string true "Chapter ID"
// @Success  204
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/chapters/{id} [delete]
func (h *MediaHandler) DeleteChapter(ctx context.Context, c *app.RequestContext) {
	if err := h.svc.DeleteChapter(ctx, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(consts.StatusNoContent)
}

// Image godoc
// @Summary  Proxy an AVIF page object from R2 (retired; gated fallback).
// @Description Legacy proxy kept behind the manga.read gate so the bucket stays private. Prefer the presigned URLs from /v1/chapters/{id}/pages.
// @Tags     reader
// @Produce  image/avif
// @Param    key query string true "R2 object key"
// @Success  200 {file} binary
// @Failure  401 {object} handler.ErrorResponse
// @Failure  403 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/image [get]
func (h *MediaHandler) Image(ctx context.Context, c *app.RequestContext) {
	key := c.Query("key")
	if key == "" {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "key is required")
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
