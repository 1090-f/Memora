package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/google/uuid"
)

// P2（顺序根治）的核心不变式：终态事件必须在状态落库之后发布，
// 否则前端收到 agent.run.completed 后立即查库会拿到 running + final_result=NULL。
// 本文件用记录调用次序的 fake 锁住该顺序，避免后续重构无意中调换。

const (
	p2ActionMarkCompleted = "repo.MarkCompleted"
	p2ActionMarkFailed    = "repo.MarkFailed"
	p2ActionRunCompleted  = "event.run.completed"
	p2ActionRunFailed     = "event.run.failed"
)

type p2OrderRecorder struct{ order []string }

func (r *p2OrderRecorder) firstIndexOf(target string) int {
	for i, action := range r.order {
		if action == target {
			return i
		}
	}
	return -1
}

// p2RunRepo 只实现 P2 关注的方法，其余方法靠嵌入接口占位（执行路径不会调用）。
type p2RunRepo struct {
	repository.AgentRunRepository
	rec *p2OrderRecorder
}

func (r p2RunRepo) MarkCompleted(context.Context, uuid.UUID, string, int, int, int, int64, string, string) error {
	r.rec.order = append(r.rec.order, p2ActionMarkCompleted)
	return nil
}

func (r p2RunRepo) MarkFailed(context.Context, uuid.UUID, string, string, string, int64, int, int, int) error {
	r.rec.order = append(r.rec.order, p2ActionMarkFailed)
	return nil
}

func (p2RunRepo) UpdateObservability(context.Context, uuid.UUID, repository.AgentRunObservabilityUpdate) error {
	return nil
}

