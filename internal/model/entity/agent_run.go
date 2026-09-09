// Package entity 定义与数据库表一一对应的持久化实体。
package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentRun 表示 Agent 单次执行运行的数据库实体。
// 对应 000005_conversation_agent 中的 agent_runs 表，记录 Agent 执行的完整生命周期。
type AgentRun struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID              uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`                                                                                // 所属用户 ID
	KnowledgeBaseID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"knowledge_base_id"`                                                                      // 所属知识库 ID
	ConversationID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"conversation_id"`                                                                        // 关联会话 ID
	UserMessageID       uuid.UUID      `gorm:"type:uuid;not null" json:"user_message_id"`                                                                              // 触发本次运行的用户消息 ID
	AssistantMessageID  *uuid.UUID     `gorm:"type:uuid" json:"assistant_message_id,omitempty"`                                                                        // 运行完成后生成的助手消息 ID（可选）
	AgentConfigID       uuid.UUID      `gorm:"type:uuid;not null" json:"agent_config_id"`                                                                              // 运行的 Agent 配置 ID
	ChatModelID         uuid.UUID      `gorm:"type:uuid;not null" json:"chat_model_id"`                                                                                // 本次运行固化的 Chat 模型身份引用
	RetryOfRunID        *uuid.UUID     `gorm:"type:uuid" json:"retry_of_run_id,omitempty"`                                                                             // 若为重试运行，指向被重试的原始运行 ID（可选）
	Query               string         `gorm:"type:text;not null" json:"query"`                                                                                        // 用户本次查询的问题原文
	TraceID             *string        `gorm:"type:varchar(32)" json:"trace_id,omitempty"`                                                                             // 跨服务 Trace ID
	TraceParentSpanID   *string        `gorm:"type:varchar(16)" json:"trace_parent_span_id,omitempty"`                                                                 // 创建运行的 HTTP Span ID，用于异步调用树续接
	TraceSampled        bool           `gorm:"not null;default:true" json:"trace_sampled"`                                                                             // 创建运行时的采样决策，异步 Worker 必须保持一致
	RequestID           *string        `gorm:"type:varchar(128)" json:"request_id,omitempty"`                                                                          // 触发运行的 HTTP 请求 ID
	ExecutionMode       *string        `gorm:"type:varchar(20);check:execution_mode IN ('react')" json:"execution_mode,omitempty"`                                     // Router 选择的执行模式
	RouterReasonSummary string         `gorm:"type:varchar(1000)" json:"router_reason_summary,omitempty"`                                                              // Router 决策原因摘要（面向用户展示）
	RouterConfidence    *float64       `gorm:"type:numeric(5,4);check:router_confidence BETWEEN 0 AND 1" json:"router_confidence,omitempty"`                           // Router 决策置信度（可选）
	RouterFallbackUsed  bool           `gorm:"not null;default:false" json:"router_fallback_used"`                                                                     // Router 是否使用了兜底策略（如解析失败时默认 react）
	KnowledgeStatus     *string        `gorm:"type:varchar(20);check:knowledge_status IN ('sufficient','insufficient','ambiguous')" json:"knowledge_status,omitempty"` // 知识充分性状态
	ExecutionTrace      datatypes.JSON `gorm:"type:jsonb" json:"execution_trace,omitempty"`                                                                            // 执行轨迹摘要（JSON 格式，不保存完整思维链）
	MemoryUsedCount     int            `gorm:"not null;default:0;check:memory_used_count >= 0" json:"memory_used_count"`                                               // 本次运行使用的长期记忆条数
	Status              string         `gorm:"type:varchar(20);not null;default:'queued'" json:"status"`                                                               // 运行状态：queued/running/completed/failed/cancelled
	InputTokens         int            `gorm:"not null;default:0;check:input_tokens >= 0" json:"input_tokens"`                                                         // 总输入 Token 数
	OutputTokens        int            `gorm:"not null;default:0;check:output_tokens >= 0" json:"output_tokens"`                                                       // 总输出 Token 数
	TotalTokens         int            `gorm:"not null;default:0;check:total_tokens >= 0" json:"total_tokens"`                                                         // 总 Token 数
	DurationMs          *int64         `gorm:"check:duration_ms >= 0" json:"duration_ms,omitempty"`                                                                    // 执行耗时（毫秒）
	FirstTokenAt        *time.Time     `json:"first_token_at,omitempty"`
	FirstTokenLatencyMs *int64         `gorm:"check:first_token_latency_ms >= 0" json:"first_token_latency_ms,omitempty"`
	ModelGenerateMs     *int64         `gorm:"column:model_generate_duration_ms;check:model_generate_duration_ms >= 0" json:"model_generate_duration_ms,omitempty"`
	FinalResult         *string        `gorm:"type:text" json:"final_result,omitempty"`      // 最终回答结果
	ErrorCode           *string        `gorm:"type:varchar(64)" json:"error_code,omitempty"` // 失败时的错误码
	ErrorMessage        *string        `gorm:"type:text" json:"error_message,omitempty"`     // 失败时的错误信息
	FailureStage        *string        `gorm:"type:varchar(40)" json:"failure_stage,omitempty"`
	Retryable           *bool          `json:"retryable,omitempty"`
	RecoveryAdvice      *string        `gorm:"type:varchar(1000)" json:"recovery_advice,omitempty"`
	StartedAt           *time.Time     `json:"started_at,omitempty"`             // 执行开始时间
	EndedAt             *time.Time     `json:"ended_at,omitempty"`               // 执行结束时间
	CreatedAt           time.Time      `gorm:"autoCreateTime" json:"created_at"` // 创建时间
}

// TableName 指定 agent_runs 表名。
func (AgentRun) TableName() string {
	return "agent_runs"
}
