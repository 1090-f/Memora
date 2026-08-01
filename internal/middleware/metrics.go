package middleware

import (
	"time"

	"github.com/1090-f/Memora/pkg/metrics"
	"github.com/gin-gonic/gin"
)

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
