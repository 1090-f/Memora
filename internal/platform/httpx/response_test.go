package httpx_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSuccessEnvelope(t *testing.T) {
	c, recorder := newTestContext("req-123")

	httpx.Success(c, http.StatusOK, gin.H{"value": 1})

	require.JSONEq(t, `{"code":"OK","message":"success","data":{"value":1},"request_id":"req-123"}`, recorder.Body.String())
}

func TestFailureEnvelopeMapsResourceNotFoundAndHidesCause(t *testing.T) {
	c, recorder := newTestContext("req-123")

	httpx.Failure(c, contracts.AppError{
		Code:    contracts.ResourceNotFound,
		Message: "memory not found",
		Cause:   errors.New("database connection details"),
	})

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.JSONEq(t, `{"code":"RESOURCE_NOT_FOUND","message":"memory not found","request_id":"req-123"}`, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "database connection details")
}

func TestFailureEnvelopeMapsApplicationErrorCodesToHTTPStatus(t *testing.T) {
	testCases := []struct {
		name   string
		code   contracts.ErrorCode
		status int
	}{
		{"invalid argument", contracts.InvalidArgument, http.StatusBadRequest},
		{"invalid state", contracts.InvalidState, http.StatusBadRequest},
		{"unsupported file type", contracts.UnsupportedFileType, http.StatusBadRequest},
		{"write MCP tool forbidden", contracts.WriteMCPToolForbidden, http.StatusBadRequest},
		{"unauthorized", contracts.Unauthorized, http.StatusUnauthorized},
		{"forbidden", contracts.Forbidden, http.StatusForbidden},
		{"network disabled", contracts.NetworkDisabled, http.StatusForbidden},
		{"resource not found", contracts.ResourceNotFound, http.StatusNotFound},
		{"duplicate resource", contracts.DuplicateResource, http.StatusConflict},
		{"index version conflict", contracts.IndexVersionConflict, http.StatusConflict},
		{"payload too large", contracts.PayloadTooLarge, http.StatusRequestEntityTooLarge},
		{"knowledge insufficient", contracts.KnowledgeInsufficient, http.StatusUnprocessableEntity},
		{"rate limited", contracts.RateLimited, http.StatusTooManyRequests},
		{"internal error", contracts.InternalError, http.StatusInternalServerError},
		{"model call failed", contracts.ModelCallFailed, http.StatusBadGateway},
		{"MCP call failed", contracts.MCPCallFailed, http.StatusBadGateway},
		{"upstream timeout", contracts.UpstreamTimeout, http.StatusGatewayTimeout},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c, recorder := newTestContext("req-123")

			httpx.Failure(c, contracts.AppError{Code: testCase.code})

			require.Equal(t, testCase.status, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+string(testCase.code)+`"`)
		})
	}
}

func TestRequestIDAcceptsValidHeaderAndWritesItToResponse(t *testing.T) {
	router := gin.New()
	router.Use(httpx.RequestID())
	router.GET("/", func(c *gin.Context) {
		require.Equal(t, "req-123", httpx.RequestIDFrom(c))
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "req-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, "req-123", recorder.Header().Get("X-Request-ID"))
}

func TestRequestIDGeneratesUUIDForMissingHeader(t *testing.T) {
	router := gin.New()
	router.Use(httpx.RequestID())
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, httpx.RequestIDFrom(c))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, recorder.Body.String())
	require.Equal(t, recorder.Body.String(), recorder.Header().Get("X-Request-ID"))
}

func newTestContext(requestID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("request_id", requestID)
	return c, recorder
}
