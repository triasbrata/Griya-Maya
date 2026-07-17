package oidc

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/go-webauthn/webauthn/protocol"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// WebAuthnHandler serves passkey (biometric) registration for the authenticated
// caller. Registration is bearer-gated: the subject is taken from the verified
// access token, so a user must already be signed in (password) to add a passkey.
// Passkey *login* itself lives on the OP's net/http mux (webauthn_login.go).
type WebAuthnHandler struct {
	storage *Storage
}

// NewWebAuthnHandler builds the handler from the OIDC provider's storage.
func NewWebAuthnHandler(p *Provider) *WebAuthnHandler {
	return &WebAuthnHandler{storage: p.storage}
}

// webauthnOptions documents the (opaque) WebAuthn ceremony options returned to
// the client for navigator.credentials.create/get. The publicKey member is the
// standard PublicKeyCredentialCreationOptions/RequestOptions structure.
type webauthnOptions struct {
	PublicKey map[string]any `json:"publicKey"`
}

// webauthnCredentialResponse is returned after a successful registration.
type webauthnCredentialResponse struct {
	ID string `json:"id"` // base64url credential id
}

// RegisterBegin starts a passkey registration for the caller.
//
// @Summary     Begin passkey registration
// @Description Returns WebAuthn PublicKeyCredentialCreationOptions for the authenticated user. Pass the result to navigator.credentials.create (browser) or ASAuthorizationController (native iOS), then submit the attestation to /v1/webauthn/register/finish.
// @Tags        webauthn
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} oidc.webauthnOptions "CredentialCreation options"
// @Failure     401 {object} oidc.userErrorBody
// @Failure     503 {object} oidc.userErrorBody "passkeys not configured"
// @Router      /v1/webauthn/register/begin [post]
func (h *WebAuthnHandler) RegisterBegin(ctx context.Context, c *app.RequestContext) {
	if h.storage.web == nil {
		h.fail(c, consts.StatusServiceUnavailable, "passkeys are not enabled")
		return
	}
	subject := SubjectFromContext(c)
	if subject == "" {
		h.fail(c, consts.StatusUnauthorized, "no authenticated subject")
		return
	}
	user, err := h.storage.loadWebAuthnUser(ctx, subject)
	if err != nil {
		h.fail(c, consts.StatusInternalServerError, "could not load user")
		return
	}
	creation, session, err := h.storage.web.BeginRegistration(user)
	if err != nil {
		h.fail(c, consts.StatusInternalServerError, "could not start registration")
		return
	}
	if err := h.storage.putWebAuthnSession(ctx, kvWebAuthnRegPrefix+subject, session); err != nil {
		h.fail(c, consts.StatusInternalServerError, "could not start registration")
		return
	}
	c.JSON(consts.StatusOK, creation)
}

// RegisterFinish verifies the attestation and persists the new credential.
//
// @Summary     Finish passkey registration
// @Description Verifies the authenticator attestation from RegisterBegin and stores the credential for the authenticated user. Optional ?name= labels the device.
// @Tags        webauthn
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       name query string false "device label"
// @Success     200 {object} oidc.webauthnCredentialResponse "registered credential id"
// @Failure     400 {object} oidc.userErrorBody
// @Failure     401 {object} oidc.userErrorBody
// @Failure     503 {object} oidc.userErrorBody "passkeys not configured"
// @Router      /v1/webauthn/register/finish [post]
func (h *WebAuthnHandler) RegisterFinish(ctx context.Context, c *app.RequestContext) {
	if h.storage.web == nil {
		h.fail(c, consts.StatusServiceUnavailable, "passkeys are not enabled")
		return
	}
	subject := SubjectFromContext(c)
	if subject == "" {
		h.fail(c, consts.StatusUnauthorized, "no authenticated subject")
		return
	}
	user, err := h.storage.loadWebAuthnUser(ctx, subject)
	if err != nil {
		h.fail(c, consts.StatusInternalServerError, "could not load user")
		return
	}
	session, err := h.storage.getWebAuthnSession(ctx, kvWebAuthnRegPrefix+subject)
	if err != nil {
		h.fail(c, consts.StatusBadRequest, "registration session expired, please retry")
		return
	}

	body, err := c.Body()
	if err != nil {
		h.fail(c, consts.StatusBadRequest, "invalid request body")
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(body)
	if err != nil {
		h.fail(c, consts.StatusBadRequest, "invalid attestation")
		return
	}
	cred, err := h.storage.web.CreateCredential(user, *session, parsed)
	if err != nil {
		h.fail(c, consts.StatusBadRequest, "attestation verification failed")
		return
	}
	name := string(c.Query("name"))
	if err := h.storage.addCredential(ctx, subject, name, cred); err != nil {
		h.fail(c, consts.StatusInternalServerError, "could not save credential")
		return
	}
	c.JSON(consts.StatusOK, webauthnCredentialResponse{ID: credentialKey(cred.ID)})
}

func (h *WebAuthnHandler) fail(c *app.RequestContext, status int, msg string) {
	code := "internal"
	switch status {
	case consts.StatusUnauthorized:
		code = "unauthorized"
	case consts.StatusBadRequest:
		code = "bad_request"
	case consts.StatusServiceUnavailable:
		code = "unavailable"
	}
	c.AbortWithStatusJSON(status, domain.APIResponse[any]{Success: false, ErrorCode: code, Message: msg})
}
