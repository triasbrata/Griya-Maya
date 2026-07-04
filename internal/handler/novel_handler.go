package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// NovelHandler exposes the text-chapter registration endpoint. Novel text is
// served inline through the normal /v1/chapters/{id}/pages endpoint, so there is
// no dedicated read route here.
type NovelHandler struct {
	svc NovelService
}

// NewNovelHandler wires a NovelHandler.
func NewNovelHandler(svc NovelService) *NovelHandler {
	return &NovelHandler{svc: svc}
}

// Register godoc
// @Summary  Register a text chapter (stored as .txt in R2) as a chapter's novel page.
// @Description Provide inline `text` (the server stores it as a .txt in R2) or `textKey` of an already-uploaded object. The chapter then serves as a single novel page whose `body` carries the text.
// @Tags     novel
// @Accept   json
// @Produce  json
// @Param    request body domain.NovelRegisterRequest true "Novel registration"
// @Success  200 {object} domain.Page
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/novel [post]
func (h *NovelHandler) Register(ctx context.Context, c *app.RequestContext) {
	var req domain.NovelRegisterRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ErrorResponse{Error: "invalid_input", Message: err.Error()})
		return
	}
	page, err := h.svc.Register(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, page)
}
