package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const idempotencyPrefix = "worker:idempotency:"

// RedisIdempotencyStore 是基于 Redis 的幂等性存储实现。
type RedisIdempotencyStore struct{ redis *redis.Client }

// NewRedisIdempotencyStore 创建一个新的 Redis 幂等性存储实例。
func NewRedisIdempotencyStore(client *redis.Client) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{redis: client}
}

// Claim 尝试抢占指定键的幂等性锁，成功返回 true。
func (s *RedisIdempotencyStore) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	claimed, err := s.redis.SetNX(ctx, idempotencyPrefix+key, "running", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("获取 Worker 幂等键失败: %w", err)
	}
	return claimed, nil
}

// Complete 将指定键标记为已完成。
func (s *RedisIdempotencyStore) Complete(ctx context.Context, key string, ttl time.Duration) error {
	if err := s.redis.Set(ctx, idempotencyPrefix+key, "completed", ttl).Err(); err != nil {
		return fmt.Errorf("将 Worker 幂等键标记为完成失败: %w", err)
	}
	return nil
}

// Release 释放指定键的幂等性锁。
func (s *RedisIdempotencyStore) Release(ctx context.Context, key string) error {
	if err := s.redis.Del(ctx, idempotencyPrefix+key).Err(); err != nil {
		return fmt.Errorf("释放 Worker 幂等键失败: %w", err)
	}
	return nil
}
