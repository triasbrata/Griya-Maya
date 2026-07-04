// Command server is the entrypoint for the Mihon manga server.
//
// It runs a Hertz HTTP API (handler -> service -> repository over D1/R2),
// converts CBZ/EPUB/PDF archives to AVIF, and protects write endpoints with an
// embedded self-hosted OpenID Provider (zitadel/oidc, backed by D1 + KV) plus
// OAuth2 Dynamic Client Registration. Designed to run in a Cloudflare Container.
// Configuration comes entirely from the environment (see .env.example).
//
// @title                     Mihon Manga Server
// @version                   0.1.0
// @description               Manga catalog + reader API that mirrors the Mihon iOS SourceRuntime contract, plus CBZ/EPUB/PDF -> AVIF conversion backed by Cloudflare R2/D1. Write endpoints are protected by an embedded OpenID Provider; clients onboard via OAuth2 Dynamic Client Registration (RFC 7591/7592).
// @BasePath                  /
//
// @tag.name catalog
// @tag.name reader
// @tag.name convert
// @tag.name oauth
// @tag.name system
//
// @securityDefinitions.apikey BearerAuth
// @in                        header
// @name                      Authorization
// @description               OAuth2 JWT access token issued by the embedded OpenID Provider. Use "Bearer <token>".
//
// @securityDefinitions.apikey RegistrationToken
// @in                        header
// @name                      Authorization
// @description               RFC 7592 registration_access_token. Use "Bearer <token>".
package main

import (
	"log/slog"
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/triasbrata/mihon-manga-server/internal/app"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	fx.New(
		app.Module,
		fx.WithLogger(func() fxevent.Logger {
			return &fxevent.SlogLogger{Logger: slog.Default()}
		}),
	).Run()
}
