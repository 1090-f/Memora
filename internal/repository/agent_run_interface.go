// Package repository 定义数据访问接口与实现。AgentRunRepository 用于 Agent 执行运行数据的持久化。
package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/google/uuid"
)

// AgentRunRepository 定义 Agent 运行记录的持久化接口。
// 方法参数中的 userID 用于强制所有者过滤，worker 使用的方法除外。
type AgentRunRepository interface {
	// CreateQueued 创建一条状态为 queued 的 Agent 运行记录。
	CreateQueued(ctx context.Context, run *entity.AgentRun) error

	// FindByID 根据运行 ID 和用户 ID 查找运行记录（强制所有者过滤）。
	FindByID(ctx context.Context, userID, runID uuid.UUID) (*entity.AgentRun, error)

	// FindByIDAdmin 根据运行 ID 直接查找（用于 Worker，跳过用户过滤）。
	FindByIDAdmin(ctx context.Context, runID uuid.UUID) (*entity.AgentRun, error)

	// ListByOwner 按用户、知识库分页查询运行记录，按创建时间降序排列。
	ListByOwner(ctx context.Context, userID, kbID uuid.UUID, page, pageSize int) ([]entity.AgentRun, int64, error)

	// ListQueued 获取所有 queued 状态的运行记录（用于 Worker 批量领取）。
	ListQueued(ctx context.Context, limit int) ([]entity.AgentRun, error)

	// ReserveQueued 原子地将一条 queued 记录标记为 running，返回运行记录或 nil。
	// 使用条件更新避免并发 Worker 重复领取。
	ReserveQueued(ctx context.Context, runID uuid.UUID) (*entity.AgentRun, error)

	// MarkRunning 更新运行状态为 running，设置开始时间。
	MarkRunning(ctx context.Context, runID uuid.UUID, startedAt string) error

	// MarkCompleted 更新运行状态为 completed，记录最终结果、Token 用量、耗时、执行模式、知识状态和结束时间。
	MarkCompleted(ctx context.Context, runID uuid.UUID, finalResult string, inputTokens, outputTokens, totalTokens int, durationMs int64, executionMode, knowledgeStatus string) error

	// MarkFailed 更新运行状态为 failed，记录错误码、错误信息、执行模式、Token 用量、耗时和结束时间。
	// inputTokens/outputTokens/totalTokens 用于记录失败前已消耗的 Token。
	MarkFailed(ctx context.Context, runID uuid.UUID, errorCode, errorMessage, executionMode string, durationMs int64, inputTokens, outputTokens, totalTokens int) error

	// MarkCancelled 更新运行状态为 cancelled（需同时验证用户 ID 确保所有者可取消）。
	MarkCancelled(ctx context.Context, userID, runID uuid.UUID) error

	// SetAssistantMessageID 设置运行记录的助手消息 ID（运行完成后调用）。
	SetAssistantMessageID(ctx context.Context, runID uuid.UUID, assistantMessageID uuid.UUID) error

	// MarkCancelledAdmin 直接按 runID 取消运行（用于 Worker 超时或内部取消）。
	MarkCancelledAdmin(ctx context.Context, runID uuid.UUID) error

	// CreateRetry 基于已有运行创建新的排队运行（retry_of_run_id 指向原运行），返回新运行 ID。
	CreateRetry(ctx context.Context, originalRunID, userID uuid.UUID) (uuid.UUID, error)

	// DeleteByConversationID 删除指定会话的所有 Agent 运行记录。
	DeleteByConversationID(ctx context.Context, conversationID uuid.UUID) error
}
