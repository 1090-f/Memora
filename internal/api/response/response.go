// Package response 提供 Gin HTTP API 的统一响应格式。
package response

import (
	stderrors "errors"

	"github.com/1090-f/Memora/internal/api/httperror"
	"github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Envelope 是统一的 API 响应信封结构。
type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id"`
}

// Success 发送成功的 JSON 响应。
func Success(c *gin.Context, status int, data any) {
	c.JSON(status, Envelope{
		Code:      string(contracts.CodeOK),
		Message:   httperror.Message(contracts.CodeOK),
		Data:      data,
		RequestID: requestID(c),
	})
}

// Failure 将应用错误转换为统一的 HTTP 错误响应。
func Failure(c *gin.Context, err error) {
	appError := apperror.ErrInternal
	var typed *apperror.AppError
	if stderrors.As(err, &typed) {
		appError = typed
	}
	status := httperror.Status(appError.Code)
	logFailure(c, appError.Code, status, appError)
	c.JSON(status, Envelope{
		Code:      string(appError.Code),
		Message:   httperror.Message(appError.Code),
		Details:   appError.Details,
		RequestID: requestID(c),
	})
}

func logFailure(c *gin.Context, code contracts.ErrorCode, status int, appError *apperror.AppError) {
	fields := []zap.Field{
		zap.String("request_id", requestID(c)),
		zap.String("trace_id", c.GetString("trace_id")),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("code", string(code)),
		zap.Int("status", status),
	}
	if appError != nil && appError.Cause != nil {
		fields = append(fields, zap.Error(appError.Cause))
	} else {
		fields = append(fields, zap.String("message", httperror.Message(code)))
	}
	if status >= 500 {
		logger.Error("请求处理失败", fields...)
		return
	}
	logger.Warn("请求处理失败", fields...)
}

func requestID(c *gin.Context) string {
	value, _ := c.Get("request_id")
	requestID, _ := value.(string)
	return requestID
}
