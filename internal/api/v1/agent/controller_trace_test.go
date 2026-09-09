package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type traceSpanRepoFake struct{ spans []entity.TraceSpan }

func (r traceSpanRepoFake) ListByRunTrace(context.Context, string, string) ([]entity.TraceSpan, error) {
	return r.spans, nil
}

func TestListTraceSpansChecksRunOwnershipAndReturnsSpans(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID, runID := uuid.New(), uuid.New()
	traceID := "0123456789abcdef0123456789abcdef"
	now := time.Now().UTC()
	controller := &Controller{
		runOwnerRepo: sseRunOwner{run: &entity.AgentRun{ID: runID, TraceID: &traceID}},
		traceSpanRepo: traceSpanRepoFake{spans: []entity.TraceSpan{{
			TraceID: traceID, SpanID: "0123456789abcdef", Name: "agent.run", Kind: "consumer", StatusCode: "Ok",
			StartedAt: now, EndedAt: now.Add(time.Second), DurationMS: 1000, Attributes: datatypes.JSON(`{"memora.run_id":"run-1"}`), Events: datatypes.JSON(`[]`),
		}}},
	}
	router := gin.New()
	router.GET("/runs/:id/trace", func(c *gin.Context) {
		c.Set("auth_user", &entity.User{BaseEntity: entity.BaseEntity{ID: userID.String()}})
		controller.ListTraceSpans(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/runs/"+runID.String()+"/trace", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"span_id":"0123456789abcdef"`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
