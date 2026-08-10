package background

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Manager 在 API 进程内发布 Outbox 并消费 Redis Stream 文档任务。
type Manager struct {
	redis     *redis.Client
	tasks     repository.ImportTaskRepository
	outbox    repository.TaskOutboxRepository
	processor service.DocumentProcessService
	consumer  config.DocumentConsumerConfig
	outboxCfg config.OutboxConfig
	name      string
}

func NewManager(redisClient *redis.Client, tasks repository.ImportTaskRepository, outbox repository.TaskOutboxRepository, processor service.DocumentProcessService, consumer config.DocumentConsumerConfig, outboxCfg config.OutboxConfig) *Manager {
	return &Manager{redis: redisClient, tasks: tasks, outbox: outbox, processor: processor, consumer: consumer, outboxCfg: outboxCfg, name: consumerName()}
}

func (m *Manager) Run(ctx context.Context) error {
	if !m.consumer.Enabled {
		<-ctx.Done()
		return nil
	}
	if err := m.ensureGroup(ctx); err != nil {
		return err
	}
	if count, err := m.tasks.RecoverStale(ctx, time.Now().UTC().Add(-m.consumer.ClaimIdle).Unix()); err != nil {
		return fmt.Errorf("恢复过期文档任务失败: %w", err)
	} else if count > 0 {
		logger.Info("已恢复过期文档任务", zap.Int64("count", count))
	}

	var wg sync.WaitGroup
	wg.Add(2 + m.consumer.Concurrency)
	go func() { defer wg.Done(); m.publishLoop(ctx) }()
	go func() { defer wg.Done(); m.reclaimLoop(ctx) }()
	for i := 0; i < m.consumer.Concurrency; i++ {
		workerName := fmt.Sprintf("%s-%d", m.name, i+1)
		go func() { defer wg.Done(); m.consumeLoop(ctx, workerName) }()
	}
	logger.Info("API 内嵌文档消费者已启动", zap.Int("concurrency", m.consumer.Concurrency), zap.String("stream", m.consumer.Stream))
	<-ctx.Done()
	wg.Wait()
	return nil
}

func (m *Manager) ensureGroup(ctx context.Context) error {
	err := m.redis.XGroupCreateMkStream(ctx, m.consumer.Stream, m.consumer.Group, "0").Err()
	if err != nil && !stringsContains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("创建 Redis Stream Consumer Group 失败: %w", err)
	}
	return nil
}

func (m *Manager) publishLoop(ctx context.Context) {
	ticker := time.NewTicker(m.outboxCfg.PollInterval)
	defer ticker.Stop()
	for {
		m.publishBatch(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) publishBatch(ctx context.Context) {
	events, err := m.outbox.ListUnpublished(ctx, m.outboxCfg.BatchSize)
	if err != nil {
		if ctx.Err() == nil {
			logger.Error("读取 Outbox 失败", zap.Error(err))
		}
		return
	}
	for _, event := range events {
		if ctx.Err() != nil {
			return
		}
		_, err := m.redis.XAdd(ctx, &redis.XAddArgs{Stream: m.consumer.Stream, MaxLen: 100000, Approx: true, Values: map[string]any{
			"event_id": event.ID, "event_type": event.EventType, "task_id": event.AggregateID,
		}}).Result()
		if err != nil {
			logger.Error("发布文档任务到 Redis Stream 失败", zap.String("event_id", event.ID), zap.Error(err))
			return
		}
		if err := m.outbox.MarkPublished(ctx, event.ID); err != nil {
			logger.Error("回写 Outbox 发布状态失败", zap.String("event_id", event.ID), zap.Error(err))
			return
		}
	}
}

func (m *Manager) consumeLoop(ctx context.Context, consumerName string) {
	for ctx.Err() == nil {
		streams, err := m.redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: m.consumer.Group, Consumer: consumerName, Streams: []string{m.consumer.Stream, ">"}, Count: 1, Block: m.consumer.BlockTimeout}).Result()
		if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
			continue
		}
		if err != nil {
			logger.Error("读取 Redis Stream 文档任务失败", zap.Error(err))
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				m.handleMessage(ctx, message)
			}
		}
	}
}

func (m *Manager) handleMessage(parent context.Context, message redis.XMessage) {
	taskID, ok := message.Values["task_id"].(string)
	if !ok || taskID == "" {
		logger.Error("Redis Stream 文档消息缺少 task_id", zap.String("message_id", message.ID))
		_ = m.redis.XAck(parent, m.consumer.Stream, m.consumer.Group, message.ID).Err()
		return
	}
	task, err := m.tasks.ClaimPendingByID(parent, taskID)
	if err != nil {
		logger.Error("认领 Redis Stream 文档任务失败", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task == nil {
		_ = m.redis.XAck(parent, m.consumer.Stream, m.consumer.Group, message.ID).Err()
		return
	}
	ctx, cancel := context.WithTimeout(parent, m.consumer.ProcessingTimeout)
	err = m.processor.ProcessImportTask(ctx, contracts.ID(taskID))
	cancel()
	if err != nil {
		if task.Attempt < m.consumer.MaxAttempts {
			if requeueErr := m.tasks.RequeueTask(parent, taskID, err.Error()); requeueErr != nil {
				logger.Error("重新排队文档任务失败", zap.String("task_id", taskID), zap.Error(requeueErr))
				return
			}
		} else if failErr := m.tasks.FailTask(parent, taskID, err.Error()); failErr != nil {
			logger.Error("标记文档任务失败", zap.String("task_id", taskID), zap.Error(failErr))
			return
		}
	}
	if err := m.redis.XAck(parent, m.consumer.Stream, m.consumer.Group, message.ID).Err(); err != nil && parent.Err() == nil {
		logger.Error("确认 Redis Stream 文档任务失败", zap.String("task_id", taskID), zap.Error(err))
	}
}

func (m *Manager) reclaimLoop(ctx context.Context) {
	interval := m.consumer.ClaimIdle / 2
	if interval > time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.tasks.RecoverStale(ctx, time.Now().UTC().Add(-m.consumer.ClaimIdle).Unix())
			messages, _, err := m.redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: m.consumer.Stream, Group: m.consumer.Group, Consumer: m.name + "-reclaimer", MinIdle: m.consumer.ClaimIdle, Start: "0-0", Count: int64(m.consumer.Concurrency)}).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				logger.Error("回收 Redis Stream Pending 文档任务失败", zap.Error(err))
				continue
			}
			for _, message := range messages {
				m.handleMessage(ctx, message)
			}
		}
	}
}

func consumerName() string {
	host, _ := os.Hostname()
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), hex.EncodeToString(buf))
}

func stringsContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
