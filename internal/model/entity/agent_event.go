package entity

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AgentEvent 对应 agent_events 表的实体，持久化 Agent 运行过程中的中间事件。
// 每条记录对应一个 SSE 推送的 AgentEvent，用于会话切换/页面刷新后的状态重建。
type AgentEvent struct {
	ID        uint           `gorm:"primaryKey;autoIncrement"                             json:"id"`
	RunID     string         `gorm:"type:uuid;not null;index:idx_agent_events_run_id_seq" json:"run_id"`
	Sequence  int64          `gorm:"not null"                                             json:"sequence"`
	EventType string         `gorm:"type:varchar(64);not null"                            json:"event_type"`
	Timestamp time.Time      `gorm:"not null;default:now()"                               json:"timestamp"`
	Data      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"                     json:"data"`
	CreatedAt time.Time      `gorm:"not null;default:now()"                               json:"created_at"`
}

func (AgentEvent) TableName() string {
	return "agent_events"
}

// BeforeCreate GORM 钩子：写入前始终设置 Timestamp
func (e *AgentEvent) BeforeCreate(tx *gorm.DB) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	return nil
}
