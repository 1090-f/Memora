package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const idempotencyPrefix = "worker:idempotency:"

type RedisIdempotencyStore struct{ redis *redis.Client }

func NewRedisIdempotencyStore(client *redis.Client) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{redis: client}
}

func (s *RedisIdempotencyStore) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	claimed, err := s.redis.SetNX(ctx, idempotencyPrefix+key, "running", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("claim worker idempotency key: %w", err)
	}
	return claimed, nil
}

func (s *RedisIdempotencyStore) Complete(ctx context.Context, key string, ttl time.Duration) error {
	if err := s.redis.Set(ctx, idempotencyPrefix+key, "completed", ttl).Err(); err != nil {
		return fmt.Errorf("complete worker idempotency key: %w", err)
	}
	return nil
}

func (s *RedisIdempotencyStore) Release(ctx context.Context, key string) error {
	if err := s.redis.Del(ctx, idempotencyPrefix+key).Err(); err != nil {
		return fmt.Errorf("release worker idempotency key: %w", err)
	}
	return nil
}
