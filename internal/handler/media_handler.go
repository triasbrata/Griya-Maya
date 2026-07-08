package handler

import (
	"context"
	"strconv"
	"strings"

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
// @Param    subType         query []string false "Filter by sub-type slug (repeatable): e.g. manga|manhwa|manhua"
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
// @Param    subType         query []string false "Filter by sub-type slug (repeatable): e.g. manga|manhwa|manhua"
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
// @Param    subType         query []string false "Filter by sub-type slug (repeatable): e.g. manga|manhwa|manhua"
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

// Recommendations godoc
// @Summary  Content-based recommendations for a source
// @Description Ranks a source's catalog by shared sub-type with the supplied subTypes (aggregated client-side from recent reading; history stays on the client), tie-broken by the popular order. Media whose sub-type is not requested, and any id in exclude, are omitted. With no subTypes it falls back to the source's popular feed.
// @Tags     catalog
// @Produce  json
// @Param    sourceId path  string true  "Source ID"
// @Param    subTypes query string false "Comma-separated sub-type slugs to match"
// @Param    exclude  query string false "Comma-separated media ids to omit (already-read / seed)"
// @Param    page     query int    false "Page (1-based)"
// @Success  200 {object} domain.MediaPage
// @Router   /v1/sources/{sourceId}/recommendations [get]
func (h *MediaHandler) Recommendations(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Recommendations(ctx, c.Param("sourceId"), queryAll(c, "subTypes"), queryAll(c, "exclude"), queryInt(c, "page", 1))
	if err != nil {
		writeError(c, err)
		return
	}
	writePagination(c, domain.OffsetPagination(res.Page, domain.CatalogPageSize, -1, res.HasNext))
	writeOK(c, consts.StatusOK, res)
}

// SubTypes godoc
// @Summary  Filterable sub-types present in a source
// @Tags     catalog
// @Produce  json
// @Param    sourceId path string true "Source ID"
// @Success  200 {array} domain.SubType
// @Router   /v1/sources/{sourceId}/subtypes [get]
func (h *MediaHandler) SubTypes(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.SubTypes(ctx, c.Param("sourceId"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// SubTypeCatalog godoc
// @Summary  Sub-type vocabulary grouped by media type
// @Description Returns the managed set of sub-types allowed per media type (manga|novel|video) — the source of truth for populating a sub-type selector. Backed by the `sub_type` table.
// @Tags     catalog
// @Produce  json
// @Success  200 {object} map[string][]domain.SubType
// @Router   /v1/subtypes [get]
func (h *MediaHandler) SubTypeCatalog(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.SubTypeCatalog(ctx)
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

// ChapterNeighbors godoc
// @Summary  Previous/next chapter around a chapter
// @Description Returns the chapters immediately before and after this one within its media (by chapter number); either side is null at the ends. Public read (metadata only, no page bytes) — matches the chapters-list gate.
// @Tags     catalog
// @Produce  json
// @Param    id path string true "Chapter ID"
// @Success  200 {object} domain.ChapterNeighbors
// @Failure  404 {object} handler.ErrorResponse
// @Router   /v1/chapters/{id}/adjacent [get]
func (h *MediaHandler) ChapterNeighbors(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.ChapterNeighbors(ctx, c.Param("id"))
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

// deleteChaptersRequest is the batch-delete payload: a list of chapter ids (one
// entry is a single delete). Unknown ids are ignored (idempotent).
type deleteChaptersRequest struct {
	IDs []string `json:"ids"`
}

// DeleteChapters godoc
// @Summary  Delete one or more chapters (and their pages) in one call.
// @Description Batch-capable chapter delete. Accepts an array of ids of any length (a single-element array is a single delete). Deleted pages' R2 artifacts are cleaned up asynchronously. Unknown ids are ignored.
// @Tags     media
// @Accept   json
// @Param    request body handler.deleteChaptersRequest true "Chapter ids"
// @Success  204
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/chapters/delete [post]
func (h *MediaHandler) DeleteChapters(ctx context.Context, c *app.RequestContext) {
	var req deleteChaptersRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	ids := make([]string, 0, len(req.IDs))
	for _, id := range req.IDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "ids is required")
		return
	}
	if err := h.svc.DeleteChapters(ctx, ids); err != nil {
		writeError(c, err)
		return
	}
	c.Status(consts.StatusNoContent)
}

// AdminChapterPages godoc
// @Summary  List a chapter's pages with raw R2 keys (admin).
// @Description Admin-only page listing that exposes each page's raw r2Key alongside a short-lived presigned imageUrl, so an operator can inspect and delete individual artifacts. Requires admin.read.
// @Tags     admin
// @Produce  json
// @Param    id path string true "Chapter ID"
// @Success  200 {array} domain.AdminPage
// @Failure  401 {object} handler.ErrorResponse
// @Failure  403 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/admin/chapters/{id}/pages [get]
func (h *MediaHandler) AdminChapterPages(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.ChapterPagesAdmin(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// AdminDeleteChapterPage godoc
// @Summary  Delete a single chapter page and its artifact (admin).
// @Description Removes one page (by index) from a chapter and schedules its R2 object for async cleanup. Requires admin.write.
// @Tags     admin
// @Param    id  path string true "Chapter ID"
// @Param    idx path int    true "Page index"
// @Success  204
// @Failure  400 {object} handler.ErrorResponse
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/admin/chapters/{id}/pages/{idx} [delete]
func (h *MediaHandler) AdminDeleteChapterPage(ctx context.Context, c *app.RequestContext) {
	idx, err := strconv.Atoi(c.Param("idx"))
	if err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "idx must be an integer")
		return
	}
	if err := h.svc.DeleteChapterPage(ctx, c.Param("id"), idx); err != nil {
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
