// Package server builds the Hertz engine, wires routes to handlers, applies the
// OAuth2 middleware to protected groups, and serves the OpenAPI docs.
package server

import (
	"context"
	_ "embed"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.uber.org/fx"

	"github.com/triasbrata/mihon-manga-server/internal/config"
	"github.com/triasbrata/mihon-manga-server/internal/handler"
	"github.com/triasbrata/mihon-manga-server/internal/oidc"
)

// The API spec is generated from swag annotations (`make docs`) and embedded
// here as the single source of truth. Regenerate after changing any handler's
// swagger comments.
//
//go:embed swagger/swagger.yaml
var openAPISpec []byte

// RouterParams collects everything the router needs via fx.
type RouterParams struct {
	fx.In

	Config   config.Config
	OIDC     *oidc.Provider
	DCR      *oidc.DCRHandler
	Health   *handler.HealthHandler
	Manga    *handler.MangaHandler
	Convert  *handler.ConvertHandler
}

// New builds the Hertz server and registers all routes.
func New(p RouterParams) *server.Hertz {
	h := server.New(
		server.WithHostPorts(p.Config.HTTP.Addr),
		server.WithMaxRequestBodySize(512<<20), // 512 MiB uploads (archives)
	)

	// System + docs (public).
	h.GET("/healthz", p.Health.Health)
	h.GET("/openapi.yaml", serveSpec)
	h.GET("/docs", serveSwaggerUI)

	// Embedded OpenID Provider: discovery, authorize, token, userinfo, keys,
	// revoke, end_session, plus the htmx login/consent UI. The provider is a
	// net/http handler; we adapt it and mount it under the endpoint prefixes it
	// owns (avoiding /healthz, which we serve ourselves).
	opHandler := adaptor.HertzHandler(p.OIDC.Handler())
	for _, path := range []string{
		"/.well-known/*action",
		"/authorize",
		"/callback",
		"/oauth/*action",
		"/userinfo",
		"/keys",
		"/revoke",
		"/end_session",
		"/device_authorization",
		"/login/*action",
		"/logged-out",
	} {
		h.Any(path, opHandler)
	}

	// Catalog + reader (public reads).
	v1 := h.Group("/v1")
	{
		v1.GET("/sources/:sourceId/popular", p.Manga.Popular)
		v1.GET("/sources/:sourceId/latest", p.Manga.Latest)
		v1.GET("/sources/:sourceId/search", p.Manga.Search)
		v1.GET("/sources/:sourceId/genres", p.Manga.Genres)
		v1.GET("/manga/:id", p.Manga.Details)
		v1.GET("/manga/:id/chapters", p.Manga.Chapters)
		v1.GET("/chapters/:id/pages", p.Manga.Pages)
		v1.GET("/image", p.Manga.Image)
	}

	// Conversion (protected by the OIDC access-token middleware).
	secured := h.Group("/v1/convert", p.OIDC.Middleware())
	{
		secured.POST("/upload", p.Convert.Upload)
		secured.POST("", p.Convert.Convert)
		secured.GET("/jobs/:id", p.Convert.JobStatus)
	}

	// OAuth2 Dynamic Client Registration (RFC 7591/7592), backed by D1. RFC 7592
	// endpoints authenticate with the registration_access_token, so no bearer
	// middleware here.
	reg := h.Group("/connect/register")
	{
		reg.POST("", p.DCR.Register)
		reg.GET("/:id", p.DCR.Read)
		reg.PUT("/:id", p.DCR.Update)
		reg.DELETE("/:id", p.DCR.Delete)
	}

	return h
}

// serveSpec returns the embedded swag-generated API document.
func serveSpec(_ context.Context, c *app.RequestContext) {
	c.Data(consts.StatusOK, "application/yaml; charset=utf-8", openAPISpec)
}

// serveSwaggerUI renders a self-loading Swagger UI page pointing at /openapi.yaml.
func serveSwaggerUI(_ context.Context, c *app.RequestContext) {
	c.Data(consts.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
}

const swaggerHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Mihon Manga Server — API</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: '/openapi.yaml', dom_id: '#swagger-ui' });
  </script>
</body>
</html>`
