package middleware

import (
	"net/http"
	"time"

	"github.com/1090-f/Memora/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const slowRequestThreshold = time.Second

type requestLogLevel uint8

const (
	requestLogDebug requestLogLevel = iota
	requestLogInfo
	requestLogWarn
)

// Logger 返回一个 HTTP 请求日志中间件，记录请求方法、路径、状态码、耗时等信息。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		duration := time.Since(started)
		fields := []zap.Field{
			zap.String("request_id", GetRequestID(c)), zap.String("trace_id", GetTraceID(c)), zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path), zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", duration), zap.String("client_ip", c.ClientIP()),
		}
		switch selectRequestLogLevel(c.Request.Method, c.Writer.Status(), duration) {
		case requestLogDebug:
			logger.Debug("HTTP Request", fields...)
		case requestLogWarn:
			logger.Warn("HTTP Request", fields...)
		default:
			logger.Info("HTTP Request", fields...)
		}
	}
}

func selectRequestLogLevel(method string, status int, duration time.Duration) requestLogLevel {
	if status >= http.StatusBadRequest || duration >= slowRequestThreshold {
		return requestLogWarn
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return requestLogDebug
	default:
		return requestLogInfo
	}
}
