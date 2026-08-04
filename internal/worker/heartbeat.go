package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const heartbeatPrefix = "worker:heartbeat:"

// Heartbeat 通过 Redis 定时写入心跳键实现 Worker 存活检测。
type Heartbeat struct {
	redis    *redis.Client
	key      string
	interval time.Duration
	ttl      time.Duration
}

// NewHeartbeat 创建一个新的心跳实例，基于主机名和 PID 生成唯一键。
func NewHeartbeat(client *redis.Client, interval, ttl time.Duration) (*Heartbeat, error) {
	if client == nil || interval <= 0 || ttl <= interval {
		return nil, fmt.Errorf("worker heartbeat client and valid durations are required")
	}
	hostname, _ := os.Hostname()
	key := fmt.Sprintf("%s%s-%d", heartbeatPrefix, hostname, os.Getpid())
	return &Heartbeat{redis: client, key: key, interval: interval, ttl: ttl}, nil
}

// Run 启动心跳循环，定期写入心跳键，上下文取消时自动清理。
func (h *Heartbeat) Run(ctx context.Context) error {
	h.beat(ctx)
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.redis.Del(cleanupCtx, h.key).Err()
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			h.beat(ctx)
		}
	}
}

func (h *Heartbeat) beat(ctx context.Context) {
	if err := h.redis.Set(ctx, h.key, time.Now().UTC().Format(time.RFC3339Nano), h.ttl).Err(); err != nil {
		logger.Error("worker heartbeat failed", zap.Error(err))
		return
	}
	metrics.WorkerHeartbeat()
}
