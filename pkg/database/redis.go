package database

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/redis/go-redis/v9"
)

// InitRedis 初始化Redis客户端并验证连接是否可用
func InitRedis(ctx context.Context, cfg *config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{Addr: cfg.Address, Password: cfg.Password, DB: cfg.DB, PoolSize: cfg.PoolSize})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

// CheckRedis 检查Redis连接是否健康
func CheckRedis(ctx context.Context, client *redis.Client) error {
	if client == nil {
		return fmt.Errorf("redis is not initialized")
	}
	return client.Ping(ctx).Err()
}

// CountWorkerHeartbeats 统计Redis中Worker心跳键的数量
func CountWorkerHeartbeats(ctx context.Context, client *redis.Client) (int64, error) {
	if client == nil {
		return 0, fmt.Errorf("redis is not initialized")
	}
	var cursor uint64
	var count int64
	for {
		keys, next, err := client.Scan(ctx, cursor, "worker:heartbeat:*", 100).Result()
		if err != nil {
			return 0, fmt.Errorf("scan worker heartbeats: %w", err)
		}
		count += int64(len(keys))
		cursor = next
		if cursor == 0 {
			return count, nil
		}
	}
}
