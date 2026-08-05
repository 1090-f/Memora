package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/api/response"
	"github.com/1090-f/Memora/internal/api/v1/auth"
	"github.com/1090-f/Memora/internal/api/v1/directory"
	"github.com/1090-f/Memora/internal/api/v1/knowledgebase"
	"github.com/1090-f/Memora/internal/api/v1/user"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// HealthCheck 是一个健康检查函数类型，用于检测依赖服务的健康状态。
type HealthCheck func(context.Context) error

// WorkerCount 是一个函数类型，用于返回当前活跃的 Worker 数量。
type WorkerCount func(context.Context) (int64, error)

// Dependencies 持有 API 路由所需的所有外部依赖。
type Dependencies struct {
	Config         config.CORSConfig
	Auth           service.AuthService
	Users          service.UserService
	KnowledgeBases service.KnowledgeBaseService
	Directories    service.DirectoryService
	PostgresHealth HealthCheck
	RedisHealth    HealthCheck
	MinIOHealth    HealthCheck
	WorkerCount    WorkerCount
}

// NewRouter 创建一个新的 Gin 引擎，注册所有中间件、健康检查端点和 v1 路由组。
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
	knowledgebase.NewController(deps.KnowledgeBases).RegisterRoutes(v1, authRequired)
	directory.NewController(deps.Directories).RegisterRoutes(v1, authRequired)
	return engine
}

// workerHealth 返回一个处理器，用于报告当前活跃的 Worker 数量。
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
			failure := apperrors.New(contracts.ErrServiceUnavailable, err)
			failure.Details = gin.H{"active_workers": count}
			response.Failure(c, failure)
			return
		}
		response.Success(c, http.StatusOK, gin.H{"status": "available", "active_workers": count})
	}
}

// readiness 返回一个处理器，并发检查所有基础设施依赖的就绪状态。
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
				err := apperrors.New(contracts.ErrServiceUnavailable, nil)
				err.Details = results
				response.Failure(c, err)
				return
			}
		}
		response.Success(c, http.StatusOK, gin.H{"status": "ready"})
	}
}
