// Package app assembles the dependency graph with Uber FX. It binds concrete
// repositories to the service-layer ports and wires handlers into the server.
package app

import (
	"context"

	"go.uber.org/fx"

	"github.com/triasbrata/mihon-manga-server/internal/config"
	"github.com/triasbrata/mihon-manga-server/internal/convert"
	"github.com/triasbrata/mihon-manga-server/internal/covermirror"
	"github.com/triasbrata/mihon-manga-server/internal/handler"
	"github.com/triasbrata/mihon-manga-server/internal/oidc"
	"github.com/triasbrata/mihon-manga-server/internal/repository/d1"
	"github.com/triasbrata/mihon-manga-server/internal/repository/kv"
	"github.com/triasbrata/mihon-manga-server/internal/repository/oauth"
	"github.com/triasbrata/mihon-manga-server/internal/repository/queue"
	"github.com/triasbrata/mihon-manga-server/internal/repository/r2"
	"github.com/triasbrata/mihon-manga-server/internal/server"
	"github.com/triasbrata/mihon-manga-server/internal/service"
)

// Module is the root fx module for the server.
var Module = fx.Options(
	fx.Provide(config.Load),

	// Explode config into the sub-structs constructors depend on.
	fx.Provide(
		func(c config.Config) config.D1Config { return c.D1 },
		func(c config.Config) config.R2Config { return c.R2 },
		func(c config.Config) config.KVConfig { return c.KV },
		func(c config.Config) config.QueueConfig { return c.Queue },
		func(c config.Config) config.OIDCConfig { return c.OIDC },
		func(c config.Config) convert.EncodeOptions {
			return convert.EncodeOptions{
				Quality: c.Image.Quality,
				Speed:   c.Image.Speed,
				MaxEdge: c.Image.MaxEdge,
			}
		},
	),

	// Infrastructure clients.
	fx.Provide(
		d1.New,
		kv.New,
		queue.New,
	),

	// Embedded OpenID Provider (D1 + KV backed).
	fx.Provide(
		oidc.NewStorage,
		oidc.NewProvider,
		oidc.NewDCRHandler,
		oidc.NewUserAdminHandler,
	),

	// Repositories bound to service ports. MediaRepo and the R2 store are also
	// bound to the covermirror ports (one instance, multiple interfaces).
	fx.Provide(
		fx.Annotate(d1.NewMediaRepo,
			fx.As(new(service.MediaRepository)), fx.As(new(covermirror.CoverUpdater))),
		fx.Annotate(d1.NewJobRepo, fx.As(new(service.JobRepository))),
		fx.Annotate(d1.NewConnectionRepo, fx.As(new(service.ConnectionRepository))),
		fx.Annotate(d1.NewSourceRepo, fx.As(new(service.SourceRepository))),
		fx.Annotate(r2.New,
			fx.As(new(service.ObjectStore)), fx.As(new(covermirror.ObjectPutter))),
		fx.Annotate(convert.NewConverter, fx.As(new(service.ArchiveConverter))),
		fx.Annotate(oauth.NewClient, fx.As(new(service.OAuthClient))),
		fx.Annotate(kv.NewStateStore, fx.As(new(service.StateStore))),
	),

	// Async cover mirror: producer bound to the media service's queue port, plus
	// the background consumer whose lifecycle is registered below.
	fx.Provide(
		fx.Annotate(covermirror.NewProducer, fx.As(new(service.CoverMirrorQueue))),
		covermirror.NewWorker,
	),

	// Services bound to the handler-layer ports.
	fx.Provide(
		fx.Annotate(newMediaService, fx.As(new(handler.MediaService))),
		fx.Annotate(service.NewSourceService, fx.As(new(handler.SourceService))),
		fx.Annotate(service.NewTaxonomyService, fx.As(new(handler.TaxonomyService))),
		fx.Annotate(newConvertService, fx.As(new(handler.ConvertService))),
		fx.Annotate(newVideoService, fx.As(new(handler.VideoService))),
		fx.Annotate(service.NewNovelService, fx.As(new(handler.NovelService))),
		fx.Annotate(newConnectionService, fx.As(new(handler.ConnectionService))),
	),

	// Handlers.
	fx.Provide(
		handler.NewHealthHandler,
		handler.NewMediaHandler,
		handler.NewSourceHandler,
		handler.NewTaxonomyHandler,
		handler.NewConvertHandler,
		handler.NewVideoHandler,
		handler.NewNovelHandler,
		handler.NewConnectionHandler,
	),

	// HTTP server + lifecycle.
	fx.Provide(server.New),
	fx.Invoke(server.Register),
	fx.Invoke(registerCoverMirror),
)

// newMediaService injects the public base URL, presign TTL, and cover-mirror
// queue from config.
func newMediaService(repo service.MediaRepository, store service.ObjectStore, coverQueue service.CoverMirrorQueue, c config.Config) *service.MediaService {
	return service.NewMediaService(repo, store, coverQueue, c.HTTP.PublicBaseURL, c.R2.PresignTTL)
}

// registerCoverMirror starts/stops the background cover-mirror consumer with the
// app lifecycle.
func registerCoverMirror(lc fx.Lifecycle, w *covermirror.Worker) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { w.Start(); return nil },
		OnStop:  func(context.Context) error { w.Stop(); return nil },
	})
}

// newVideoService injects the public base URL from config.
func newVideoService(jobs service.JobRepository, store service.ObjectStore, c config.Config) *service.VideoService {
	return service.NewVideoService(jobs, store, c.HTTP.PublicBaseURL)
}

// newConnectionService injects the 32-byte connections encryption key from config.
func newConnectionService(
	repo service.ConnectionRepository,
	oauthClient service.OAuthClient,
	state service.StateStore,
	c config.Config,
) *service.ConnectionService {
	return service.NewConnectionService(repo, oauthClient, state, []byte(c.Connections.EncKey))
}

// newConvertService injects the convert timeout from config.
func newConvertService(
	jobs service.JobRepository,
	store service.ObjectStore,
	conv service.ArchiveConverter,
	c config.Config,
) *service.ConvertService {
	return service.NewConvertService(jobs, store, conv, c.Image.ConvertTimeout)
}
