package response

import (
	stderrors "errors"
	"net/http"

	apperrors "github.com/1090-f/Memora/pkg/errors"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Envelope 是统一的API响应信封结构，包含状态码、消息、数据和请求ID
type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id"`
}

// Success 发送成功的JSON响应，包含业务数据和请求ID
func Success(c *gin.Context, status int, data any) {
	c.JSON(status, Envelope{Code: string(apperrors.CodeOK), Message: apperrors.Message(apperrors.CodeOK), Data: data, RequestID: requestID(c)})
}

// Failure 发送错误的JSON响应，自动将AppError转换为统一响应格式
func Failure(c *gin.Context, err error) {
	appError := apperrors.ErrInternal
	var typed *apperrors.AppError
	if stderrors.As(err, &typed) {
		appError = typed
	}
	status := appError.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}
	logFailure(c, appError.Code, status, appError)
	c.JSON(status, Envelope{Code: string(appError.Code), Message: apperrors.Message(appError.Code), Details: appError.Details, RequestID: requestID(c)})
}

// logFailure 在错误边界统一记录结构化错误日志：5xx 记录为 Error（带堆栈），4xx 记录为 Warn。
func logFailure(c *gin.Context, code apperrors.Code, status int, appError *apperrors.AppError) {
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
		fields = append(fields, zap.String("message", apperrors.Message(code)))
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
