package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/1090-f/Memora/internal/app/api"
	"github.com/1090-f/Memora/internal/platform/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLiveHealthUsesStandardEnvelope(t *testing.T) {
	router := newTestRouter(t, allDependenciesHealthy())
	response := performRequest(router, http.MethodGet, "/health/live")

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"code":"OK"`)
}

func TestReadyHealthReturnsServiceUnavailableWhenDependencyFails(t *testing.T) {
	deps := allDependenciesHealthy()
	deps.RedisHealth = func(context.Context) error { return errors.New("redis unavailable") }
	router := newTestRouter(t, deps)

	response := performRequest(router, http.MethodGet, "/health/ready")

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Contains(t, response.Body.String(), `"code":"INTERNAL_ERROR"`)
}

func TestRecoveryReturnsInternalErrorWithRequestID(t *testing.T) {
	router := newTestRouter(t, allDependenciesHealthy())
	router.GET("/panic", func(*gin.Context) { panic("unexpected") })

	response := performRequest(router, http.MethodGet, "/panic")

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Contains(t, response.Body.String(), `"code":"INTERNAL_ERROR"`)
	require.NotEmpty(t, response.Header().Get("X-Request-ID"))
	require.Contains(t, response.Body.String(), response.Header().Get("X-Request-ID"))
}

func newTestRouter(t *testing.T, deps api.Dependencies) *gin.Engine {
	t.Helper()
	app, err := api.New(deps)
	require.NoError(t, err)
	return app.Router()
}

func allDependenciesHealthy() api.Dependencies {
	healthy := func(context.Context) error { return nil }
	return api.Dependencies{
		HTTP:           config.HTTPConfig{},
		DatabaseHealth: healthy,
		RedisHealth:    healthy,
		MinIOHealth:    healthy,
	}
}

func performRequest(router http.Handler, method, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
