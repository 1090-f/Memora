package database

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/pkg/config"
	"github.com/redis/go-redis/v9"
)

// InitRedis 初始化Redis客户端并验证连接是否可用
func InitRedis(ctx context.Context, cfg *config.RedisConfig) (*redis.Client, error) {
	client := NewRedisClient(cfg)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("Ping Redis 失败: %w", err)
	}
	return client, nil
}

// NewRedisClient 创建独立的 Redis 连接池。阻塞式 Stream 读取必须与鉴权等短请求隔离。
func NewRedisClient(cfg *config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: cfg.Address, Password: cfg.Password, DB: cfg.DB, PoolSize: cfg.PoolSize,
	})
}

// CheckRedis 检查Redis连接是否健康
func CheckRedis(ctx context.Context, client *redis.Client) error {
	if client == nil {
		return fmt.Errorf("Redis 尚未初始化")
	}
	return client.Ping(ctx).Err()
}

// CountWorkerHeartbeats 统计Redis中Worker心跳键的数量
func CountWorkerHeartbeats(ctx context.Context, client *redis.Client) (int64, error) {
	if client == nil {
		return 0, fmt.Errorf("Redis 尚未初始化")
	}
	var cursor uint64
	var count int64
	for {
		keys, next, err := client.Scan(ctx, cursor, "worker:heartbeat:*", 100).Result()
		if err != nil {
			return 0, fmt.Errorf("扫描 Worker 心跳失败: %w", err)
		}
		count += int64(len(keys))
		cursor = next
		if cursor == 0 {
			return count, nil
		}
	}
}
