package oidc

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// claimsKey namespaces the verified access-token claims on the request context.
const claimsKey = "oidc.claims"

// Middleware returns a Hertz handler that rejects requests lacking a valid
// Bearer JWT access token. It verifies the token locally against the OP's
// signing keys (issuer, signature, expiration) and enforces the provider-wide
// RequiredScope (typically manga.write, gating ingest/convert).
func (p *Provider) Middleware() app.HandlerFunc {
	return p.middleware(p.requiredScope)
}

// MiddlewareScope is like Middleware but enforces a specific scope, letting
// reads (manga.read) be gated independently from writes (manga.write).
func (p *Provider) MiddlewareScope(scope string) app.HandlerFunc {
	return p.middleware(scope)
}

// middleware builds the token-verifying handler enforcing the given scope. An
// empty scope skips the scope check (any valid token passes).
func (p *Provider) middleware(scope string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		token := bearerToken(string(c.GetHeader("Authorization")))
		if token == "" {
			unauthorized(c, "missing bearer token")
			return
		}
		verifier := p.op.AccessTokenVerifier(ctx)
		claims, err := op.VerifyAccessToken[*oidc.AccessTokenClaims](ctx, token, verifier)
		if err != nil {
			unauthorized(c, "invalid or expired token")
			return
		}
		if scope != "" && !hasScope(claims.Scopes, scope) {
			forbidden(c, "insufficient scope")
			return
		}
		c.Set(claimsKey, claims)
		c.Next(ctx)
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}

func hasScope(granted oidc.SpaceDelimitedArray, required string) bool {
	for _, s := range granted {
		if s == required {
			return true
		}
	}
	return false
}

func unauthorized(c *app.RequestContext, reason string) {
	c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
	c.AbortWithStatusJSON(consts.StatusUnauthorized, map[string]string{
		"error":             "unauthorized",
		"error_description": reason,
	})
}

func forbidden(c *app.RequestContext, reason string) {
	c.Header("WWW-Authenticate", `Bearer error="insufficient_scope"`)
	c.AbortWithStatusJSON(consts.StatusForbidden, map[string]string{
		"error":             "forbidden",
		"error_description": reason,
	})
}
