package handler

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// SourceHandler serves the reader-facing source listing and the admin
// source-management CRUD.
type SourceHandler struct {
	svc SourceService
}

// NewSourceHandler builds the handler from a source service.
func NewSourceHandler(svc SourceService) *SourceHandler {
	return &SourceHandler{svc: svc}
}

// List returns the enabled sources for the reader.
//
// @Summary     List enabled sources
// @Description Sources the reader can browse. Gated by manga.read.
// @Tags        catalog
// @Produce     json
// @Success     200 {array} domain.Source
// @Failure     401 {object} handler.ErrorResponse
// @Failure     403 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/sources [get]
func (h *SourceHandler) List(ctx context.Context, c *app.RequestContext) {
	sources, err := h.svc.List(ctx, true)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, sources)
}

// AdminList returns every source (including disabled) for management.
//
// @Summary     List all sources (admin)
// @Tags        sources
// @Produce     json
// @Success     200 {array} domain.Source
// @Failure     401 {object} handler.ErrorResponse
// @Failure     403 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/sources [get]
func (h *SourceHandler) AdminList(ctx context.Context, c *app.RequestContext) {
	sources, err := h.svc.List(ctx, false)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, sources)
}

// Get returns a single source by id.
//
// @Summary     Get a source (admin)
// @Tags        sources
// @Produce     json
// @Param       id path string true "Source ID"
// @Success     200 {object} domain.Source
// @Failure     404 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/sources/{id} [get]
func (h *SourceHandler) Get(ctx context.Context, c *app.RequestContext) {
	src, err := h.svc.Get(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, src)
}

// Create adds a new source.
//
// @Summary     Create a source (admin)
// @Tags        sources
// @Accept      json
// @Produce     json
// @Param       request body domain.SourceWriteRequest true "New source"
// @Success     201 {object} domain.Source
// @Failure     400 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/sources [post]
func (h *SourceHandler) Create(ctx context.Context, c *app.RequestContext) {
	var req domain.SourceWriteRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	src, err := h.svc.Create(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusCreated, src)
}

// Update edits an existing source (id is immutable).
//
// @Summary     Update a source (admin)
// @Tags        sources
// @Accept      json
// @Produce     json
// @Param       id path string true "Source ID"
// @Param       request body domain.SourceWriteRequest true "Updated fields"
// @Success     200 {object} domain.Source
// @Failure     400 {object} handler.ErrorResponse
// @Failure     404 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/sources/{id} [put]
func (h *SourceHandler) Update(ctx context.Context, c *app.RequestContext) {
	var req domain.SourceWriteRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	src, err := h.svc.Update(ctx, c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, src)
}

// Delete removes a source (refused when it still has media).
//
// @Summary     Delete a source (admin)
// @Tags        sources
// @Produce     json
// @Param       id path string true "Source ID"
// @Success     204 "No Content"
// @Failure     400 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/sources/{id} [delete]
func (h *SourceHandler) Delete(ctx context.Context, c *app.RequestContext) {
	if err := h.svc.Delete(ctx, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.SetStatusCode(consts.StatusNoContent)
}
