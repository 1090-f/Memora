// Package repository 实现 AgentEventRepository 的 GORM 数据访问层。
package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

// agentEventRepository 是 AgentEventRepository 接口的 GORM 实现。
type agentEventRepository struct {
	db *gorm.DB
}

// NewAgentEventRepository 创建 AgentEventRepository 实例。
func NewAgentEventRepository(db *gorm.DB) AgentEventRepository {
	return &agentEventRepository{db: db}
}

// BatchCreate 批量创建事件记录，使用单个 INSERT 语句写入。
func (r *agentEventRepository) BatchCreate(ctx context.Context, events []entity.AgentEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(events, 100).Error
}

// ListAfterSequence 查询指定运行在指定序号之后的事件，按 sequence 升序返回。
func (r *agentEventRepository) ListAfterSequence(ctx context.Context, runID string, afterSequence int64) ([]entity.AgentEvent, error) {
	var events []entity.AgentEvent
	err := r.db.WithContext(ctx).
		Where("run_id = ? AND sequence > ?", runID, afterSequence).
		Order("sequence ASC").
		Find(&events).Error
	return events, err
}

// DeleteByRunID 删除指定运行的所有事件。
func (r *agentEventRepository) DeleteByRunID(ctx context.Context, runID string) error {
	return r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Delete(&entity.AgentEvent{}).Error
}

// 编译时确保实现接口
var _ AgentEventRepository = (*agentEventRepository)(nil)
