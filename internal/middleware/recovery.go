package middleware

import (
	"runtime/debug"

	apperrors "github.com/1090-f/Memora/pkg/errors"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("recovered HTTP panic", zap.String("request_id", GetRequestID(c)), zap.String("trace_id", GetTraceID(c)), zap.Any("panic", recovered), zap.ByteString("stack", debug.Stack()))
				c.Abort()
				response.Failure(c, apperrors.ErrInternal)
			}
		}()
		c.Next()
	}
}
