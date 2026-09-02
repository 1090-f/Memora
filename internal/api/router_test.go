package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWorkerHealthReturnsQueueDiagnostics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health/workers", workerHealth(func(context.Context) (WorkerHealthSnapshot, error) {
		return WorkerHealthSnapshot{
			ActiveWorkers: 2,
			Document:      WorkerQueueHealth{Pending: 3, Running: 1, RedisPending: 2},
			Preview:       WorkerQueueHealth{Failed: 1},
			OutboxBacklog: 4,
		}, nil
	}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/workers", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{`"active_workers":2`, `"pending":3`, `"redis_pending":2`, `"outbox_backlog":4`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, recorder.Body.String())
		}
	}
}

func TestWorkerHealthIsUnavailableWithoutHeartbeat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health/workers", workerHealth(func(context.Context) (WorkerHealthSnapshot, error) {
		return WorkerHealthSnapshot{Document: WorkerQueueHealth{Pending: 2}}, nil
	}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/workers", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"pending":2`) {
		t.Fatalf("failure must retain queue diagnostics: %s", recorder.Body.String())
	}
}
