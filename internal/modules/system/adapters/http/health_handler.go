// Package http contains system HTTP handlers.
package http

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const readinessTimeout = 2 * time.Second

// HealthCheck reports the health of one external dependency.
type HealthCheck func(context.Context) error

// HealthDependencies contains the dependency checks used by the readiness endpoint.
type HealthDependencies struct {
	Database HealthCheck
	Redis    HealthCheck
	MinIO    HealthCheck
}

// HealthHandler serves liveness and readiness checks.
type HealthHandler struct {
	dependencies HealthDependencies
}

func NewHealthHandler(dependencies HealthDependencies) *HealthHandler {
	return &HealthHandler{dependencies: dependencies}
}

// Live reports whether the HTTP process is running. It does not query dependencies.
func (h *HealthHandler) Live(c *gin.Context) {
	httpx.Success(c, 200, gin.H{"status": "live"})
}

// Ready checks all required dependencies concurrently with individual short deadlines.
func (h *HealthHandler) Ready(c *gin.Context) {
	results := h.checkAll(c.Request.Context())
	for _, err := range results {
		if err != nil {
			httpx.FailureWithStatus(c, http.StatusServiceUnavailable, contracts.AppError{
				Code:    contracts.InternalError,
				Message: "service unavailable",
				Details: readinessDetails(results),
			})
			return
		}
	}
	httpx.Success(c, 200, gin.H{"status": "ready"})
}

func (h *HealthHandler) checkAll(ctx context.Context) map[string]error {
	checks := map[string]HealthCheck{
		"database": h.dependencies.Database,
		"redis":    h.dependencies.Redis,
		"minio":    h.dependencies.MinIO,
	}
	results := make(map[string]error, len(checks))
	var mu sync.Mutex
	var wait sync.WaitGroup
	for name, check := range checks {
		wait.Add(1)
		go func() {
			defer wait.Done()
			checkCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
			defer cancel()
			var err error
			if check == nil {
				err = errors.New("health check is not configured")
			} else {
				err = check(checkCtx)
			}
			mu.Lock()
			results[name] = err
			mu.Unlock()
		}()
	}
	wait.Wait()
	return results
}

func readinessDetails(results map[string]error) map[string]string {
	details := make(map[string]string, len(results))
	for name, err := range results {
		if err == nil {
			details[name] = "ok"
			continue
		}
		details[name] = "unavailable"
	}
	return details
}
