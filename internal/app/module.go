// Package app assembles the dependency graph with Uber FX. It binds concrete
// repositories to the service-layer ports and wires handlers into the server.
package app

import (
	"context"

	"go.uber.org/fx"

	"github.com/triasbrata/mihon-manga-server/internal/cleanup"
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
		func(c config.Config) config.OIDCConfig { return c.OIDC },
		func(c config.Config) convert.EncodeOptions {
			return convert.EncodeOptions{
				Quality: c.Image.Quality,
				Speed:   c.Image.Speed,
				MaxEdge: c.Image.MaxEdge,
			}
		},
	),

	// Infrastructure clients. The two Cloudflare Queue clients (cover mirror +
	// R2 cleanup) share a type, so they are built inside the constructors below
	// from their own config rather than provided as a shared *queue.Client.
	fx.Provide(
		d1.New,
		kv.New,
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
		fx.Annotate(d1.NewAdRepo, fx.As(new(service.AdRepository))),
		fx.Annotate(r2.New,
			fx.As(new(service.ObjectStore)), fx.As(new(covermirror.ObjectPutter)),
			fx.As(new(cleanup.ObjectDeleter))),
		fx.Annotate(oauth.NewClient, fx.As(new(service.OAuthClient))),
		fx.Annotate(kv.NewStateStore, fx.As(new(service.StateStore))),
	),

	// Async cover mirror + R2 cleanup: each producer is bound to the media
	// service's queue port, plus a background consumer whose lifecycle is
	// registered below. Each constructor builds its own queue client from config.
	fx.Provide(
		fx.Annotate(newCoverMirrorProducer, fx.As(new(service.CoverMirrorQueue))),
		newCoverMirrorWorker,
		fx.Annotate(newCleanupProducer, fx.As(new(service.CleanupQueue))),
		newCleanupWorker,
	),

	// Services bound to the handler-layer ports.
	fx.Provide(
		fx.Annotate(newMediaService, fx.As(new(handler.MediaService))),
		fx.Annotate(service.NewSourceService, fx.As(new(handler.SourceService))),
		fx.Annotate(service.NewSubTypeService, fx.As(new(handler.SubTypeService))),
		fx.Annotate(newAdService, fx.As(new(handler.AdService))),
		fx.Annotate(service.NewTaxonomyService, fx.As(new(handler.TaxonomyService))),
		fx.Annotate(service.NewConvertService, fx.As(new(handler.ConvertService))),
		fx.Annotate(newVideoService, fx.As(new(handler.VideoService))),
		fx.Annotate(service.NewNovelService, fx.As(new(handler.NovelService))),
		fx.Annotate(newConnectionService, fx.As(new(handler.ConnectionService))),
	),

	// Handlers.
	fx.Provide(
		handler.NewHealthHandler,
		handler.NewMediaHandler,
		handler.NewSourceHandler,
		handler.NewSubTypeHandler,
		handler.NewAdHandler,
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
	fx.Invoke(registerCleanup),
)

// newMediaService injects the public base URL, presign TTL, and the cover-mirror
// + cleanup queues from config.
func newMediaService(repo service.MediaRepository, store service.ObjectStore, coverQueue service.CoverMirrorQueue, cleanupQueue service.CleanupQueue, c config.Config) *service.MediaService {
	return service.NewMediaService(repo, store, coverQueue, cleanupQueue, c.HTTP.PublicBaseURL, c.R2.PresignTTL)
}

// newCoverMirrorProducer builds the cover-mirror queue producer over its queue.
func newCoverMirrorProducer(c config.Config) *covermirror.Producer {
	return covermirror.NewProducer(queue.New(c.Queue))
}

// newCoverMirrorWorker builds the cover-mirror background consumer.
func newCoverMirrorWorker(c config.Config, store covermirror.ObjectPutter, repo covermirror.CoverUpdater, opt convert.EncodeOptions) *covermirror.Worker {
	return covermirror.NewWorker(queue.New(c.Queue), store, repo, opt)
}

// newCleanupProducer builds the R2-cleanup queue producer over its queue.
func newCleanupProducer(c config.Config) *cleanup.Producer {
	return cleanup.NewProducer(queue.New(c.CleanupQueue.AsQueue()))
}

// newCleanupWorker builds the R2-cleanup background consumer.
func newCleanupWorker(c config.Config, store cleanup.ObjectDeleter) *cleanup.Worker {
	return cleanup.NewWorker(queue.New(c.CleanupQueue.AsQueue()), store)
}

// registerCoverMirror starts/stops the background cover-mirror consumer with the
// app lifecycle.
func registerCoverMirror(lc fx.Lifecycle, w *covermirror.Worker) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { w.Start(); return nil },
		OnStop:  func(context.Context) error { w.Stop(); return nil },
	})
}

// registerCleanup starts/stops the background R2-cleanup consumer with the app
// lifecycle.
func registerCleanup(lc fx.Lifecycle, w *cleanup.Worker) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { w.Start(); return nil },
		OnStop:  func(context.Context) error { w.Stop(); return nil },
	})
}

// newAdService injects the R2 store and presign TTL from config so the reader's
// house-ad image URLs share the page-URL lifetime.
func newAdService(repo service.AdRepository, store service.ObjectStore, c config.Config) *service.AdService {
	return service.NewAdService(repo, store, c.R2.PresignTTL)
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

