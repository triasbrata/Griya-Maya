package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
)

// ConvertHandler exposes upload + conversion endpoints.
type ConvertHandler struct {
	svc   *service.ConvertService
	store service.ObjectStore
}

// NewConvertHandler wires a ConvertHandler.
func NewConvertHandler(svc *service.ConvertService, store service.ObjectStore) *ConvertHandler {
	return &ConvertHandler{svc: svc, store: store}
}

// UploadResponse is returned after storing a raw archive.
type UploadResponse struct {
	SourceKey string `json:"sourceKey"`
}

// Upload godoc
// @Summary  Upload a CBZ/EPUB/PDF archive into R2 (multipart field "file").
// @Tags     convert
// @Accept   multipart/form-data
// @Produce  json
// @Param    file formData file   true  "Archive file"
// @Param    key  formData string false "Target R2 key (defaults to uploads/{uuid})"
// @Success  201 {object} handler.UploadResponse
// @Security BearerAuth
// @Router   /v1/convert/upload [post]
func (h *ConvertHandler) Upload(ctx context.Context, c *app.RequestContext) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(consts.StatusBadRequest, ErrorResponse{Error: "invalid_input", Message: "multipart field 'file' is required"})
		return
	}
	f, err := fh.Open()
	if err != nil {
		writeError(c, err)
		return
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		writeError(c, err)
		return
	}

	key := c.PostForm("key")
	if key == "" {
		key = fmt.Sprintf("uploads/%s/%s", uuid.NewString(), fh.Filename)
	}
	if err := h.store.Put(ctx, key, buf, "application/octet-stream"); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusCreated, UploadResponse{SourceKey: key})
}

// Convert godoc
// @Summary  Convert an archive already in R2 into AVIF pages (stored in R2).
// @Tags     convert
// @Accept   json
// @Produce  json
// @Param    request body domain.ConvertRequest true "Conversion request"
// @Success  200 {object} service.ConvertResult
// @Failure  400 {object} handler.ErrorResponse
// @Failure  415 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/convert [post]
func (h *ConvertHandler) Convert(ctx context.Context, c *app.RequestContext) {
	var req domain.ConvertRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ErrorResponse{Error: "invalid_input", Message: err.Error()})
		return
	}
	res, err := h.svc.Convert(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, res)
}

// JobStatus godoc
// @Summary  Get a conversion job's status.
// @Tags     convert
// @Produce  json
// @Param    id path string true "Job ID"
// @Success  200 {object} domain.ConvertJob
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/convert/jobs/{id} [get]
func (h *ConvertHandler) JobStatus(ctx context.Context, c *app.RequestContext) {
	job, err := h.svc.Job(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(consts.StatusOK, job)
}
