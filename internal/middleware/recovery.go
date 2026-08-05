package middleware

import (
	"runtime/debug"

	apperrors "github.com/1090-f/Memora/pkg/errors"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery 返回一个 Panic 恢复中间件，捕获 HTTP 处理过程中的 panic 并返回 500 错误响应。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("已恢复 HTTP 运行时恐慌", zap.String("request_id", GetRequestID(c)), zap.String("trace_id", GetTraceID(c)), zap.Any("panic", recovered), zap.ByteString("stack", debug.Stack()))
				c.Abort()
				response.Failure(c, apperrors.ErrInternal)
			}
		}()
		c.Next()
	}
}
