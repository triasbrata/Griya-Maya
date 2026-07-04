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

// seedAdminClient ensures the static `admin-web` public PKCE client exists in
// D1. Best-effort: failures are logged (D1 may be unconfigured in local dev).
func seedAdminClient(ctx context.Context, d1c *d1.Client, cfg config.OIDCConfig) {
	rows, err := d1c.Query(ctx, `SELECT id FROM oidc_client WHERE id = ?1`, AdminClientID)
	if err != nil {
		slog.Warn("oidc: admin client seed skipped (lookup failed)", "err", err)
		return
	}
	if len(rows) > 0 {
		return
	}

	redirects := cfg.AdminRedirectURIs
	if len(redirects) == 0 {
		redirects = []string{"http://localhost:3000/auth/callback"}
	}
	err = d1c.Exec(ctx,
		`INSERT INTO oidc_client
		   (id, secret_hash, application_type, auth_method, redirect_uris,
		    post_logout_redirect_uris, grant_types, response_types, scopes,
		    access_token_type, dev_mode, client_name, registration_access_token, created_at)
		 VALUES (?1, '', 0, ?2, ?3, ?4, ?5, ?6, ?7, ?8, 0, ?9, '', ?10)`,
		AdminClientID,
		string(oidc.AuthMethodNone),
		stringsToJSON(redirects),
		stringsToJSON([]string{}),
		stringsToJSON([]string{string(oidc.GrantTypeCode), string(oidc.GrantTypeRefreshToken)}),
		stringsToJSON([]string{string(oidc.ResponseTypeCode)}),
		stringsToJSON([]string{
			oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail,
			oidc.ScopeOfflineAccess, ScopeMangaWrite,
		}),
		int(op.AccessTokenTypeJWT),
		"Mihon Admin Web",
		time.Now().Unix(),
	)
	if err != nil {
		slog.Warn("oidc: admin client seed failed", "err", err)
		return
	}
	slog.Info("oidc: seeded admin-web client", "redirect_uris", redirects)
}
