package oidc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/triasbrata/mihon-manga-server/internal/repository/d1"
)

// DCRHandler implements OAuth2 Dynamic Client Registration (RFC 7591) and client
// configuration management (RFC 7592) against D1.
type DCRHandler struct {
	d1     *d1.Client
	issuer string
}

// NewDCRHandler builds a DCR handler from the OIDC provider's storage.
func NewDCRHandler(p *Provider) *DCRHandler {
	return &DCRHandler{d1: p.storage.d1, issuer: p.storage.issuer}
}

// clientMetadata is the subset of RFC 7591 metadata we accept/echo.
type clientMetadata struct {
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	ApplicationType         string   `json:"application_type,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
	PostLogoutRedirectURIs  []string `json:"post_logout_redirect_uris,omitempty"`
}

// dcrErrorResponse is the RFC 7591/6749 error document returned by DCR endpoints.
type dcrErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// registrationResponse is the RFC 7591 success document.
type registrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at"`
	RegistrationAccessToken string   `json:"registration_access_token"`
	RegistrationClientURI   string   `json:"registration_client_uri"`
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ApplicationType         string   `json:"application_type,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
}

// Register handles RFC 7591 POST /connect/register.
//
// @Summary     Register an OAuth2 client (RFC 7591)
// @Description Dynamic Client Registration. Unauthenticated. Returns the new client_id plus a registration_access_token used to manage the client via RFC 7592. A client_secret is issued unless token_endpoint_auth_method is "none".
// @Tags        oauth
// @Accept      json
// @Produce     json
// @Param       request body oidc.clientMetadata true "Client metadata"
// @Success     201 {object} oidc.registrationResponse
// @Failure     400 {object} oidc.dcrErrorResponse
// @Router      /connect/register [post]
func (h *DCRHandler) Register(ctx context.Context, c *app.RequestContext) {
	var meta clientMetadata
	if err := json.Unmarshal(c.Request.Body(), &meta); err != nil {
		dcrError(c, consts.StatusBadRequest, "invalid_client_metadata", "malformed JSON body")
		return
	}

	// Apply RFC defaults.
	if len(meta.GrantTypes) == 0 {
		meta.GrantTypes = []string{string(oidc.GrantTypeCode)}
	}
	if len(meta.ResponseTypes) == 0 {
		meta.ResponseTypes = []string{string(oidc.ResponseTypeCode)}
	}
	authMethod := meta.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = string(oidc.AuthMethodBasic)
	}
	public := authMethod == string(oidc.AuthMethodNone)
	scopes := splitScope(meta.Scope)

	clientID := "dcr-" + randToken(12)
	regToken := randToken(32)

	var secret, secretHash string
	if !public {
		secret = randToken(32)
		var err error
		secretHash, err = hashPassword(secret)
		if err != nil {
			dcrError(c, consts.StatusInternalServerError, "server_error", "hash failure")
			return
		}
	}

	now := time.Now()
	err := h.d1.Exec(ctx,
		`INSERT INTO oidc_client
		   (id, secret_hash, application_type, auth_method, redirect_uris,
		    post_logout_redirect_uris, grant_types, response_types, scopes,
		    access_token_type, dev_mode, client_name, registration_access_token, created_at)
		 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, 0, ?11, ?12, ?13)`,
		clientID, secretHash, applicationTypeInt(meta.ApplicationType), authMethod,
		stringsToJSON(meta.RedirectURIs), stringsToJSON(meta.PostLogoutRedirectURIs),
		stringsToJSON(meta.GrantTypes), stringsToJSON(meta.ResponseTypes), stringsToJSON(scopes),
		int(op.AccessTokenTypeJWT), meta.ClientName, regToken, now.Unix())
	if err != nil {
		dcrError(c, consts.StatusInternalServerError, "server_error", "could not persist client")
		return
	}

	resp := registrationResponse{
		ClientID:                clientID,
		ClientSecret:            secret,
		ClientIDIssuedAt:        now.Unix(),
		ClientSecretExpiresAt:   0,
		RegistrationAccessToken: regToken,
		RegistrationClientURI:   h.issuer + "/connect/register/" + clientID,
		RedirectURIs:            meta.RedirectURIs,
		GrantTypes:              meta.GrantTypes,
		ResponseTypes:           meta.ResponseTypes,
		TokenEndpointAuthMethod: authMethod,
		ApplicationType:         meta.ApplicationType,
		ClientName:              meta.ClientName,
		Scope:                   strings.Join(scopes, " "),
	}
	writeJSON(c, consts.StatusCreated, resp)
}

// Read handles RFC 7592 GET /connect/register/{id}.
//
// @Summary     Read a client's registration (RFC 7592)
// @Tags        oauth
// @Produce     json
// @Param       id path string true "Client ID"
// @Success     200 {object} oidc.registrationResponse
// @Failure     401 {object} oidc.dcrErrorResponse
// @Failure     404 {object} oidc.dcrErrorResponse
// @Security    RegistrationToken
// @Router      /connect/register/{id} [get]
func (h *DCRHandler) Read(ctx context.Context, c *app.RequestContext) {
	row, ok := h.authorize(ctx, c)
	if !ok {
		return
	}
	writeJSON(c, consts.StatusOK, rowToResponse(row, h.issuer))
}

// Update handles RFC 7592 PUT /connect/register/{id}.
//
// @Summary     Update a client's registration (RFC 7592)
// @Tags        oauth
// @Accept      json
// @Produce     json
// @Param       id path string true "Client ID"
// @Param       request body oidc.clientMetadata true "Updated client metadata"
// @Success     200 {object} oidc.registrationResponse
// @Failure     400 {object} oidc.dcrErrorResponse
// @Failure     401 {object} oidc.dcrErrorResponse
// @Failure     404 {object} oidc.dcrErrorResponse
// @Security    RegistrationToken
// @Router      /connect/register/{id} [put]
func (h *DCRHandler) Update(ctx context.Context, c *app.RequestContext) {
	row, ok := h.authorize(ctx, c)
	if !ok {
		return
	}
	var meta clientMetadata
	if err := json.Unmarshal(c.Request.Body(), &meta); err != nil {
		dcrError(c, consts.StatusBadRequest, "invalid_client_metadata", "malformed JSON body")
		return
	}
	id := strVal(row["id"])
	authMethod := meta.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = strVal(row["auth_method"])
	}
	if len(meta.GrantTypes) == 0 {
		meta.GrantTypes = jsonToStrings(row["grant_types"])
	}
	if len(meta.ResponseTypes) == 0 {
		meta.ResponseTypes = jsonToStrings(row["response_types"])
	}
	scopes := splitScope(meta.Scope)
	if len(scopes) == 0 {
		scopes = jsonToStrings(row["scopes"])
	}
	err := h.d1.Exec(ctx,
		`UPDATE oidc_client SET
		   application_type = ?2, auth_method = ?3, redirect_uris = ?4,
		   post_logout_redirect_uris = ?5, grant_types = ?6, response_types = ?7,
		   scopes = ?8, client_name = ?9
		 WHERE id = ?1`,
		id, applicationTypeInt(meta.ApplicationType), authMethod,
		stringsToJSON(meta.RedirectURIs), stringsToJSON(meta.PostLogoutRedirectURIs),
		stringsToJSON(meta.GrantTypes), stringsToJSON(meta.ResponseTypes),
		stringsToJSON(scopes), meta.ClientName)
	if err != nil {
		dcrError(c, consts.StatusInternalServerError, "server_error", "could not update client")
		return
	}
	updated, _ := h.clientRow(ctx, id)
	writeJSON(c, consts.StatusOK, rowToResponse(updated, h.issuer))
}

// Delete handles RFC 7592 DELETE /connect/register/{id}.
//
// @Summary     Delete a client's registration (RFC 7592)
// @Tags        oauth
// @Produce     json
// @Param       id path string true "Client ID"
// @Success     204 "Deleted"
// @Failure     401 {object} oidc.dcrErrorResponse
// @Failure     404 {object} oidc.dcrErrorResponse
// @Security    RegistrationToken
// @Router      /connect/register/{id} [delete]
func (h *DCRHandler) Delete(ctx context.Context, c *app.RequestContext) {
	row, ok := h.authorize(ctx, c)
	if !ok {
		return
	}
	if err := h.d1.Exec(ctx, `DELETE FROM oidc_client WHERE id = ?1`, strVal(row["id"])); err != nil {
		dcrError(c, consts.StatusInternalServerError, "server_error", "could not delete client")
		return
	}
	c.SetStatusCode(consts.StatusNoContent)
}

// authorize loads the client row addressed by the path id and checks the
// registration_access_token bearer. On failure it writes the response and
// returns ok=false.
func (h *DCRHandler) authorize(ctx context.Context, c *app.RequestContext) (map[string]any, bool) {
	id := c.Param("id")
	row, err := h.clientRow(ctx, id)
	if err != nil || row == nil {
		dcrError(c, consts.StatusNotFound, "invalid_client_id", "unknown client")
		return nil, false
	}
	token := bearerToken(string(c.GetHeader("Authorization")))
	if token == "" || token != strVal(row["registration_access_token"]) {
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		dcrError(c, consts.StatusUnauthorized, "invalid_token", "invalid registration access token")
		return nil, false
	}
	return row, true
}

func (h *DCRHandler) clientRow(ctx context.Context, id string) (map[string]any, error) {
	rows, err := h.d1.Query(ctx,
		`SELECT id, secret_hash, application_type, auth_method, redirect_uris,
		        post_logout_redirect_uris, grant_types, response_types, scopes,
		        access_token_type, dev_mode, client_name, registration_access_token, created_at
		 FROM oidc_client WHERE id = ?1`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func rowToResponse(row map[string]any, issuer string) registrationResponse {
	id := strVal(row["id"])
	return registrationResponse{
		ClientID:                id,
		ClientIDIssuedAt:        timeVal(row["created_at"]).Unix(),
		ClientSecretExpiresAt:   0,
		RegistrationAccessToken: strVal(row["registration_access_token"]),
		RegistrationClientURI:   issuer + "/connect/register/" + id,
		RedirectURIs:            jsonToStrings(row["redirect_uris"]),
		GrantTypes:              jsonToStrings(row["grant_types"]),
		ResponseTypes:           jsonToStrings(row["response_types"]),
		TokenEndpointAuthMethod: strVal(row["auth_method"]),
		ClientName:              strVal(row["client_name"]),
		Scope:                   strings.Join(jsonToStrings(row["scopes"]), " "),
	}
}

func splitScope(scope string) []string {
	return strings.Fields(scope)
}

func applicationTypeInt(t string) int {
	switch t {
	case "native":
		return int(op.ApplicationTypeNative)
	case "user_agent", "useragent":
		return int(op.ApplicationTypeUserAgent)
	default:
		return int(op.ApplicationTypeWeb)
	}
}

func writeJSON(c *app.RequestContext, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		dcrError(c, consts.StatusInternalServerError, "server_error", "encode failure")
		return
	}
	c.Data(status, "application/json", b)
}

func dcrError(c *app.RequestContext, status int, code, desc string) {
	c.AbortWithStatusJSON(status, dcrErrorResponse{
		Error:            code,
		ErrorDescription: desc,
	})
}
