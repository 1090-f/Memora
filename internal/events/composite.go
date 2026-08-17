// Package events 提供 CompositeEventPublisher，同时将 Agent 事件写入 Redis Stream 和 PostgreSQL。
package events

import (
	"context"
	"sync"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// CompositeEventPublisher 同时写入 Redis Stream（实时 SSE）和 PostgreSQL（持久化）。
// Redis 写入失败仅记录日志，不阻断流程；Postgres 写入失败的日志同样不阻断。
// 两个路径独立执行，任一失败不影响另一路的事件分发。
type CompositeEventPublisher struct {
	redisPub    contracts.EventPublisher // RedisEventPublisher 实例，用于实时 SSE 推送
	postgresPub contracts.EventPublisher // PostgresEventPublisher 实例，用于持久化存储
}

// NewCompositeEventPublisher 创建 CompositeEventPublisher 实例。
// redisPub 负责实时事件推送，postgresPub 负责持久化存储。
func NewCompositeEventPublisher(redisPub, postgresPub contracts.EventPublisher) *CompositeEventPublisher {
	return &CompositeEventPublisher{
		redisPub:    redisPub,
		postgresPub: postgresPub,
	}
}

// Publish 同时将事件写入 Redis 和 PG，使用 wg 并发执行以最小化延迟影响。
func (p *CompositeEventPublisher) Publish(ctx context.Context, event contracts.AgentEvent) error {
	var wg sync.WaitGroup
	var redisErr, pgErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := p.redisPub.Publish(ctx, event); err != nil {
			redisErr = err
			logger.Warn("Redis 事件发布失败（非阻塞）",
				zap.String("run_id", string(event.RunID)),
				zap.String("event_type", string(event.EventType)),
				zap.Int64("sequence", event.Sequence),
				zap.Error(err),
			)
		}
	}()
	go func() {
		defer wg.Done()
		if err := p.postgresPub.Publish(ctx, event); err != nil {
			pgErr = err
			logger.Warn("PG 事件持久化失败（非阻塞）",
				zap.String("run_id", string(event.RunID)),
				zap.String("event_type", string(event.EventType)),
				zap.Int64("sequence", event.Sequence),
				zap.Error(err),
			)
		}
	}()
	wg.Wait()

	// 两个路径都失败才返回错误，单个失败不阻断
	if redisErr != nil && pgErr != nil {
		return redisErr
	}
	return nil
}

// 编译时检查接口实现
var _ contracts.EventPublisher = (*CompositeEventPublisher)(nil)
