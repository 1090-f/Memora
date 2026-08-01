package middleware

import (
	"time"

	"github.com/1090-f/Memora/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("HTTP request",
			zap.String("request_id", GetRequestID(c)), zap.String("trace_id", GetTraceID(c)), zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path), zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", time.Since(started)), zap.String("client_ip", c.ClientIP()),
		)
	}
}
