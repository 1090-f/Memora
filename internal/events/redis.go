// Package events 提供基于 Redis 的 Agent 事件发布与订阅实现。
// EventPublisher 使用 Redis Pub/Sub 发布事件，EventSubscriber 通过 Redis Pub/Sub 订阅事件。
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/redis/go-redis/v9"
)

// EventChannel 是 Agent 事件发布的 Redis Pub/Sub 频道名称。
const EventChannel = "agent:events"

// RedisEventPublisher 实现 contracts.EventPublisher 接口，将 Agent 事件发布到 Redis Pub/Sub。
// 每条事件被序列化为 JSON 后发送到 agent:events 频道。
type RedisEventPublisher struct {
	redis *redis.Client // Redis 客户端连接
}

// NewRedisEventPublisher 创建基于 Redis 的 EventPublisher 实例。
// redisClient 是已连接的 Redis 客户端，用于执行 PUBLISH 命令。
func NewRedisEventPublisher(redisClient *redis.Client) *RedisEventPublisher {
	return &RedisEventPublisher{redis: redisClient}
}

// Publish 将 Agent 事件序列化为 JSON 并发布到 Redis Pub/Sub 频道。
// 如果序列化失败或 Redis 发布失败，返回错误。
func (p *RedisEventPublisher) Publish(ctx context.Context, event contracts.AgentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化 Agent 事件失败: %w", err)
	}

	err = p.redis.Publish(ctx, EventChannel, data).Err()
	if err != nil {
		return fmt.Errorf("发布 Agent 事件到 Redis 失败: %w", err)
	}
	return nil
}

// RedisEventSubscriber 实现 contracts.EventSubscriber 接口，通过 Redis Pub/Sub 订阅 Agent 事件。
// 订阅者按 runID 过滤事件，支持断线重连和指定起始序列号的回放。
type RedisEventSubscriber struct {
	redis *redis.Client // Redis 客户端连接
}

// NewRedisEventSubscriber 创建基于 Redis 的 EventSubscriber 实例。
func NewRedisEventSubscriber(redisClient *redis.Client) *RedisEventSubscriber {
	return &RedisEventSubscriber{redis: redisClient}
}

// Subscribe 返回一个通道，用于接收指定运行 RunID 在 afterSequence 之后的事件。
// 客户端通过该通道持续接收实时事件，直到上下文被取消。
// afterSequence 参数用于断线重连时跳过已处理的事件。
func (s *RedisEventSubscriber) Subscribe(ctx context.Context, runID contracts.ID, afterSequence int64) (<-chan contracts.AgentEvent, error) {
	// 订阅 Redis Pub/Sub 频道
	pubSub := s.redis.Subscribe(ctx, EventChannel)

	// 等待订阅确认，确保通道就绪后才开始消费
	_, err := pubSub.Receive(ctx)
	if err != nil {
		// 订阅失败，关闭连接并返回错误
		pubSub.Close()
		return nil, fmt.Errorf("订阅 Redis 频道失败: %w", err)
	}

	// 创建带缓冲的事件通道，防止发布者因订阅者消费慢而阻塞
	ch := make(chan contracts.AgentEvent, 128)

	// 启动后台 goroutine 持续消费 Redis 消息
	go func() {
		defer close(ch)      // 退出时关闭事件通道，通知消费者结束
		defer pubSub.Close() // 退出时关闭 Redis 订阅连接

		// Channel() 返回一个 channel，当 Redis 收到消息时会投递过来
		redisCh := pubSub.Channel()
		for {
			select {
			case <-ctx.Done():
				// 客户端取消订阅，正常退出
				return
			case msg, ok := <-redisCh:
				if !ok {
					// Redis 订阅通道已关闭（连接断开），退出
					return
				}

				var event contracts.AgentEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					// 反序列化失败，跳过该消息（不影响其他消息的消费）
					continue
				}

				// 按 runID 过滤：只返回匹配运行的事件
				if event.RunID != runID {
					continue
				}

				// 按起始序列号过滤：跳过客户端已处理的事件
				if event.Sequence <= afterSequence {
					continue
				}

				select {
				case ch <- event:
				case <-ctx.Done():
					// 取消时不再尝试发送
					return
				}
			}
		}
	}()

	return ch, nil
}

// 编译时检查接口实现
var _ contracts.EventPublisher = (*RedisEventPublisher)(nil)
var _ contracts.EventSubscriber = (*RedisEventSubscriber)(nil)
