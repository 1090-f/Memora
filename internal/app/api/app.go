// Package api wires the Memora HTTP API.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	systemhttp "github.com/1090-f/Memora/internal/modules/system/adapters/http"
	"github.com/1090-f/Memora/internal/platform/config"
	"github.com/gin-gonic/gin"
)

// Dependencies are the external checks and runtime settings required by the API.
type Dependencies struct {
	HTTP           config.HTTPConfig
	DatabaseHealth systemhttp.HealthCheck
	RedisHealth    systemhttp.HealthCheck
	MinIOHealth    systemhttp.HealthCheck
	Logger         *slog.Logger
}

// App owns the HTTP router and server lifecycle.
type App struct {
	router          *gin.Engine
	server          *http.Server
	shutdownTimeout time.Duration
}

// New builds an API application from explicit dependencies.
func New(deps Dependencies) (*App, error) {
	if deps.DatabaseHealth == nil || deps.RedisHealth == nil || deps.MinIOHealth == nil {
		return nil, fmt.Errorf("API health dependencies must be configured")
	}
	router := newRouter(deps)
	return &App{router: router, server: newServer(deps.HTTP, router), shutdownTimeout: shutdownTimeout(deps.HTTP)}, nil
}

// Router returns the configured HTTP router for tests and embedding.
func (a *App) Router() *gin.Engine {
	return a.router
}

// Run serves HTTP until the context is cancelled or the server stops unexpectedly.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		err := a.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
		defer cancel()
		return a.server.Shutdown(shutdownCtx)
	}
}
