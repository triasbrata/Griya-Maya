package handler

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// SubTypeHandler serves the admin CRUD for the managed sub-type vocabulary
// (`sub_type` table). The reader/client discovery endpoint (GET /v1/subtypes)
// lives on MediaHandler.SubTypeCatalog.
type SubTypeHandler struct {
	svc SubTypeService
}

// NewSubTypeHandler builds the handler from a sub-type service.
func NewSubTypeHandler(svc SubTypeService) *SubTypeHandler {
	return &SubTypeHandler{svc: svc}
}

// List returns every managed sub-type (flat, each carrying its owning type).
//
// @Summary     List all sub-types (admin)
// @Tags        subtypes
// @Produce     json
// @Success     200 {array} domain.SubType
// @Failure     401 {object} handler.ErrorResponse
// @Failure     403 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/subtypes [get]
func (h *SubTypeHandler) List(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.List(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Create adds a new sub-type.
//
// @Summary     Create a sub-type (admin)
// @Tags        subtypes
// @Accept      json
// @Produce     json
// @Param       request body domain.SubTypeWriteRequest true "New sub-type"
// @Success     201 {object} domain.SubType
// @Failure     400 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/subtypes [post]
func (h *SubTypeHandler) Create(ctx context.Context, c *app.RequestContext) {
	var req domain.SubTypeWriteRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	st, err := h.svc.Create(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusCreated, st)
}

// Update edits an existing sub-type (slug is immutable).
//
// @Summary     Update a sub-type (admin)
// @Tags        subtypes
// @Accept      json
// @Produce     json
// @Param       slug path string true "Sub-type slug"
// @Param       request body domain.SubTypeWriteRequest true "Updated fields"
// @Success     200 {object} domain.SubType
// @Failure     400 {object} handler.ErrorResponse
// @Failure     404 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/subtypes/{slug} [put]
func (h *SubTypeHandler) Update(ctx context.Context, c *app.RequestContext) {
	var req domain.SubTypeWriteRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	st, err := h.svc.Update(ctx, c.Param("slug"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, st)
}

// Delete removes a sub-type.
//
// @Summary     Delete a sub-type (admin)
// @Tags        subtypes
// @Produce     json
// @Param       slug path string true "Sub-type slug"
// @Success     204 "No Content"
// @Failure     400 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/subtypes/{slug} [delete]
func (h *SubTypeHandler) Delete(ctx context.Context, c *app.RequestContext) {
	if err := h.svc.Delete(ctx, c.Param("slug")); err != nil {
		writeError(c, err)
		return
	}
	c.SetStatusCode(consts.StatusNoContent)
}
