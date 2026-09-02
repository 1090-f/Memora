package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestTraceContinuesIncomingW3CContext(t *testing.T) {
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	}()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), Trace())
	router.GET("/trace", func(c *gin.Context) {
		traceID, requestID := contracts.CorrelationFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"trace_id": traceID, "request_id": requestID})
	})

	request := httptest.NewRequest(http.MethodGet, "/trace", nil)
	request.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	request.Header.Set("X-Request-ID", "request-1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Trace-ID") != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("X-Trace-ID = %q", recorder.Header().Get("X-Trace-ID"))
	}
	if recorder.Header().Get("X-Request-ID") != "request-1" {
		t.Fatalf("X-Request-ID = %q", recorder.Header().Get("X-Request-ID"))
	}
}
