// Package httpx contains HTTP transport helpers.
package httpx

import (
	"net/http"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/gin-gonic/gin"
)

// Success writes a successful public API response.
func Success(c *gin.Context, status int, data any) {
	c.JSON(status, contracts.Envelope{
		Code:      "OK",
		Message:   "success",
		Data:      data,
		RequestID: RequestIDFrom(c),
	})
}

// Failure writes a public API error response without exposing its internal cause.
func Failure(c *gin.Context, appError contracts.AppError) {
	FailureWithStatus(c, statusFor(appError.Code), appError)
}

// FailureWithStatus writes a public error response using an explicit HTTP status.
func FailureWithStatus(c *gin.Context, status int, appError contracts.AppError) {
	code := appError.Code
	if code == "" {
		code = contracts.InternalError
	}

	c.JSON(status, contracts.Envelope{
		Code:      string(code),
		Message:   errorMessage(code, appError.Message),
		Details:   appError.Details,
		RequestID: RequestIDFrom(c),
	})
}

func statusFor(code contracts.ErrorCode) int {
	switch code {
	case contracts.InvalidArgument, contracts.InvalidState, contracts.UnsupportedFileType, contracts.WriteMCPToolForbidden:
		return http.StatusBadRequest
	case contracts.Unauthorized:
		return http.StatusUnauthorized
	case contracts.Forbidden, contracts.NetworkDisabled:
		return http.StatusForbidden
	case contracts.ResourceNotFound:
		return http.StatusNotFound
	case contracts.DuplicateResource, contracts.IndexVersionConflict:
		return http.StatusConflict
	case contracts.PayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case contracts.KnowledgeInsufficient:
		return http.StatusUnprocessableEntity
	case contracts.RateLimited:
		return http.StatusTooManyRequests
	case contracts.ModelCallFailed, contracts.MCPCallFailed:
		return http.StatusBadGateway
	case contracts.UpstreamTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func errorMessage(code contracts.ErrorCode, message string) string {
	if message != "" {
		return message
	}

	switch code {
	case contracts.InvalidArgument:
		return "invalid argument"
	case contracts.InvalidState:
		return "invalid state"
	case contracts.UnsupportedFileType:
		return "unsupported file type"
	case contracts.WriteMCPToolForbidden:
		return "write MCP tool forbidden"
	case contracts.Unauthorized:
		return "unauthorized"
	case contracts.Forbidden:
		return "forbidden"
	case contracts.NetworkDisabled:
		return "network disabled"
	case contracts.ResourceNotFound:
		return "resource not found"
	case contracts.DuplicateResource:
		return "duplicate resource"
	case contracts.IndexVersionConflict:
		return "index version conflict"
	case contracts.PayloadTooLarge:
		return "payload too large"
	case contracts.KnowledgeInsufficient:
		return "knowledge insufficient"
	case contracts.RateLimited:
		return "rate limited"
	case contracts.ModelCallFailed:
		return "model call failed"
	case contracts.MCPCallFailed:
		return "MCP call failed"
	case contracts.UpstreamTimeout:
		return "upstream timeout"
	default:
		return "internal server error"
	}
}
