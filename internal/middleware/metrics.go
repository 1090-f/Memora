package middleware

import (
	"time"

	"github.com/1090-f/Memora/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// Metrics 返回一个 HTTP 指标收集中间件，按方法、路径、状态码维度上报 Prometheus 指标。
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		metrics.HTTPStarted()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		metrics.HTTPFinished(c.Request.Method, path, c.Writer.Status(), time.Since(started))
	}
}
