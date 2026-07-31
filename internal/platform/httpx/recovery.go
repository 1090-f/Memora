package httpx

import (
	"log/slog"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/gin-gonic/gin"
)

// Recovery converts unexpected handler panics to the public error envelope.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("recovered HTTP panic", "request_id", RequestIDFrom(c), "panic", recovered)
				c.Abort()
				Failure(c, contracts.AppError{Code: contracts.InternalError})
			}
		}()
		c.Next()
	}
}
