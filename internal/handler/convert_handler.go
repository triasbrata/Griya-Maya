package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// ConvertHandler exposes the browser-side ingest endpoints: mint presigned R2
// upload URLs and register the resulting AVIF pages onto a chapter.
type ConvertHandler struct {
	svc ConvertService
}

// NewConvertHandler wires a ConvertHandler.
func NewConvertHandler(svc ConvertService) *ConvertHandler {
	return &ConvertHandler{svc: svc}
}

// PresignRequest asks for a batch of direct-upload URLs.
type PresignRequest struct {
	// Count is how many page upload URLs to mint (1..5000).
	Count int `json:"count"`
	// ContentType binds the required Content-Type of each PUT (default image/avif).
	ContentType string `json:"contentType,omitempty"`
}

// Presign godoc
// @Summary  Mint a batch of presigned R2 PUT URLs for browser-encoded AVIF pages.
// @Tags     convert
// @Accept   json
// @Produce  json
// @Param    request body handler.PresignRequest true "How many upload URLs to mint"
// @Success  200 {object} service.PresignResult
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/convert/presign [post]
func (h *ConvertHandler) Presign(ctx context.Context, c *app.RequestContext) {
	var req PresignRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.PresignUploads(ctx, req.Count, req.ContentType)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// RegisterPagesRequest attaches already-uploaded AVIF pages to a chapter.
type RegisterPagesRequest struct {
	Pages []RegisterPageItem `json:"pages"`
}

// RegisterPageItem is one uploaded page: its R2 object key plus dimensions.
type RegisterPageItem struct {
	Index  int    `json:"index"`
	R2Key  string `json:"r2Key"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// RegisterPages godoc
// @Summary  Register browser-uploaded AVIF pages as a chapter's page rows.
// @Tags     convert
// @Accept   json
// @Produce  json
// @Param    id      path string                       true "Chapter ID"
// @Param    request body handler.RegisterPagesRequest true "Uploaded pages"
// @Success  200 {array} domain.Page
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/chapters/{id}/pages [post]
func (h *ConvertHandler) RegisterPages(ctx context.Context, c *app.RequestContext) {
	var req RegisterPagesRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	pages := make([]domain.StoredPage, 0, len(req.Pages))
	for _, p := range req.Pages {
		pages = append(pages, domain.StoredPage{
			Index:  p.Index,
			R2Key:  p.R2Key,
			Width:  p.Width,
			Height: p.Height,
		})
	}
	out, err := h.svc.RegisterPages(ctx, c.Param("id"), pages)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, out)
}
