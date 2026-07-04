package oidc

import (
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// Client is the persisted OAuth/OIDC client model (a row of oidc_client). It
// implements op.Client.
type Client struct {
	id              string
	secretHash      string // argon2id PHC string; empty for public (PKCE) clients
	redirectURIs    []string
	postLogout      []string
	applicationType op.ApplicationType
	authMethod      oidc.AuthMethod
	responseTypes   []oidc.ResponseType
	grantTypes      []oidc.GrantType
	accessTokenType op.AccessTokenType
	devMode         bool
	clientName      string
	regAccessToken  string
	createdAt       time.Time
}

func (c *Client) GetID() string                       { return c.id }
func (c *Client) RedirectURIs() []string              { return c.redirectURIs }
func (c *Client) PostLogoutRedirectURIs() []string    { return c.postLogout }
func (c *Client) ApplicationType() op.ApplicationType { return c.applicationType }
func (c *Client) AuthMethod() oidc.AuthMethod         { return c.authMethod }
func (c *Client) ResponseTypes() []oidc.ResponseType  { return c.responseTypes }
func (c *Client) GrantTypes() []oidc.GrantType        { return c.grantTypes }
func (c *Client) AccessTokenType() op.AccessTokenType { return c.accessTokenType }
func (c *Client) IDTokenLifetime() time.Duration      { return time.Hour }
func (c *Client) DevMode() bool                       { return c.devMode }
func (c *Client) ClockSkew() time.Duration            { return 0 }
func (c *Client) IDTokenUserinfoClaimsAssertion() bool { return false }

// LoginURL redirects the user agent to our htmx login page, carrying the auth
// request id.
func (c *Client) LoginURL(id string) string {
	return "/login/username?authRequestID=" + id
}

func (c *Client) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string { return scopes }
}

func (c *Client) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string { return scopes }
}

// IsScopeAllowed permits our custom write scope on top of the standard ones.
func (c *Client) IsScopeAllowed(scope string) bool {
	return scope == ScopeMangaWrite
}

// clientFromRow maps a D1 oidc_client row onto a Client.
func clientFromRow(row map[string]any) *Client {
	return &Client{
		id:              strVal(row["id"]),
		secretHash:      strVal(row["secret_hash"]),
		redirectURIs:    jsonToStrings(row["redirect_uris"]),
		postLogout:      jsonToStrings(row["post_logout_redirect_uris"]),
		applicationType: op.ApplicationType(intVal(row["application_type"])),
		authMethod:      oidc.AuthMethod(strOr(strVal(row["auth_method"]), string(oidc.AuthMethodNone))),
		responseTypes:   toResponseTypes(jsonToStrings(row["response_types"])),
		grantTypes:      toGrantTypes(jsonToStrings(row["grant_types"])),
		accessTokenType: op.AccessTokenType(intVal(row["access_token_type"])),
		devMode:         intVal(row["dev_mode"]) != 0,
		clientName:      strVal(row["client_name"]),
		regAccessToken:  strVal(row["registration_access_token"]),
		createdAt:       timeVal(row["created_at"]),
	}
}

func toResponseTypes(vals []string) []oidc.ResponseType {
	out := make([]oidc.ResponseType, 0, len(vals))
	for _, v := range vals {
		out = append(out, oidc.ResponseType(v))
	}
	if len(out) == 0 {
		out = append(out, oidc.ResponseTypeCode)
	}
	return out
}

func toGrantTypes(vals []string) []oidc.GrantType {
	out := make([]oidc.GrantType, 0, len(vals))
	for _, v := range vals {
		out = append(out, oidc.GrantType(v))
	}
	if len(out) == 0 {
		out = append(out, oidc.GrantTypeCode)
	}
	return out
}

func strOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
