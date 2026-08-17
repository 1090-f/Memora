// Package events 提供基于 PostgreSQL 的 Agent 事件持久化发布实现。
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"gorm.io/datatypes"
)

// PostgresEventPublisher 实现 contracts.EventPublisher 接口，将 Agent 事件持久化写入 agent_events 表。
// 每次 Publish 直接写入 DB，与 RedisEventPublisher 构成双写链路。
type PostgresEventPublisher struct {
	eventRepo repository.AgentEventRepository
}

// NewPostgresEventPublisher 创建 PostgresEventPublisher 实例。
func NewPostgresEventPublisher(eventRepo repository.AgentEventRepository) *PostgresEventPublisher {
	return &PostgresEventPublisher{eventRepo: eventRepo}
}

// Publish 将 Agent 事件写入 agent_events 表。
// 如果事件数据为空，使用空 JSON 对象 {} 代替。
func (p *PostgresEventPublisher) Publish(ctx context.Context, event contracts.AgentEvent) error {
	data := event.Data
	if len(data) == 0 {
		data = json.RawMessage("{}")
	}
	ent := entity.AgentEvent{
		RunID:     string(event.RunID),
		Sequence:  event.Sequence,
		EventType: string(event.EventType),
		Timestamp: event.Timestamp,
		Data:      datatypes.JSON(data),
	}
	if event.Timestamp.IsZero() {
		ent.Timestamp = time.Now().UTC()
	}
	return p.eventRepo.BatchCreate(ctx, []entity.AgentEvent{ent})
}

// 编译时检查接口实现
var _ contracts.EventPublisher = (*PostgresEventPublisher)(nil)
