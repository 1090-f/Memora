package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/api/v1/auth"
	mcpapi "github.com/1090-f/Memora/internal/api/v1/mcp"
	"github.com/1090-f/Memora/internal/api/v1/user"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/config"
	apperrors "github.com/1090-f/Memora/pkg/errors"
	"github.com/1090-f/Memora/pkg/metrics"
	"github.com/1090-f/Memora/pkg/response"
	"github.com/gin-gonic/gin"
)

type HealthCheck func(context.Context) error
type WorkerCount func(context.Context) (int64, error)

type Dependencies struct {
	Config         config.CORSConfig
	Auth           service.AuthService
	Users          service.UserService
	MCP            service.ImportService
	PostgresHealth HealthCheck
	RedisHealth    HealthCheck
	MinIOHealth    HealthCheck
	WorkerCount    WorkerCount
}

func NewRouter(deps Dependencies) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Trace(), middleware.Metrics(), middleware.Logger(), middleware.Recovery(), middleware.CORS(deps.Config))
	engine.GET("/metrics", gin.WrapH(metrics.Handler()))
	engine.GET("/health/live", func(c *gin.Context) { response.Success(c, http.StatusOK, gin.H{"status": "live"}) })
	engine.GET("/health/ready", readiness(deps))
	engine.GET("/health/workers", workerHealth(deps.WorkerCount))
	v1 := engine.Group("/api/v1")
	authRequired := middleware.Auth(deps.Auth)
	auth.NewController(deps.Auth).RegisterRoutes(v1, authRequired)
	user.NewController(deps.Users).RegisterRoutes(v1, authRequired)
	if deps.MCP != nil {
		mcpapi.NewController(deps.MCP).RegisterRoutes(v1, authRequired)
	}
	return engine
}

func workerHealth(countWorkers WorkerCount) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if countWorkers == nil {
			response.Failure(c, apperrors.ErrInternal)
			return
		}
		count, err := countWorkers(ctx)
		if err != nil || count == 0 {
			failure := *apperrors.ErrInternal
			failure.HTTPStatus = http.StatusServiceUnavailable
			failure.Details = gin.H{"active_workers": count}
			response.Failure(c, &failure)
			return
		}
		response.Success(c, http.StatusOK, gin.H{"status": "available", "active_workers": count})
	}
}

func readiness(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		checks := map[string]HealthCheck{"postgres": deps.PostgresHealth, "redis": deps.RedisHealth, "minio": deps.MinIOHealth}
		results := make(map[string]string, len(checks))
		var mutex sync.Mutex
		var wait sync.WaitGroup
		for name, check := range checks {
			wait.Add(1)
			go func() {
				defer wait.Done()
				ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
				defer cancel()
				status := "ok"
				if check == nil || check(ctx) != nil {
					status = "unavailable"
				}
				mutex.Lock()
				results[name] = status
				mutex.Unlock()
			}()
		}
		wait.Wait()
		for _, status := range results {
			if status != "ok" {
				err := *apperrors.ErrInternal
				err.HTTPStatus = http.StatusServiceUnavailable
				err.Details = results
				response.Failure(c, &err)
				return
			}
		}
		response.Success(c, http.StatusOK, gin.H{"status": "ready"})
	}
}
