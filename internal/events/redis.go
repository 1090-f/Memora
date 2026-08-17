// Package events 提供基于 Redis Stream 的 Agent 事件发布与订阅实现。
// 每个 Agent Run 使用独立 Stream，既可回放历史事件，也可继续等待实时事件。
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/redis/go-redis/v9"
)

const (
	eventStreamPrefix   = "agent:events:"
	eventSequenceSuffix = ":sequence"
	eventStreamMaxLen   = 5000
	eventStreamTTL      = 7 * 24 * time.Hour
	eventReadBlock      = 5 * time.Second
)

var appendEventScript = redis.NewScript(`
local sequence = redis.call("INCR", KEYS[2])
local event = cjson.decode(ARGV[1])
event["sequence"] = sequence
local encoded = cjson.encode(event)
redis.call("XADD", KEYS[1], "MAXLEN", "~", ARGV[2], "*", "event", encoded)
redis.call("EXPIRE", KEYS[1], ARGV[3])
redis.call("EXPIRE", KEYS[2], ARGV[3])
return sequence
`)

func eventStreamKey(runID contracts.ID) string {
	return eventStreamPrefix + string(runID)
}

func eventSequenceKey(runID contracts.ID) string {
	return eventStreamKey(runID) + eventSequenceSuffix
}

// RedisEventPublisher 实现 contracts.EventPublisher 接口，将 Agent 事件追加到按 Run 隔离的 Redis Stream。
type RedisEventPublisher struct {
	redis *redis.Client // Redis 客户端连接
}

// NewRedisEventPublisher 创建基于 Redis 的 EventPublisher 实例。
// redisClient 是已连接的 Redis 客户端，用于执行 XADD 命令。
func NewRedisEventPublisher(redisClient *redis.Client) *RedisEventPublisher {
	return &RedisEventPublisher{redis: redisClient}
}

// Publish 将 Agent 事件序列化后追加到 Redis Stream，并刷新事件历史的保留期限。
func (p *RedisEventPublisher) Publish(ctx context.Context, event contracts.AgentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化 Agent 事件失败: %w", err)
	}

	_, err = appendEventScript.Run(
		ctx,
		p.redis,
		[]string{eventStreamKey(event.RunID), eventSequenceKey(event.RunID)},
		data,
		eventStreamMaxLen,
		int64(eventStreamTTL/time.Second),
	).Result()
	if err != nil {
		return fmt.Errorf("写入 Agent 事件到 Redis Stream 失败: %w", err)
	}
	return nil
}

// RedisEventSubscriber 实现 contracts.EventSubscriber 接口，从 Redis Stream 回放并持续读取 Agent 事件。
type RedisEventSubscriber struct {
	redis *redis.Client // Redis 客户端连接
}

// NewRedisEventSubscriber 创建基于 Redis 的 EventSubscriber 实例。
func NewRedisEventSubscriber(redisClient *redis.Client) *RedisEventSubscriber {
	return &RedisEventSubscriber{redis: redisClient}
}

// Subscribe 从 Stream 起点开始读取，以 sequence 过滤已消费事件；读完历史后 XREAD 会继续阻塞等待新事件。
// 历史读取与实时等待共享同一个 Stream cursor，因此两者之间不存在丢事件窗口。
func (s *RedisEventSubscriber) Subscribe(ctx context.Context, runID contracts.ID, afterSequence int64) (<-chan contracts.AgentEvent, error) {
	if err := s.redis.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis 事件流失败: %w", err)
	}

	ch := make(chan contracts.AgentEvent, 128)
	key := eventStreamKey(runID)

	go func() {
		defer close(ch)
		cursor := "0-0"
		for {
			streams, err := s.redis.XRead(ctx, &redis.XReadArgs{
				Streams: []string{key, cursor},
				Count:   128,
				Block:   eventReadBlock,
			}).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				return
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					cursor = message.ID
					raw, ok := message.Values["event"]
					if !ok {
						continue
					}
					var data []byte
					switch value := raw.(type) {
					case string:
						data = []byte(value)
					case []byte:
						data = value
					default:
						continue
					}
					var event contracts.AgentEvent
					if err := json.Unmarshal(data, &event); err != nil || event.RunID != runID || event.Sequence <= afterSequence {
						continue
					}
					select {
					case ch <- event:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}

// 编译时检查接口实现
var _ contracts.EventPublisher = (*RedisEventPublisher)(nil)
var _ contracts.EventSubscriber = (*RedisEventSubscriber)(nil)
