package handler

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// AdHandler serves the reader-facing house-ad listing and the admin ad-management
// CRUD (including presigned creative upload).
type AdHandler struct {
	svc AdService
}

// NewAdHandler builds the handler from an ad service.
func NewAdHandler(svc AdService) *AdHandler {
	return &AdHandler{svc: svc}
}

// List returns the active house ads for the reader, each with a presigned image
// URL. Filter by ?placement=<key> (e.g. reader_interstitial); omit for all.
//
// @Summary     List active house ads
// @Description Active house-ad creatives the reader interleaves between pages. Each imageUrl is a short-lived presigned R2 URL. Gated by manga.read.
// @Tags        ads
// @Produce     json
// @Param       placement query string false "Placement key (e.g. reader_interstitial)"
// @Success     200 {array} domain.Ad
// @Failure     401 {object} handler.ErrorResponse
// @Failure     403 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/ads [get]
func (h *AdHandler) List(ctx context.Context, c *app.RequestContext) {
	ads, err := h.svc.ListActive(ctx, c.Query("placement"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, ads)
}

// AdminList returns every ad (active and inactive) for management.
//
// @Summary     List all house ads (admin)
// @Tags        ads
// @Produce     json
// @Success     200 {array} domain.StoredAd
// @Failure     401 {object} handler.ErrorResponse
// @Failure     403 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/ads [get]
func (h *AdHandler) AdminList(ctx context.Context, c *app.RequestContext) {
	ads, err := h.svc.List(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, ads)
}

// Get returns a single ad by id.
//
// @Summary     Get a house ad (admin)
// @Tags        ads
// @Produce     json
// @Param       id path string true "Ad ID"
// @Success     200 {object} domain.StoredAd
// @Failure     404 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/ads/{id} [get]
func (h *AdHandler) Get(ctx context.Context, c *app.RequestContext) {
	ad, err := h.svc.Get(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, ad)
}

// Create adds a new ad from an already-uploaded creative (r2Key).
//
// @Summary     Create a house ad (admin)
// @Tags        ads
// @Accept      json
// @Produce     json
// @Param       request body domain.AdWriteRequest true "New ad"
// @Success     201 {object} domain.StoredAd
// @Failure     400 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/ads [post]
func (h *AdHandler) Create(ctx context.Context, c *app.RequestContext) {
	var req domain.AdWriteRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	ad, err := h.svc.Create(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusCreated, ad)
}

// Update edits an existing ad (id is immutable).
//
// @Summary     Update a house ad (admin)
// @Tags        ads
// @Accept      json
// @Produce     json
// @Param       id path string true "Ad ID"
// @Param       request body domain.AdWriteRequest true "Updated fields"
// @Success     200 {object} domain.StoredAd
// @Failure     400 {object} handler.ErrorResponse
// @Failure     404 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/ads/{id} [put]
func (h *AdHandler) Update(ctx context.Context, c *app.RequestContext) {
	var req domain.AdWriteRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	ad, err := h.svc.Update(ctx, c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, ad)
}

// Delete removes an ad by id.
//
// @Summary     Delete a house ad (admin)
// @Tags        ads
// @Produce     json
// @Param       id path string true "Ad ID"
// @Success     204 "No Content"
// @Failure     400 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/ads/{id} [delete]
func (h *AdHandler) Delete(ctx context.Context, c *app.RequestContext) {
	if err := h.svc.Delete(ctx, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.SetStatusCode(consts.StatusNoContent)
}

// AdPresignRequest asks for a single presigned creative-upload URL.
type AdPresignRequest struct {
	// ContentType binds the required Content-Type of the PUT (default image/avif).
	ContentType string `json:"contentType,omitempty"`
}

// Presign mints one presigned R2 PUT URL so the admin browser uploads a creative
// straight to R2, then sends the returned key back as r2Key on create/update.
//
// @Summary     Mint a presigned creative-upload URL (admin)
// @Tags        ads
// @Accept      json
// @Produce     json
// @Param       request body handler.AdPresignRequest false "Upload content type"
// @Success     200 {object} service.PresignItem
// @Failure     400 {object} handler.ErrorResponse
// @Security    BearerAuth
// @Router      /v1/admin/ads/presign [post]
func (h *AdHandler) Presign(ctx context.Context, c *app.RequestContext) {
	var req AdPresignRequest
	if len(c.Request.Body()) > 0 {
		if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
			writeErr(c, consts.StatusBadRequest, "invalid_input", "malformed JSON body")
			return
		}
	}
	item, err := h.svc.PresignUpload(ctx, req.ContentType)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, item)
}
