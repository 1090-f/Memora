package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

const traceIDKey = "trace_id"

// Trace 返回一个链路追踪中间件，从 traceparent 头解析或生成 Trace ID。
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		parent := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		spanCtx, span := otel.Tracer("github.com/1090-f/Memora/http").Start(parent, c.Request.Method)
		defer span.End()
		traceID := ""
		if value := span.SpanContext().TraceID(); value.IsValid() {
			traceID = value.String()
		}
		if traceID == "" {
			traceID = traceIDFromParent(c.GetHeader("traceparent"))
		}
		if traceID == "" {
			traceID = newTraceID()
		}
		if traceID == "" {
			traceID = GetRequestID(c)
		}
		c.Set(traceIDKey, traceID)
		c.Request = c.Request.WithContext(contracts.WithCorrelation(spanCtx, traceID, GetRequestID(c)))
		c.Header("X-Trace-ID", traceID)
		c.Next()
		span.SetName(c.Request.Method + " " + c.FullPath())
		span.SetAttributes(attribute.String("http.request.method", c.Request.Method), attribute.String("http.route", c.FullPath()), attribute.Int("http.response.status_code", c.Writer.Status()), attribute.String("memora.request_id", GetRequestID(c)))
		if c.Writer.Status() >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", c.Writer.Status()))
		}
	}
}

// GetTraceID 从 Gin 上下文中获取链路追踪 ID。
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
