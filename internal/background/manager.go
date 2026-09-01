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
	previewservice "github.com/1090-f/Memora/internal/service/preview"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/metrics"
	appobservability "github.com/1090-f/Memora/pkg/observability"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

// Manager 在 API 进程内发布 Outbox 并消费 Redis Stream 文档任务。
type Manager struct {
	redis            *redis.Client
	tasks            repository.ImportTaskRepository
	outbox           repository.TaskOutboxRepository
	processor        service.DocumentProcessService
	previews         repository.DocumentPreviewRepository
	previewProcessor previewservice.Processor
	consumer         config.DocumentConsumerConfig
	previewConsumer  config.PreviewConsumerConfig
	previewEnabled   bool
	outboxCfg        config.OutboxConfig
	cleanup          config.IndexCleanupConfig
	retention        repository.ObservabilityRetentionRepository
	observability    config.ObservabilityConfig
	name             string
}

func NewManager(
	redisClient *redis.Client,
	tasks repository.ImportTaskRepository,
	previews repository.DocumentPreviewRepository,
	outbox repository.TaskOutboxRepository,
	processor service.DocumentProcessService,
	previewProcessor previewservice.Processor,
	retention repository.ObservabilityRetentionRepository,
	consumer config.DocumentConsumerConfig,
	previewCfg config.PreviewConfig,
	outboxCfg config.OutboxConfig,
	cleanupCfg config.IndexCleanupConfig,
	observabilityCfg config.ObservabilityConfig,
) *Manager {
	return &Manager{
		redis: redisClient, tasks: tasks, previews: previews, outbox: outbox,
		processor: processor, previewProcessor: previewProcessor,
		consumer: consumer, previewConsumer: previewCfg.Consumer, previewEnabled: previewCfg.Enabled,
		outboxCfg: outboxCfg, cleanup: cleanupCfg, retention: retention, observability: observabilityCfg, name: consumerName(),
	}
}

func (m *Manager) Run(ctx context.Context) error {
	retentionEnabled := m.observability.Enabled && m.observability.RetentionDays > 0 && m.retention != nil
	if !m.consumer.Enabled && !m.previewEnabled && !m.cleanup.Enabled && !retentionEnabled {
		<-ctx.Done()
		return nil
	}
	if m.consumer.Enabled {
		if err := m.ensureGroup(ctx, m.consumer.Stream, m.consumer.Group); err != nil {
			return err
		}
	}
	if m.previewEnabled {
		if err := m.ensureGroup(ctx, m.previewConsumer.Stream, m.previewConsumer.Group); err != nil {
			return err
		}
	}
	if m.consumer.Enabled {
		if count, err := m.tasks.RecoverStale(ctx, time.Now().UTC().Add(-m.consumer.ClaimIdle).Unix()); err != nil {
			return fmt.Errorf("恢复过期文档任务失败: %w", err)
		} else if count > 0 {
			logger.Info("已恢复过期文档任务", zap.Int64("count", count))
		}
	}
	if m.previewEnabled && m.previews != nil {
		if count, err := m.previews.RecoverStale(ctx, time.Now().UTC().Add(-m.previewConsumer.ClaimIdle)); err != nil {
			return fmt.Errorf("恢复过期预览任务失败: %w", err)
		} else if count > 0 {
			logger.Info("已恢复过期预览任务", zap.Int64("count", count))
		}
	}

	var wg sync.WaitGroup
	workerCount := 2 // outbox publisher + independent heartbeat
	if m.consumer.Enabled {
		workerCount += 1 + m.consumer.Concurrency
	}
	if m.previewEnabled {
		workerCount += 1 + m.previewConsumer.Concurrency
	}
	if m.cleanup.Enabled {
		workerCount++
	}
	if retentionEnabled {
		workerCount++
	}
	wg.Add(workerCount)
	go func() { defer wg.Done(); m.publishLoop(ctx) }()
	go func() { defer wg.Done(); m.heartbeatLoop(ctx) }()
	if m.cleanup.Enabled {
		go func() { defer wg.Done(); m.cleanupLoop(ctx) }()
	}
	if retentionEnabled {
		go func() { defer wg.Done(); m.retentionLoop(ctx) }()
	}
	if m.consumer.Enabled {
		go func() { defer wg.Done(); m.reclaimLoop(ctx) }()
		for i := 0; i < m.consumer.Concurrency; i++ {
			workerName := fmt.Sprintf("%s-%d", m.name, i+1)
			go func() { defer wg.Done(); m.consumeLoop(ctx, workerName) }()
		}
		logger.Info("API 内嵌文档消费者已启动", zap.Int("concurrency", m.consumer.Concurrency), zap.String("stream", m.consumer.Stream))
	}
	if m.previewEnabled {
		go func() { defer wg.Done(); m.previewReclaimLoop(ctx) }()
		for i := 0; i < m.previewConsumer.Concurrency; i++ {
			workerName := fmt.Sprintf("%s-preview-%d", m.name, i+1)
			go func() { defer wg.Done(); m.previewConsumeLoop(ctx, workerName) }()
		}
		logger.Info("API 内嵌预览消费者已启动", zap.Int("concurrency", m.previewConsumer.Concurrency), zap.String("stream", m.previewConsumer.Stream))
	}
	<-ctx.Done()
	wg.Wait()
	return nil
}

