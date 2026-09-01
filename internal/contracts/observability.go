package contracts

import (
	"context"
	"time"
)

type correlationContextKey string
type agentStageReporterContextKey struct{}

const (
	traceIDContextKey   correlationContextKey = "memora.trace_id"
	requestIDContextKey correlationContextKey = "memora.request_id"
)

func WithCorrelation(ctx context.Context, traceID, requestID string) context.Context {
	if traceID != "" {
		ctx = context.WithValue(ctx, traceIDContextKey, traceID)
	}
	if requestID != "" {
		ctx = context.WithValue(ctx, requestIDContextKey, requestID)
	}
	return ctx
}

func CorrelationFromContext(ctx context.Context) (traceID, requestID string) {
	if ctx == nil {
		return "", ""
	}
	traceID, _ = ctx.Value(traceIDContextKey).(string)
	requestID, _ = ctx.Value(requestIDContextKey).(string)
	return traceID, requestID
}

// AgentStageReporter 将底层检索节点的安全阶段摘要上报给运行事件发布器。
type AgentStageReporter func(context.Context, AgentStage, StageStatus, int64, string, map[string]any)

func WithAgentStageReporter(ctx context.Context, reporter AgentStageReporter) context.Context {
	if reporter == nil {
		return ctx
	}
	return context.WithValue(ctx, agentStageReporterContextKey{}, reporter)
}

func ReportAgentStage(ctx context.Context, stage AgentStage, status StageStatus, durationMS int64, summary string, metadata map[string]any) {
	if ctx == nil {
		return
	}
	if reporter, ok := ctx.Value(agentStageReporterContextKey{}).(AgentStageReporter); ok {
		reporter(ctx, stage, status, durationMS, summary, metadata)
	}
}

// StageStatus 是文档与问答阶段共用的稳定状态语义。
type StageStatus string

const (
	StagePending   StageStatus = "pending"
	StageRunning   StageStatus = "running"
	StageSucceeded StageStatus = "succeeded"
	StageFailed    StageStatus = "failed"
	StageSkipped   StageStatus = "skipped"
)

// DocumentStage 是面向产品与持久化事件的文档处理阶段。
type DocumentStage string

const (
	DocumentStageUpload    DocumentStage = "upload"
	DocumentStageParse     DocumentStage = "parse"
	DocumentStageNormalize DocumentStage = "normalize"
	DocumentStageChunk     DocumentStage = "chunk"
	DocumentStageEmbed     DocumentStage = "embed"
	DocumentStageIndex     DocumentStage = "index"
	DocumentStagePreview   DocumentStage = "preview"
)

// AgentStage 是问答运行的稳定产品阶段；内部 Agent 事件可以继续提供更细粒度信息。
type AgentStage string

const (
	AgentStageRoute           AgentStage = "route"
	AgentStageQueryRewrite    AgentStage = "query_rewrite"
	AgentStageKeywordRetrieve AgentStage = "keyword_retrieve"
	AgentStageVectorRetrieve  AgentStage = "vector_retrieve"
	AgentStageFusion          AgentStage = "fusion"
	AgentStageRerank          AgentStage = "rerank"
	AgentStageKnowledgeCheck  AgentStage = "knowledge_check"
	AgentStageContextBuild    AgentStage = "context_build"
	AgentStageModelGenerate   AgentStage = "model_generate"
	AgentStageToolCall        AgentStage = "tool_call"
	AgentStageAnswer          AgentStage = "answer"
)

// StageObservation 是跨 API、事件与前端共享的安全阶段摘要。
// Metadata 只允许调用方写入经过白名单筛选且不含正文、Prompt 或凭据的低敏字段。
type StageObservation struct {
	Stage        string         `json:"stage"`
	Status       StageStatus    `json:"status"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	EndedAt      *time.Time     `json:"ended_at,omitempty"`
	DurationMS   *int64         `json:"duration_ms,omitempty"`
	ErrorCode    ErrorCode      `json:"error_code,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}
