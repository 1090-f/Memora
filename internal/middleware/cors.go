package middleware

import (
	"strconv"
	"strings"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/gin-gonic/gin"
)

func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Vary", "Origin")
		}
		for _, allowed := range cfg.AllowOrigins {
			if allowed == origin || (allowed == "*" && !cfg.AllowCredentials) {
				c.Header("Access-Control-Allow-Origin", allowed)
				break
			}
		}
		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
		c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
		c.Header("Access-Control-Allow-Credentials", strconv.FormatBool(cfg.AllowCredentials))
		c.Header("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
