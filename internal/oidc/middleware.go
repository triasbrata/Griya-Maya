package oidc

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// claimsKey namespaces the verified access-token claims on the request context.
const claimsKey = "oidc.claims"

// Middleware returns a Hertz handler that rejects requests lacking a valid
// Bearer JWT access token. It verifies the token locally against the OP's
// signing keys (issuer, signature, expiration) and enforces the provider-wide
// RequiredScope (typically manga.write, gating ingest/convert).
func (p *Provider) Middleware() app.HandlerFunc {
	return p.middleware(func(*app.RequestContext) string { return p.requiredScope })
}

// MiddlewareScope is like Middleware but enforces a specific scope, letting
// reads (manga.read) be gated independently from writes (manga.write).
func (p *Provider) MiddlewareScope(scope string) app.HandlerFunc {
	return p.middleware(func(*app.RequestContext) string { return scope })
}

// MiddlewareTaxonomyWrite gates taxonomy mutations on a per-kind write scope
// (taksonomi.<kind>.write) resolved from the :kind path param, so e.g. editing
// genres needs taksonomi.genres.write. Unknown kinds resolve to no scope and are
// left to the handler's 404 (there is nothing to authorize for a kind that does
// not exist).
func (p *Provider) MiddlewareTaxonomyWrite() app.HandlerFunc {
	return p.middleware(func(c *app.RequestContext) string {
		kind := c.Param("kind")
		if !isTaxonomyWriteKind(kind) {
			return ""
		}
		return ScopeTaxonomyWrite(kind)
	})
}

// middleware builds the token-verifying handler enforcing the scope returned by
// scopeFor for the request. A "" scope skips the scope check (any valid token
// passes), letting callers defer unknown-resource cases to the handler.
func (p *Provider) middleware(scopeFor func(*app.RequestContext) string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		token := bearerToken(string(c.GetHeader("Authorization")))
		if token == "" {
			unauthorized(c, "missing bearer token")
			return
		}
		// AccessTokenVerifier reads the expected issuer from the context
		// (IssuerFromContext). Our Hertz middleware never passes through zitadel's
		// IssuerInterceptor (which wraps only the OP's own HTTP handlers), so we
		// inject the configured issuer here — otherwise the verifier expects an
		// empty issuer and rejects every token ("issuer does not match").
		vctx := op.ContextWithIssuer(ctx, p.issuer)
		verifier := p.op.AccessTokenVerifier(vctx)
		claims, err := op.VerifyAccessToken[*oidc.AccessTokenClaims](vctx, token, verifier)
		if err != nil {
			// Surface the concrete reason (expired, not-yet-valid, bad signature,
			// issuer mismatch, unknown signing key, …) instead of a blanket
			// "invalid or expired token" — a valid-looking token that fails here is
			// almost always a signing-key rotation or issuer mismatch, not expiry.
			unauthorized(c, "token verification failed: "+err.Error())
			return
		}
		if scope := scopeFor(c); scope != "" && !hasScope(claims.Scopes, scope) {
			forbidden(c, "insufficient scope: token needs '"+scope+"', has ["+strings.Join(claims.Scopes, " ")+"]")
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
	c.AbortWithStatusJSON(consts.StatusUnauthorized, domain.APIResponse[any]{
		Success:   false,
		ErrorCode: "unauthorized",
		Message:   reason,
	})
}

func forbidden(c *app.RequestContext, reason string) {
	c.Header("WWW-Authenticate", `Bearer error="insufficient_scope"`)
	c.AbortWithStatusJSON(consts.StatusForbidden, domain.APIResponse[any]{
		Success:   false,
		ErrorCode: "forbidden",
		Message:   reason,
	})
}
