// Package server builds the Hertz engine, wires routes to handlers, applies the
// OAuth2 middleware to protected groups, and serves the OpenAPI docs.
package server

import (
	"context"
	_ "embed"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/cors"
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

	Config     config.Config
	OIDC       *oidc.Provider
	DCR        *oidc.DCRHandler
	UserAdmin  *oidc.UserAdminHandler
	Health     *handler.HealthHandler
	Media      *handler.MediaHandler
	Source     *handler.SourceHandler
	Taxonomy   *handler.TaxonomyHandler
	Convert    *handler.ConvertHandler
	Video      *handler.VideoHandler
	Novel      *handler.NovelHandler
	Connection *handler.ConnectionHandler
}

// New builds the Hertz server and registers all routes.
func New(p RouterParams) *server.Hertz {
	h := server.New(
		server.WithHostPorts(p.Config.HTTP.Addr),
		server.WithMaxRequestBodySize(512<<20), // 512 MiB uploads (archives)
	)

	// Browser CORS for the admin panel. It is served from a different origin and
	// reaches the API through the cloudflared tunnel (bypassing the fronting
	// Worker), so the server answers preflights itself — Hertz routes OPTIONS
	// through this global middleware, which aborts 204 with the allow headers.
	h.Use(cors.New(cors.Config{
		AllowOrigins: p.Config.HTTP.CORSAllowOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type", "Origin"},
		// Pagination metadata rides in response headers; expose them so the
		// cross-origin admin panel can read them off list responses.
		ExposeHeaders:    handler.PaginationHeaders,
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

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
		// zitadel/oidc registers the post-login/consent callback at
		// "<authorize endpoint>/callback" (see op.authCallbackPath) — i.e.
		// /authorize/callback. It must be bridged or the flow 404s after consent.
		"/authorize/callback",
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

	// Catalog (public reads — these responses carry no R2 page bytes). Media is
	// the unified entity (manga | video | novel), discriminated by `type`.
	v1 := h.Group("/v1")
	{
		v1.GET("/sources/:sourceId/popular", p.Media.Popular)
		v1.GET("/sources/:sourceId/latest", p.Media.Latest)
		v1.GET("/sources/:sourceId/search", p.Media.Search)
		v1.GET("/sources/:sourceId/recommendations", p.Media.Recommendations)
		v1.GET("/sources/:sourceId/genres", p.Media.Genres)
		v1.GET("/sources/:sourceId/categories", p.Media.Categories)
		v1.GET("/media/:id", p.Media.Details)
		v1.GET("/media/:id/chapters", p.Media.Chapters)
		// Previous/next chapter around a chapter — metadata only (no R2 page
		// bytes), so it shares the public catalog gate with the chapters list.
		v1.GET("/chapters/:id/adjacent", p.Media.ChapterNeighbors)
		// HLS streaming proxy (public read): path-based so a playlist's relative
		// segment URIs resolve. Used when no public R2 domain is configured.
		v1.GET("/stream/*key", p.Video.Stream)
	}

	// Reader (gated by manga.read): the page list hands out short-lived
	// presigned R2 URLs, so a valid read token is required to mint fetchable
	// page bytes. The legacy /v1/image proxy is retired behind the same gate so
	// the bucket stays fully private (prefer presigned URLs; keep R2 public base
	// empty).
	read := h.Group("/v1", p.OIDC.MiddlewareScope(oidc.ScopeMangaRead))
	{
		read.GET("/chapters/:id/pages", p.Media.Pages)
		read.GET("/image", p.Media.Image)

		// Taxonomy reads share the reader scope: listing genres/categories/
		// authors/artists needs only manga.read.
		read.GET("/taxonomies/:kind", p.Taxonomy.List)

		// Enabled source directory for the reader (end client).
		read.GET("/sources", p.Source.List)
	}

	// Admin surface (gated separately from catalog scopes): source management.
	// Reads (incl. disabled sources) need admin.read; mutations need admin.write.
	adminRead := h.Group("/v1/admin", p.OIDC.MiddlewareScope(oidc.ScopeAdminRead))
	{
		adminRead.GET("/sources", p.Source.AdminList)
		adminRead.GET("/sources/:id", p.Source.Get)
	}
	adminWrite := h.Group("/v1/admin", p.OIDC.MiddlewareScope(oidc.ScopeAdminWrite))
	{
		adminWrite.POST("/sources", p.Source.Create)
		adminWrite.PUT("/sources/:id", p.Source.Update)
		adminWrite.DELETE("/sources/:id", p.Source.Delete)
	}

	// Catalog management (gated by manga.write): create/update/delete media and
	// chapters.
	manage := h.Group("/v1", p.OIDC.Middleware())
	{
		manage.POST("/media", p.Media.CreateMedia)
		manage.PUT("/media/:id", p.Media.UpdateMedia)
		manage.DELETE("/media/:id", p.Media.DeleteMedia)
		manage.POST("/media/:id/chapters", p.Media.CreateChapter)
		manage.PUT("/chapters/:id", p.Media.UpdateChapter)
		manage.DELETE("/chapters/:id", p.Media.DeleteChapter)
	}

	// Taxonomy mutations, gated per kind by taksonomi.<kind>.write (resolved from
	// the :kind path param) so each taxonomy can be delegated independently.
	// /v1/taxonomies/{kind} where kind is one of genres | categories | authors |
	// artists. Reads live in the manga.read group above.
	taxonomy := h.Group("/v1", p.OIDC.MiddlewareTaxonomyWrite())
	{
		taxonomy.POST("/taxonomies/:kind", p.Taxonomy.Create)
		taxonomy.PUT("/taxonomies/:kind/:id", p.Taxonomy.Update)
		taxonomy.DELETE("/taxonomies/:kind/:id", p.Taxonomy.Delete)
	}

	// External-source OAuth connections (MyAnimeList first), gated by their own
	// connections.write scope so managing upstream credentials is authorized
	// separately from catalog writes. The authorize → callback → refresh flow
	// stores encrypted tokens for later use.
	connections := h.Group("/v1", p.OIDC.MiddlewareScope(oidc.ScopeConnectionsWrite))
	{
		connections.POST("/connections", p.Connection.Create)
		connections.GET("/connections", p.Connection.List)
		connections.GET("/connections/:id", p.Connection.Get)
		connections.PUT("/connections/:id", p.Connection.Update)
		connections.DELETE("/connections/:id", p.Connection.Delete)
		connections.POST("/connections/:id/authorize", p.Connection.Authorize)
		connections.POST("/connections/callback", p.Connection.Callback)
		connections.POST("/connections/:id/refresh", p.Connection.Refresh)
		connections.GET("/connections/:id/search", p.Connection.Search)
	}

	// Admin user management. Reads are gated by users.read, mutations by
	// users.write, so a read-only admin role can be granted independently.
	usersRead := h.Group("/v1/users", p.OIDC.MiddlewareScope(oidc.ScopeUsersRead))
	{
		usersRead.GET("", p.UserAdmin.List)
		usersRead.GET("/:id", p.UserAdmin.Get)
	}
	usersWrite := h.Group("/v1/users", p.OIDC.MiddlewareScope(oidc.ScopeUsersWrite))
	{
		usersWrite.POST("", p.UserAdmin.Create)
		usersWrite.PUT("/:id", p.UserAdmin.Update)
		usersWrite.DELETE("/:id", p.UserAdmin.Delete)
	}

	// Conversion (protected by the OIDC access-token middleware).
	secured := h.Group("/v1/convert", p.OIDC.Middleware())
	{
		secured.POST("/upload", p.Convert.Upload)
		secured.POST("", p.Convert.Convert)
		secured.POST("/probe", p.Convert.Probe)
		secured.GET("/jobs/:id", p.Convert.JobStatus)
	}

	// HLS video ingest (protected): upload a bundle, then register it as a
	// chapter's video page.
	video := h.Group("/v1/video", p.OIDC.Middleware())
	{
		video.POST("/upload", p.Video.Upload)
		video.POST("", p.Video.Register)
	}

	// Novel text ingest (protected): register a chapter's text (inline or by an
	// already-uploaded R2 key). Served inline via /v1/chapters/{id}/pages.
	novel := h.Group("/v1/novel", p.OIDC.Middleware())
	{
		novel.POST("", p.Novel.Register)
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
