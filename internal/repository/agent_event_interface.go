// Package repository 定义数据访问接口。AgentEventRepository 用于持久化 Agent 运行中间过程事件。
package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
)

// AgentEventRepository 定义 Agent 运行事件持久化接口。
type AgentEventRepository interface {
	// BatchCreate 批量创建事件记录，单次写入所有中间过程事件。
	BatchCreate(ctx context.Context, events []entity.AgentEvent) error

	// ListAfterSequence 查询指定运行在指定序号之后的事件，按 sequence 升序返回。
	ListAfterSequence(ctx context.Context, runID string, afterSequence int64) ([]entity.AgentEvent, error)

	// DeleteByRunID 删除指定运行的所有事件（级联清理）。
	DeleteByRunID(ctx context.Context, runID string) error
}
