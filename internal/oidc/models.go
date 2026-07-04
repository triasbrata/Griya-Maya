// Package oidc implements a self-hosted OpenID Provider (built on
// github.com/zitadel/oidc/v3) backed by Cloudflare D1 (durable state: clients,
// signing key, refresh tokens, admin users) and Cloudflare KV (ephemeral state:
// auth requests, auth codes, access tokens with TTL).
//
// It supports the authorization_code + PKCE, refresh_token and
// client_credentials (M2M) flows, ships an htmx login + consent UI, and exposes
// OAuth2 Dynamic Client Registration (RFC 7591/7592). It replaces the previous
// Ory-based auth stack.
//
// The design mirrors the zitadel/oidc example server (example/server/storage +
// example/server/exampleop), swapping the in-memory maps for D1/KV.
package oidc

import (
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// AuthRequest is the durable-enough model of an in-flight authorization request.
// It is stored as JSON in KV (keyed by id, and looked up by auth code) with a
// TTL, and implements op.AuthRequest.
//
// Fields are exported so the value round-trips through JSON in KV. The interface
// method Done() reads IsDone (a method and field cannot share a name).
type AuthRequest struct {
	ID            string             `json:"id"`
	CreationDate  time.Time          `json:"creation_date"`
	ApplicationID string             `json:"application_id"`
	CallbackURI   string             `json:"callback_uri"`
	TransferState string             `json:"transfer_state"`
	Prompt        []string           `json:"prompt"`
	LoginHint     string             `json:"login_hint"`
	MaxAuthAge    *time.Duration     `json:"max_auth_age"`
	UserID        string             `json:"user_id"`
	Scopes        []string           `json:"scopes"`
	ResponseType  oidc.ResponseType  `json:"response_type"`
	ResponseMode  oidc.ResponseMode  `json:"response_mode"`
	Nonce         string             `json:"nonce"`
	CodeChallenge *OIDCCodeChallenge `json:"code_challenge"`

	IsDone   bool      `json:"is_done"`
	AuthTime time.Time `json:"auth_time"`
}

func (a *AuthRequest) GetID() string            { return a.ID }
func (a *AuthRequest) GetACR() string           { return "" }
func (a *AuthRequest) GetAudience() []string    { return []string{a.ApplicationID} }
func (a *AuthRequest) GetAuthTime() time.Time   { return a.AuthTime }
func (a *AuthRequest) GetClientID() string      { return a.ApplicationID }
func (a *AuthRequest) GetNonce() string         { return a.Nonce }
func (a *AuthRequest) GetRedirectURI() string   { return a.CallbackURI }
func (a *AuthRequest) GetScopes() []string      { return a.Scopes }
func (a *AuthRequest) GetState() string         { return a.TransferState }
func (a *AuthRequest) GetSubject() string       { return a.UserID }
func (a *AuthRequest) Done() bool               { return a.IsDone }

func (a *AuthRequest) GetResponseType() oidc.ResponseType { return a.ResponseType }
func (a *AuthRequest) GetResponseMode() oidc.ResponseMode { return a.ResponseMode }

func (a *AuthRequest) GetAMR() []string {
	// This provider only authenticates with a password.
	if a.IsDone {
		return []string{"pwd"}
	}
	return nil
}

func (a *AuthRequest) GetCodeChallenge() *oidc.CodeChallenge {
	return codeChallengeToOIDC(a.CodeChallenge)
}

// OIDCCodeChallenge is the PKCE challenge captured from the auth request.
type OIDCCodeChallenge struct {
	Challenge string `json:"challenge"`
	Method    string `json:"method"`
}

func codeChallengeToOIDC(c *OIDCCodeChallenge) *oidc.CodeChallenge {
	if c == nil {
		return nil
	}
	method := oidc.CodeChallengeMethodPlain
	if c.Method == "S256" {
		method = oidc.CodeChallengeMethodS256
	}
	return &oidc.CodeChallenge{Challenge: c.Challenge, Method: method}
}

// authRequestToInternal maps the library's parsed request onto our model.
func authRequestToInternal(authReq *oidc.AuthRequest, userID string) *AuthRequest {
	var challenge *OIDCCodeChallenge
	if authReq.CodeChallenge != "" {
		challenge = &OIDCCodeChallenge{
			Challenge: authReq.CodeChallenge,
			Method:    string(authReq.CodeChallengeMethod),
		}
	}
	var maxAge *time.Duration
	if authReq.MaxAge != nil {
		d := time.Duration(*authReq.MaxAge) * time.Second
		maxAge = &d
	}
	return &AuthRequest{
		CreationDate:  time.Now(),
		ApplicationID: authReq.ClientID,
		CallbackURI:   authReq.RedirectURI,
		TransferState: authReq.State,
		Prompt:        promptToInternal(authReq.Prompt),
		LoginHint:     authReq.LoginHint,
		MaxAuthAge:    maxAge,
		UserID:        userID,
		Scopes:        authReq.Scopes,
		ResponseType:  authReq.ResponseType,
		ResponseMode:  authReq.ResponseMode,
		Nonce:         authReq.Nonce,
		CodeChallenge: challenge,
	}
}

func promptToInternal(oidcPrompt oidc.SpaceDelimitedArray) []string {
	prompts := make([]string, 0, len(oidcPrompt))
	for _, p := range oidcPrompt {
		switch p {
		case oidc.PromptNone, oidc.PromptLogin, oidc.PromptConsent, oidc.PromptSelectAccount:
			prompts = append(prompts, p)
		}
	}
	return prompts
}

// Token is the access-token record stored in KV (keyed by token ID) so the
// userinfo/introspection endpoints can resolve an opaque token back to a
// subject. JWT access tokens are additionally self-contained and verified by
// signature in the resource-server middleware.
type Token struct {
	ID             string    `json:"id"`
	ApplicationID  string    `json:"application_id"`
	Subject        string    `json:"subject"`
	RefreshTokenID string    `json:"refresh_token_id"`
	Audience       []string  `json:"audience"`
	Expiration     time.Time `json:"expiration"`
	Scopes         []string  `json:"scopes"`
}

// RefreshToken is the durable refresh-token record persisted in D1.
type RefreshToken struct {
	ID            string
	Token         string
	AuthTime      time.Time
	AMR           []string
	Audience      []string
	UserID        string
	ApplicationID string
	Expiration    time.Time
	Scopes        []string
	AccessToken   string // Token.ID this refresh token last minted
}

// RefreshTokenRequest wraps a RefreshToken to satisfy op.RefreshTokenRequest.
type RefreshTokenRequest struct {
	*RefreshToken
}

func refreshTokenRequest(t *RefreshToken) op.RefreshTokenRequest {
	return &RefreshTokenRequest{t}
}

func (r *RefreshTokenRequest) GetAMR() []string             { return r.AMR }
func (r *RefreshTokenRequest) GetAudience() []string        { return r.Audience }
func (r *RefreshTokenRequest) GetAuthTime() time.Time       { return r.AuthTime }
func (r *RefreshTokenRequest) GetClientID() string          { return r.ApplicationID }
func (r *RefreshTokenRequest) GetScopes() []string          { return r.Scopes }
func (r *RefreshTokenRequest) GetSubject() string           { return r.UserID }
func (r *RefreshTokenRequest) SetCurrentScopes(s []string)  { r.Scopes = s }