func (p2RunRepo) SetAssistantMessageID(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type p2MessageRepo struct{ repository.MessageRepository }

func (p2MessageRepo) Create(context.Context, *entity.Message) error { return nil }

// p2EventPublisher 同时满足 contracts.EventPublisher 与 agentRunTerminalPublisher。
type p2EventPublisher struct{ rec *p2OrderRecorder }

func (p2EventPublisher) Publish(context.Context, contracts.AgentEvent) error { return nil }

func (p p2EventPublisher) PublishRunCompleted(context.Context, contracts.ID, contracts.AgentRunResult) error {
	p.rec.order = append(p.rec.order, p2ActionRunCompleted)
	return nil
}

func (p p2EventPublisher) PublishRunFailed(context.Context, contracts.ID, contracts.ExecutionMode, error) error {
	p.rec.order = append(p.rec.order, p2ActionRunFailed)
	return nil
}

// p2PublishOnlyPublisher 只实现 Publish，用于验证缺少终态发布能力时会安全跳过。
type p2PublishOnlyPublisher struct{ published int }

func (p *p2PublishOnlyPublisher) Publish(context.Context, contracts.AgentEvent) error {
	p.published++
	return nil
}

type p2ContextBuilder struct{ err error }

func (b p2ContextBuilder) Build(context.Context, contracts.AgentContextRequest) (contracts.AgentContext, error) {
	if b.err != nil {
		return contracts.AgentContext{}, b.err
	}
	return contracts.AgentContext{}, nil
}

type p2AgentService struct {
	result contracts.AgentRunResult
	err    error
}

func (s p2AgentService) Run(context.Context, contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	return s.result, s.err
}

func (p2AgentService) Cancel(context.Context, contracts.ID, contracts.ID) error { return nil }

func (p2AgentService) Retry(context.Context, contracts.ID, contracts.ID) (contracts.ID, error) {
	return "", nil
}

func newP2Worker(rec *p2OrderRecorder, svc contracts.AgentRunService, builder contracts.ContextBuilder) *AgentWorker {
	return &AgentWorker{
		agentService:   svc,
		runRepo:        p2RunRepo{rec: rec},
		messageRepo:    p2MessageRepo{},
		contextBuilder: builder,
		events:         p2EventPublisher{rec: rec},
		config:         DefaultAgentWorkerConfig(),
	}
}

func newP2Run() *entity.AgentRun {
	startedAt := time.Now().UTC()
	return &entity.AgentRun{
		ID:              uuid.New(),
		UserID:          uuid.New(),
		KnowledgeBaseID: uuid.New(),
		ConversationID:  uuid.New(),
		ChatModelID:     uuid.New(),
		Query:           "测试问题",
		Status:          "running",
		CreatedAt:       startedAt,
		StartedAt:       &startedAt,
	}
}

func assertOrder(t *testing.T, rec *p2OrderRecorder, wantFirst, wantSecond string) {
	t.Helper()
	first := rec.firstIndexOf(wantFirst)
	second := rec.firstIndexOf(wantSecond)
	if first < 0 {
		t.Fatalf("未观察到 %s，实际次序：%v", wantFirst, rec.order)
	}
	if second < 0 {
		t.Fatalf("未观察到 %s，实际次序：%v", wantSecond, rec.order)
	}
	if first > second {
		t.Fatalf("%s 必须早于 %s，实际次序：%v", wantFirst, wantSecond, rec.order)
	}
}

func TestTerminalCompletedEventIsPublishedAfterRunPersisted(t *testing.T) {
	rec := &p2OrderRecorder{}
	now := time.Now().UTC()
	worker := newP2Worker(rec, p2AgentService{result: contracts.AgentRunResult{
		FinalResult:   "正常回答",
		ExecutionMode: contracts.ExecutionReact,
		StartedAt:     now.Add(-time.Second),
		EndedAt:       now,
	}}, p2ContextBuilder{})

	worker.executeRun(context.Background(), newP2Run())

	assertOrder(t, rec, p2ActionMarkCompleted, p2ActionRunCompleted)
}

func TestTerminalFailedEventIsPublishedAfterEmptyAnswerPersisted(t *testing.T) {
	rec := &p2OrderRecorder{}
	now := time.Now().UTC()
	worker := newP2Worker(rec, p2AgentService{result: contracts.AgentRunResult{
		FinalResult:   "   ",
		ExecutionMode: contracts.ExecutionReact,
		StartedAt:     now.Add(-time.Second),
		EndedAt:       now,
	}}, p2ContextBuilder{})

	worker.executeRun(context.Background(), newP2Run())

	assertOrder(t, rec, p2ActionMarkFailed, p2ActionRunFailed)
}

func TestTerminalFailedEventIsPublishedWhenContextBuildFails(t *testing.T) {
	rec := &p2OrderRecorder{}
	worker := newP2Worker(rec, p2AgentService{}, p2ContextBuilder{err: errors.New("数据库不可用")})

	worker.executeRun(context.Background(), newP2Run())

	assertOrder(t, rec, p2ActionMarkFailed, p2ActionRunFailed)
}

func TestTerminalFailedEventIsPublishedWhenExecutionFails(t *testing.T) {
	rec := &p2OrderRecorder{}
	worker := newP2Worker(rec, p2AgentService{
		err: &contracts.AgentRunError{ExecutionMode: contracts.ExecutionReact, Err: errors.New("模型调用失败")},
	}, p2ContextBuilder{})

	worker.executeRun(context.Background(), newP2Run())

	assertOrder(t, rec, p2ActionMarkFailed, p2ActionRunFailed)
}

func TestTerminalPublishIsSkippedWhenPublisherLacksCapability(t *testing.T) {
	publisher := &p2PublishOnlyPublisher{}
	worker := &AgentWorker{events: publisher}

	worker.publishRunCompleted(context.Background(), contracts.ID("run-1"), contracts.AgentRunResult{})
	worker.publishRunFailed(context.Background(), contracts.ID("run-1"), contracts.ExecutionReact, errors.New("boom"))

	if publisher.published != 0 {
		t.Fatalf("缺少终态发布能力时不应发布事件，实际发布 %d 次", publisher.published)
	}
}

func TestTerminalPublishContextKeepsCorrelationButDetachesCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	execCtx := contracts.WithCorrelation(parent, "trace-1", "request-1")
	cancel() // 模拟运行已被取消：终态事件仍必须能发出去

	terminal := terminalPublishContext(execCtx)

	if err := terminal.Err(); err != nil {
		t.Fatalf("终态发布上下文不应继承取消信号，实际 err = %v", err)
	}
	traceID, requestID := contracts.CorrelationFromContext(terminal)
	if traceID != "trace-1" || requestID != "request-1" {
		t.Fatalf("终态发布上下文丢失关联字段：trace=%q request=%q", traceID, requestID)
	}
}
