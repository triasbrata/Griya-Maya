package oidc

import (
	"context"
	"log/slog"
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/triasbrata/mihon-manga-server/internal/config"
	"github.com/triasbrata/mihon-manga-server/internal/repository/d1"
)

// AdminClientID is the well-known id of the seeded public PKCE client the
// TanStack admin front-end authenticates with.
const AdminClientID = "admin-web"

// IOSClientID is the well-known id of the seeded public PKCE client the iOS
// reader app authenticates with (Authorization Code + PKCE, no secret).
const IOSClientID = "mihon-ios"

// seedAdminClient ensures the static `admin-web` public PKCE client exists in
// D1. Best-effort: failures are logged (D1 may be unconfigured in local dev).
// It carries manga.read alongside manga.write so admin tooling can also exercise
// the gated reader endpoints.
func seedAdminClient(ctx context.Context, d1c *d1.Client, cfg config.OIDCConfig) {
	redirects := cfg.AdminRedirectURIs
	if len(redirects) == 0 {
		redirects = []string{"http://localhost:3000/auth/callback"}
	}
	scopes := []string{
		oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail,
		oidc.ScopeOfflineAccess, ScopeMangaWrite, ScopeMangaRead,
		ScopeConnectionsWrite, ScopeUsersRead, ScopeUsersWrite,
	}
	// Per-kind taxonomy write scopes (taksonomi.<kind>.write) so the admin panel
	// can manage every taxonomy kind; reads are covered by manga.read above.
	for _, k := range TaxonomyWriteKinds {
		scopes = append(scopes, ScopeTaxonomyWrite(k))
	}
	seedPublicClient(ctx, d1c, AdminClientID, "Mihon Admin Web", redirects, scopes)
}

// seedIOSClient ensures the static `mihon-ios` public PKCE client exists in D1.
// It is the end-user reader app's client: Authorization Code + PKCE with no
// secret, JWT access tokens, and the read-only scope set.
func seedIOSClient(ctx context.Context, d1c *d1.Client, cfg config.OIDCConfig) {
	redirects := cfg.IOSRedirectURIs
	if len(redirects) == 0 {
		redirects = []string{"mihon://auth/callback"}
	}
	seedPublicClient(ctx, d1c, IOSClientID, "Mihon iOS", redirects, []string{
		oidc.ScopeOpenID, oidc.ScopeOfflineAccess, ScopeMangaRead,
	})
}

// seedPublicClient inserts a static public (no-secret) PKCE client with the
// authorization_code + refresh_token grants if it does not already exist.
// Best-effort: failures are logged (D1 may be unconfigured in local dev).
func seedPublicClient(ctx context.Context, d1c *d1.Client, id, name string, redirects, scopes []string) {
	rows, err := d1c.Query(ctx, `SELECT id FROM oidc_client WHERE id = ?1`, id)
	if err != nil {
		slog.Warn("oidc: client seed skipped (lookup failed)", "client_id", id, "err", err)
		return
	}
	if len(rows) > 0 {
		return
	}

	err = d1c.Exec(ctx,
		`INSERT INTO oidc_client
		   (id, secret_hash, application_type, auth_method, redirect_uris,
		    post_logout_redirect_uris, grant_types, response_types, scopes,
		    access_token_type, dev_mode, client_name, registration_access_token, created_at)
		 VALUES (?1, '', 0, ?2, ?3, ?4, ?5, ?6, ?7, ?8, 0, ?9, '', ?10)`,
		id,
		string(oidc.AuthMethodNone),
		stringsToJSON(redirects),
		stringsToJSON([]string{}),
		stringsToJSON([]string{string(oidc.GrantTypeCode), string(oidc.GrantTypeRefreshToken)}),
		stringsToJSON([]string{string(oidc.ResponseTypeCode)}),
		stringsToJSON(scopes),
		int(op.AccessTokenTypeJWT),
		name,
		time.Now().Unix(),
	)
	if err != nil {
		slog.Warn("oidc: client seed failed", "client_id", id, "err", err)
		return
	}
	slog.Info("oidc: seeded client", "client_id", id, "redirect_uris", redirects)
}
