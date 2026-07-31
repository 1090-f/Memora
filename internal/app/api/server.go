package api

import (
	"net/http"
	"time"

	"github.com/1090-f/Memora/internal/platform/config"
)

const defaultShutdownTimeout = 10 * time.Second

func newServer(cfg config.HTTPConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
	}
}

func shutdownTimeout(cfg config.HTTPConfig) time.Duration {
	if cfg.ShutdownTimeout > 0 {
		return cfg.ShutdownTimeout
	}
	return defaultShutdownTimeout
}
