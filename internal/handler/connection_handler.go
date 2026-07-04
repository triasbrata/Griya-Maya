package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// ConnectionHandler exposes CRUD plus the OAuth authorize/callback/refresh flow
// for external-source connections under /v1/connections. Secrets and tokens are
// never serialized (the domain.Connection struct redacts them via json:"-").
type ConnectionHandler struct {
	svc ConnectionService
}

// NewConnectionHandler wires a ConnectionHandler.
func NewConnectionHandler(svc ConnectionService) *ConnectionHandler {
	return &ConnectionHandler{svc: svc}
}

// authorizeRequest is the body of POST /v1/connections/:id/authorize.
type authorizeRequest struct {
	RedirectURI string `json:"redirect_uri"`
}

// authorizeResponse is the body returned by the authorize endpoint.
type authorizeResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

// callbackRequest is the body of POST /v1/connections/callback.
type callbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

// Create godoc
// @Summary  Create an external-source connection.
// @Tags     connection
// @Accept   json
// @Produce  json
// @Param    request body domain.ConnectionWriteRequest true "Connection"
// @Success  201 {object} domain.Connection
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/connections [post]
func (h *ConnectionHandler) Create(ctx context.Context, c *app.RequestContext) {
	var req domain.ConnectionWriteRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.Create(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusCreated, res)
}

// List godoc
// @Summary  List external-source connections.
// @Tags     connection
// @Produce  json
// @Success  200 {array} domain.Connection
// @Security BearerAuth
// @Router   /v1/connections [get]
func (h *ConnectionHandler) List(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.List(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Get godoc
// @Summary  Get a connection by id.
// @Tags     connection
// @Produce  json
// @Param    id path string true "Connection ID"
// @Success  200 {object} domain.Connection
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/connections/{id} [get]
func (h *ConnectionHandler) Get(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Get(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Update godoc
// @Summary  Update a connection's label/credentials.
// @Tags     connection
// @Accept   json
// @Produce  json
// @Param    id      path string                         true "Connection ID"
// @Param    request body domain.ConnectionWriteRequest true "Connection"
// @Success  200 {object} domain.Connection
// @Failure  400 {object} handler.ErrorResponse
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/connections/{id} [put]
func (h *ConnectionHandler) Update(ctx context.Context, c *app.RequestContext) {
	var req domain.ConnectionWriteRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.Update(ctx, c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Delete godoc
// @Summary  Delete a connection.
// @Tags     connection
// @Param    id path string true "Connection ID"
// @Success  204
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/connections/{id} [delete]
func (h *ConnectionHandler) Delete(ctx context.Context, c *app.RequestContext) {
	if err := h.svc.Delete(ctx, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(consts.StatusNoContent)
}

// Authorize godoc
// @Summary  Begin the OAuth authorization for a connection.
// @Tags     connection
// @Accept   json
// @Produce  json
// @Param    id      path string           true "Connection ID"
// @Param    request body handler.authorizeRequest true "Redirect URI"
// @Success  200 {object} handler.authorizeResponse
// @Failure  400 {object} handler.ErrorResponse
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/connections/{id}/authorize [post]
func (h *ConnectionHandler) Authorize(ctx context.Context, c *app.RequestContext) {
	var req authorizeRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	url, err := h.svc.Authorize(ctx, c.Param("id"), req.RedirectURI)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, authorizeResponse{AuthorizeURL: url})
}

// Callback godoc
// @Summary  Complete the OAuth authorization (code + state exchange).
// @Tags     connection
// @Accept   json
// @Produce  json
// @Param    request body handler.callbackRequest true "Authorization code + state"
// @Success  200 {object} domain.Connection
// @Failure  400 {object} handler.ErrorResponse
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/connections/callback [post]
func (h *ConnectionHandler) Callback(ctx context.Context, c *app.RequestContext) {
	var req callbackRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.Callback(ctx, req.Code, req.State)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Refresh godoc
// @Summary  Refresh a connection's OAuth tokens.
// @Tags     connection
// @Produce  json
// @Param    id path string true "Connection ID"
// @Success  200 {object} domain.Connection
// @Failure  400 {object} handler.ErrorResponse
// @Failure  404 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/connections/{id}/refresh [post]
func (h *ConnectionHandler) Refresh(ctx context.Context, c *app.RequestContext) {
	res, err := h.svc.Refresh(ctx, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}
