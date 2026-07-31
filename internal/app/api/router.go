package api

import (
	"log/slog"
	"time"

	systemhttp "github.com/1090-f/Memora/internal/modules/system/adapters/http"
	"github.com/1090-f/Memora/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

func newRouter(deps Dependencies) *gin.Engine {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	router := gin.New()
	router.Use(httpx.RequestID(), accessLog(logger), httpx.Recovery(logger))

	health := systemhttp.NewHealthHandler(systemhttp.HealthDependencies{
		Database: deps.DatabaseHealth,
		Redis:    deps.RedisHealth,
		MinIO:    deps.MinIOHealth,
	})
	router.GET("/health/live", health.Live)
	router.GET("/health/ready", health.Ready)
	return router
}

func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("HTTP request",
			"request_id", httpx.RequestIDFrom(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(started),
		)
	}
}
