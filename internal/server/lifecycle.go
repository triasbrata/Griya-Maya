package server

import (
	"context"
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app/server"
	"go.uber.org/fx"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// Register hooks the Hertz server into the fx lifecycle: Spin in a goroutine on
// start, graceful Close on stop.
func Register(lc fx.Lifecycle, h *server.Hertz, cfg config.Config) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			slog.Info("http server starting", "addr", cfg.HTTP.Addr)
			go func() {
				// Spin blocks until Close; it also wires Hertz's own signal
				// handling for in-flight graceful shutdown.
				h.Spin()
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			slog.Info("http server stopping")
			return h.Shutdown(ctx)
		},
	})
}