func (m *Manager) ensureGroup(ctx context.Context, stream, group string) error {
	err := m.redis.XGroupCreateMkStream(ctx, stream, group, "0").Err()
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

func (m *Manager) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		metrics.WorkerHeartbeat()
		if err := m.redis.Set(ctx, "worker:heartbeat:"+m.name, time.Now().UTC().Unix(), 30*time.Second).Err(); err != nil && ctx.Err() == nil {
			logger.Warn("更新 Worker 心跳失败", zap.String("worker", m.name), zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) retentionLoop(ctx context.Context) {
	run := func() {
		cutoff := time.Now().UTC().AddDate(0, 0, -m.observability.RetentionDays)
		cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		deleted, err := m.retention.DeleteBefore(cleanupCtx, cutoff)
		if err != nil {
			if ctx.Err() == nil {
				logger.Error("清理过期可观测事件失败", zap.Error(err))
			}
			return
		}
		if deleted > 0 {
			logger.Info("过期可观测事件清理完成", zap.Int64("deleted", deleted), zap.Int("retention_days", m.observability.RetentionDays))
		}
	}
	run()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

// cleanupLoop 周期性清理旧索引版本与已删除文档的 Chunk/向量。
func (m *Manager) cleanupLoop(ctx context.Context) {
	interval := m.cleanup.Interval
	if interval <= 0 {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runCleanup(ctx)
		}
	}
}

// runCleanup 执行一次旧索引清理，带独立超时，失败仅记录日志不影响其他后台任务。
func (m *Manager) runCleanup(ctx context.Context) {
	if m.processor == nil {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if n, err := m.processor.CleanupInactiveIndexes(cleanupCtx, m.cleanup.Retention); err != nil {
		logger.Error("旧索引版本清理失败", zap.Error(err))
	} else if n > 0 {
		logger.Info("旧索引版本清理完成", zap.Int64("deleted", n), zap.Int("retention", m.cleanup.Retention))
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
	metrics.QueueDepth("document", "outbox_unpublished_batch", int64(len(events)))
	if m.consumer.Enabled {
		if pending, pendingErr := m.redis.XPending(ctx, m.consumer.Stream, m.consumer.Group).Result(); pendingErr == nil {
			metrics.QueueDepth("document_process", "redis_pending", pending.Count)
		}
	}
	if m.previewEnabled {
		if pending, pendingErr := m.redis.XPending(ctx, m.previewConsumer.Stream, m.previewConsumer.Group).Result(); pendingErr == nil {
			metrics.QueueDepth("document_preview", "redis_pending", pending.Count)
		}
	}
	for _, event := range events {
		if ctx.Err() != nil {
			return
		}
		stream, idField := "", ""
		switch event.EventType {
		case "document.parse":
			if m.consumer.Enabled {
				stream, idField = m.consumer.Stream, "task_id"
			}
		case repository.PreviewRenderEventType:
			if m.previewEnabled {
				stream, idField = m.previewConsumer.Stream, "preview_id"
			}
		default:
			logger.Error("未知 Outbox 事件类型，保留待发布", zap.String("event_id", event.ID), zap.String("event_type", event.EventType))
			continue
		}
		if stream == "" {
			logger.Warn("Outbox 对应消费者未启用，保留待发布", zap.String("event_id", event.ID), zap.String("event_type", event.EventType))
			continue
		}
		values := map[string]any{"event_id": event.ID, "event_type": event.EventType, idField: event.AggregateID}
		_, err := m.redis.XAdd(ctx, &redis.XAddArgs{Stream: stream, MaxLen: 100000, Approx: true, Values: values}).Result()
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

func (m *Manager) previewConsumeLoop(ctx context.Context, consumerName string) {
	for ctx.Err() == nil {
		streams, err := m.redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: m.previewConsumer.Group, Consumer: consumerName, Streams: []string{m.previewConsumer.Stream, ">"}, Count: 1, Block: m.previewConsumer.BlockTimeout}).Result()
		if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
			continue
		}
		if err != nil {
			logger.Error("读取 Redis Stream 预览任务失败", zap.Error(err))
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				m.handlePreviewMessage(ctx, message)
			}
		}
	}
}

func (m *Manager) handlePreviewMessage(parent context.Context, message redis.XMessage) {
	startedAt := time.Now()
	result := "succeeded"
	defer func() { metrics.WorkerFinished("document_preview", result, time.Since(startedAt)) }()
	previewID, ok := message.Values["preview_id"].(string)
	if !ok || previewID == "" {
		result = "invalid"
		logger.Error("Redis Stream 预览消息缺少 preview_id", zap.String("message_id", message.ID))
		_ = m.redis.XAck(parent, m.previewConsumer.Stream, m.previewConsumer.Group, message.ID).Err()
		return
	}
	if m.previews == nil || m.previewProcessor == nil {
		logger.Error("预览消费者依赖未配置", zap.String("preview_id", previewID))
		return
	}
	item, err := m.previews.ClaimPendingByID(parent, previewID)
	if err != nil {
		result = "failed"
		logger.Error("认领预览任务失败", zap.String("preview_id", previewID), zap.Error(err))
		return
	}
	if item == nil {
		_ = m.redis.XAck(parent, m.previewConsumer.Stream, m.previewConsumer.Group, message.ID).Err()
		return
	}
	metrics.StageFinished("document_preview", "queue_wait", "succeeded", time.Since(item.CreatedAt))
	ctx, cancel := context.WithTimeout(parent, m.previewConsumer.ProcessingTimeout)
	err = m.previewProcessor.Process(ctx, previewID)
	cancel()
	if err != nil {
		result = "failed"
		code := string(previewservice.ErrorCode(err))
		if item.Attempt < m.previewConsumer.MaxAttempts {
			if requeueErr := m.previews.Requeue(parent, previewID, code, err.Error()); requeueErr != nil {
				logger.Error("重新排队预览任务失败", zap.String("preview_id", previewID), zap.Error(requeueErr))
				return
			}
		} else if failErr := m.previews.MarkFailed(parent, previewID, code, err.Error()); failErr != nil {
			logger.Error("标记预览任务失败", zap.String("preview_id", previewID), zap.Error(failErr))
			return
		}
	}
	if err := m.redis.XAck(parent, m.previewConsumer.Stream, m.previewConsumer.Group, message.ID).Err(); err != nil && parent.Err() == nil {
		logger.Error("确认 Redis Stream 预览任务失败", zap.String("preview_id", previewID), zap.Error(err))
	}
}

func (m *Manager) previewReclaimLoop(ctx context.Context) {
	interval := m.previewConsumer.ClaimIdle / 2
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
			if m.previews != nil {
				_, _ = m.previews.RecoverStale(ctx, time.Now().UTC().Add(-m.previewConsumer.ClaimIdle))
			}
			messages, _, err := m.redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: m.previewConsumer.Stream, Group: m.previewConsumer.Group, Consumer: m.name + "-preview-reclaimer", MinIdle: m.previewConsumer.ClaimIdle, Start: "0-0", Count: int64(m.previewConsumer.Concurrency)}).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				logger.Error("回收 Redis Stream Pending 预览任务失败", zap.Error(err))
				continue
			}
			for _, message := range messages {
				m.handlePreviewMessage(ctx, message)
			}
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
	startedAt := time.Now()
	result := "succeeded"
	defer func() { metrics.WorkerFinished("document_process", result, time.Since(startedAt)) }()
	taskID, ok := message.Values["task_id"].(string)
	if !ok || taskID == "" {
		result = "invalid"
		logger.Error("Redis Stream 文档消息缺少 task_id", zap.String("message_id", message.ID))
		_ = m.redis.XAck(parent, m.consumer.Stream, m.consumer.Group, message.ID).Err()
		return
	}
	task, err := m.tasks.ClaimPendingByID(parent, taskID)
	if err != nil {
		result = "failed"
		logger.Error("认领 Redis Stream 文档任务失败", zap.String("task_id", taskID), zap.Error(err))
		return
	}
	if task == nil {
		_ = m.redis.XAck(parent, m.consumer.Stream, m.consumer.Group, message.ID).Err()
		return
	}
	metrics.StageFinished("document_process", "queue_wait", "succeeded", time.Since(task.CreatedAt))
	ctx, cancel := context.WithTimeout(parent, m.consumer.ProcessingTimeout)
	traceID, requestID := "", ""
	if task.TraceID != nil {
		traceID = *task.TraceID
	}
	if task.RequestID != nil {
		requestID = *task.RequestID
	}
	ctx = contracts.WithCorrelation(ctx, traceID, requestID)
	ctx = appobservability.ContextWithTraceID(ctx, traceID)
	ctx, span := otel.Tracer("github.com/1090-f/Memora/background").Start(ctx, "document.process")
	documentID := ""
	if task.DocumentID != nil {
		documentID = *task.DocumentID
	}
	span.SetAttributes(attribute.String("memora.task_id", taskID), attribute.String("memora.document_id", documentID))
	err = m.processor.ProcessImportTask(ctx, contracts.ID(taskID))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "document processing failed")
	}
	span.End()
	cancel()
	if err != nil {
		result = "failed"
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
