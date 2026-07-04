package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// HealthHandler serves liveness/readiness.
type HealthHandler struct{}

// NewHealthHandler wires a HealthHandler.
func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

// Health godoc
// @Summary  Liveness probe
// @Tags     system
// @Produce  json
// @Success  200 {object} map[string]string
// @Router   /healthz [get]
func (h *HealthHandler) Health(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusOK, map[string]string{"status": "ok"})
}
