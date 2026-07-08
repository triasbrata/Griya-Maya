package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// TaxonomyHandler manages the three normalized taxonomies through a single
// parametric resource: /v1/taxonomies/{kind}, where kind is one of
// genres | authors | artists.
type TaxonomyHandler struct {
	svc TaxonomyService
}

// NewTaxonomyHandler wires a TaxonomyHandler.
func NewTaxonomyHandler(svc TaxonomyService) *TaxonomyHandler {
	return &TaxonomyHandler{svc: svc}
}

// kindFromPath maps the plural URL segment to a domain.TaxonomyKind.
func kindFromPath(seg string) (domain.TaxonomyKind, bool) {
	switch seg {
	case "genres":
		return domain.TaxonomyGenre, true
	case "authors":
		return domain.TaxonomyAuthor, true
	case "artists":
		return domain.TaxonomyArtist, true
	}
	return "", false
}

func (h *TaxonomyHandler) kind(c *app.RequestContext) (domain.TaxonomyKind, bool) {
	kind, ok := kindFromPath(c.Param("kind"))
	if !ok {
		writeErr(c, consts.StatusNotFound, "not_found", "unknown taxonomy kind")
	}
	return kind, ok
}

// List godoc
// @Summary  List all tags of a taxonomy.
// @Tags     taxonomy
// @Produce  json
// @Param    kind path string true "Taxonomy" Enums(genres, authors, artists)
// @Success  200 {array} domain.Taxonomy
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/taxonomies/{kind} [get]
func (h *TaxonomyHandler) List(ctx context.Context, c *app.RequestContext) {
	kind, ok := h.kind(c)
	if !ok {
		return
	}
	res, err := h.svc.List(ctx, kind)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Create godoc
// @Summary  Create a taxonomy tag.
// @Tags     taxonomy
// @Accept   json
// @Produce  json
// @Param    kind    path string                     true "Taxonomy" Enums(genres, authors, artists)
// @Param    request body domain.TaxonomyWriteRequest true "Tag"
// @Success  201 {object} domain.Taxonomy
// @Failure  400 {object} handler.ErrorResponse
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/taxonomies/{kind} [post]
func (h *TaxonomyHandler) Create(ctx context.Context, c *app.RequestContext) {
	kind, ok := h.kind(c)
	if !ok {
		return
	}
	var req domain.TaxonomyWriteRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.Create(ctx, kind, req.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusCreated, res)
}

// Update godoc
// @Summary  Rename a taxonomy tag.
// @Tags     taxonomy
// @Accept   json
// @Produce  json
// @Param    kind    path string                     true "Taxonomy" Enums(genres, authors, artists)
// @Param    id      path string                     true "Tag ID"
// @Param    request body domain.TaxonomyWriteRequest true "Tag"
// @Success  200 {object} domain.Taxonomy
// @Failure  400 {object} handler.ErrorResponse
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/taxonomies/{kind}/{id} [put]
func (h *TaxonomyHandler) Update(ctx context.Context, c *app.RequestContext) {
	kind, ok := h.kind(c)
	if !ok {
		return
	}
	var req domain.TaxonomyWriteRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.Update(ctx, kind, c.Param("id"), req.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Delete godoc
// @Summary  Delete a taxonomy tag (and its media links).
// @Tags     taxonomy
// @Param    kind path string true "Taxonomy" Enums(genres, authors, artists)
// @Param    id   path string true "Tag ID"
// @Success  204
// @Failure  400 {object} handler.ErrorResponse
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/taxonomies/{kind}/{id} [delete]
func (h *TaxonomyHandler) Delete(ctx context.Context, c *app.RequestContext) {
	kind, ok := h.kind(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx, kind, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(consts.StatusNoContent)
}
