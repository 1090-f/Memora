package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/gin-gonic/gin"
)

const traceIDKey = "trace_id"

func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := traceIDFromParent(c.GetHeader("traceparent"))
		if traceID == "" {
			traceID = newTraceID()
		}
		if traceID == "" {
			traceID = GetRequestID(c)
		}
		c.Set(traceIDKey, traceID)
		c.Header("X-Trace-ID", traceID)
		c.Next()
	}
}

func GetTraceID(c *gin.Context) string {
	value, _ := c.Get(traceIDKey)
	traceID, _ := value.(string)
	return traceID
}

func traceIDFromParent(value string) string {
	parts := strings.Split(value, "-")
	if len(parts) != 4 || len(parts[1]) != 32 || parts[1] == strings.Repeat("0", 32) {
		return ""
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return ""
	}
	return strings.ToLower(parts[1])
}

func newTraceID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return ""
	}
	return hex.EncodeToString(value)
}
