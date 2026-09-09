package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type sseRunOwner struct{ run *entity.AgentRun }

func (r sseRunOwner) FindByID(context.Context, uuid.UUID, uuid.UUID) (*entity.AgentRun, error) {
	return r.run, nil
}

type sseEventRepo struct{ events []entity.AgentEvent }

func (r sseEventRepo) BatchCreate(context.Context, []entity.AgentEvent) error { return nil }
func (r sseEventRepo) DeleteByRunID(context.Context, string) error            { return nil }
func (r sseEventRepo) ListAfterSequence(_ context.Context, _ string, after int64) ([]entity.AgentEvent, error) {
	result := make([]entity.AgentEvent, 0, len(r.events))
	for _, event := range r.events {
		if event.Sequence > after {
			result = append(result, event)
		}
	}
	return result, nil
}

type sseSubscriber struct {
	events        []contracts.AgentEvent
	afterSequence int64
}

func (s *sseSubscriber) Subscribe(_ context.Context, _ contracts.ID, after int64) (<-chan contracts.AgentEvent, error) {
	s.afterSequence = after
	ch := make(chan contracts.AgentEvent, len(s.events))
	for _, event := range s.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func TestSubscribeEventsReplaysDatabaseThenContinuesLiveWithoutDuplicates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	runID := uuid.New()
	traceID := "0123456789abcdef0123456789abcdef"
	requestID := "request-1"
	stage := string(contracts.AgentStageContextBuild)
	status := string(contracts.StageSucceeded)
	now := time.Now().UTC()
	subscriber := &sseSubscriber{events: []contracts.AgentEvent{{
		RunID: contracts.ID(runID.String()), EventType: contracts.EventRunCompleted, Sequence: 3, Timestamp: now,
		TraceID: traceID, RequestID: requestID, Data: []byte(`{"duration_ms":12}`),
	}}}
	controller := &Controller{
		runOwnerRepo: sseRunOwner{run: &entity.AgentRun{ID: runID}},
		agentEventRepo: sseEventRepo{events: []entity.AgentEvent{
			{RunID: runID.String(), Sequence: 1, EventType: string(contracts.EventRunStarted), Timestamp: now, Data: datatypes.JSON(`{}`)},
			{RunID: runID.String(), Sequence: 2, EventType: string(contracts.EventStageUpdated), Timestamp: now, Data: datatypes.JSON(`{"summary":"context ready"}`), TraceID: &traceID, RequestID: &requestID, Stage: &stage, Status: &status},
		}},
		eventSub: subscriber,
	}
	router := gin.New()
	router.GET("/api/v1/agent/runs/:id/events", func(c *gin.Context) {
		c.Set("auth_user", &entity.User{BaseEntity: entity.BaseEntity{ID: userID.String()}})
		controller.SubscribeEvents(c)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/agent/runs/"+runID.String()+"/events?after_sequence=0", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
	}
	body := recorder.Body.String()
	positions := []int{strings.Index(body, `"sequence":1`), strings.Index(body, `"sequence":2`), strings.Index(body, `"sequence":3`)}
	if positions[0] < 0 || positions[1] <= positions[0] || positions[2] <= positions[1] {
		t.Fatalf("events are not ordered 1,2,3: %s", body)
	}
	if subscriber.afterSequence != 2 {
		t.Fatalf("live subscription after sequence = %d, want 2", subscriber.afterSequence)
	}
	if !strings.Contains(body, `"trace_id":"`+traceID+`"`) || !strings.Contains(body, `"stage":"context_build"`) {
		t.Fatalf("replayed event lost correlation fields: %s", body)
	}
	if strings.Contains(body, "event: complete") {
		t.Fatalf("terminal event must end stream directly: %s", body)
	}
}

func TestSubscribeEventsStopsAtTerminalDatabaseEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	runID := uuid.New()
	now := time.Now().UTC()
	subscriber := &sseSubscriber{}
	controller := &Controller{
		runOwnerRepo: sseRunOwner{run: &entity.AgentRun{ID: runID}},
		agentEventRepo: sseEventRepo{events: []entity.AgentEvent{{
			RunID: runID.String(), Sequence: 4, EventType: string(contracts.EventRunFailed), Timestamp: now, Data: datatypes.JSON(`{"error_code":"MODEL_CALL_FAILED"}`),
		}}},
		eventSub: subscriber,
	}
	router := gin.New()
	router.GET("/events/:id", func(c *gin.Context) {
		c.Set("auth_user", &entity.User{BaseEntity: entity.BaseEntity{ID: userID.String()}})
		controller.SubscribeEvents(c)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/events/"+runID.String()+"?after_sequence=3", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"sequence":4`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if subscriber.afterSequence != 0 {
		t.Fatalf("subscriber must not be called after terminal history event")
	}
}
