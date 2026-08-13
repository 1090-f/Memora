// Package worker 提供 Agent 运行的异步执行 Worker。
// Worker 通过数据库轮询模式领取排队中的 Agent 运行并异步执行。
package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AgentWorkerConfig 定义 Agent Worker 的执行参数。
type AgentWorkerConfig struct {
	Enabled    bool          // 是否启用异步 Worker（禁用时使用 goroutine 执行）
	PollPeriod time.Duration // 数据库轮询间隔时间
	BatchSize  int           // 每次轮询领取的最大运行数
	MaxRunTime time.Duration // 单次运行最大执行时间
}

// DefaultAgentWorkerConfig 返回 Agent Worker 的默认配置。
func DefaultAgentWorkerConfig() AgentWorkerConfig {
	return AgentWorkerConfig{
		Enabled:    true,
		PollPeriod: 2 * time.Second,
		BatchSize:  5,
		MaxRunTime: 300 * time.Second,
	}
}

// AgentWorker 是 Agent 运行的异步执行工作者。
// 它周期性地扫描数据库中状态为 queued 的 agent_runs 记录，
// 原子性地将其标记为 running 后调用 ContextBuilder 构建上下文并执行。
type AgentWorker struct {
	agentService   contracts.AgentRunService     // Agent 核心执行服务（负责路由和执行）
	runRepo        repository.AgentRunRepository // 运行记录 Repository（用于领取和更新状态）
	contextBuilder contracts.ContextBuilder      // 上下文构建器（从数据库加载会话、配置等信息）
	messageRepo    repository.MessageRepository  // 消息 Repository（用于持久化助手消息）
	config         AgentWorkerConfig             // Worker 配置
	mu             sync.Mutex                    // 保护 running 状态的互斥锁
	running        bool                          // 是否正在运行
}

// NewAgentWorker 创建 Agent Worker 实例。
// 需要 AgentRunService（执行路由和运行）、AgentRunRepository（领取和状态更新）、
// MessageRepository（持久化助手消息）和 ContextBuilder（从数据库重建上下文）。
func NewAgentWorker(
	agentService contracts.AgentRunService,
	runRepo repository.AgentRunRepository,
	messageRepo repository.MessageRepository,
	contextBuilder contracts.ContextBuilder,
	config AgentWorkerConfig,
) *AgentWorker {
	return &AgentWorker{
		agentService:   agentService,
		runRepo:        runRepo,
		messageRepo:    messageRepo,
		contextBuilder: contextBuilder,
		config:         config,
	}
}

// Run 启动 Worker 的主循环。该方法阻塞直到 ctx 被取消。
// Worker 启动后周期性地执行 pollAndExecute 循环，每个周期之间间隔 PollPeriod。
func (w *AgentWorker) Run(ctx context.Context) error {
	if !w.config.Enabled {
		logger.Info("Agent Worker 未启用，跳过异步执行")
		<-ctx.Done()
		return nil
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return errors.New("Agent Worker 已在运行")
	}
	w.running = true
	w.mu.Unlock()

	logger.Info("Agent Worker 已启动",
		zap.Duration("poll_period", w.config.PollPeriod),
		zap.Int("batch_size", w.config.BatchSize),
	)

	ticker := time.NewTicker(w.config.PollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Agent Worker 已停止")
			return nil
		case <-ticker.C:
			w.pollAndExecute(ctx)
		}
	}
}

// pollAndExecute 执行一次轮询和调度：
//  1. 查询 queued 状态的运行记录
//  2. 对每条记录原子性地领取（条件更新 status='queued' → 'running'）
//  3. 在独立的 goroutine 中执行 Agent 运行
func (w *AgentWorker) pollAndExecute(ctx context.Context) {
	runs, err := w.runRepo.ListQueued(ctx, w.config.BatchSize)
	if err != nil {
		if ctx.Err() == nil {
			logger.Error("查询排队中的 Agent 运行失败", zap.Error(err))
		}
		return
	}

	for i := range runs {
		run := runs[i]
		runID := run.ID

		// 原子性地将运行从 queued 标记为 running
		claimed, err := w.runRepo.ReserveQueued(ctx, runID)
		if err != nil {
			logger.Error("领取排队中的 Agent 运行失败",
				zap.String("run_id", runID.String()),
				zap.Error(err),
			)
			continue
		}
		if claimed == nil {
			// 该运行已被其他 Worker 实例领取，跳过
			continue
		}

		// 在后台 goroutine 中执行 Agent 运行
		// 使用独立的上下文，避免 Worker 主上下文取消传播到正在执行的 Agent
		go w.executeRun(claimed)
	}
}

// executeRun 在独立的 goroutine 中执行一次 Agent 运行。
// 流程：
//  1. 从已领取的运行记录中提取用户 ID、知识库 ID、会话 ID 和查询
//  2. 调用 ContextBuilder 构建完整的 Agent 执行上下文
//  3. 构造 AgentRunRequest 并调用 AgentRunService.Run 执行
//  4. 执行结果由核心服务内部处理（状态更新、Token 记录、事件发布等）
func (w *AgentWorker) executeRun(run *entity.AgentRun) {
	// 创建独立的执行上下文，带超时控制
	execCtx, cancel := context.WithTimeout(context.Background(), w.config.MaxRunTime)
	defer cancel()

	runID := contracts.ID(run.ID.String())

	// 1. 构建 Agent 执行上下文（从数据库加载会话历史、Agent 配置、记忆等）
	agentCtx, err := w.contextBuilder.Build(execCtx, contracts.AgentContextRequest{
		UserID:          contracts.ID(run.UserID.String()),
		KnowledgeBaseID: contracts.ID(run.KnowledgeBaseID.String()),
		ConversationID:  contracts.ID(run.ConversationID.String()),
		RunID:           runID,
		Query:           run.Query,
	})
	if err != nil {
		logger.Error("构建 Agent 执行上下文失败",
			zap.String("run_id", run.ID.String()),
			zap.Error(err),
		)
		// 标记运行失败，避免一直处于 running 状态
		if markErr := w.runRepo.MarkFailed(execCtx, run.ID, "context_build_error", fmt.Sprintf("构建上下文失败: %v", err)); markErr != nil {
			logger.Error("标记运行失败状态出错", zap.String("run_id", run.ID.String()), zap.Error(markErr))
		}
		return
	}

	// 2. 构造运行请求
	runRequest := contracts.AgentRunRequest{
		RunID:   runID,
		Context: agentCtx,
		Config:  contracts.DefaultAgentConfig(),
	}

	// 3. 调用核心服务执行 Agent 运行
	result, err := w.agentService.Run(execCtx, runRequest)
	if err != nil {
		// 执行失败时显式落库，避免运行记录一直停留在 running 状态。
		logger.Error("Agent 运行执行失败",
			zap.String("run_id", run.ID.String()),
			zap.Error(err),
		)
		if markErr := w.runRepo.MarkFailed(context.Background(), run.ID, "agent_run_error", err.Error()); markErr != nil {
			logger.Error("标记 Agent 运行失败状态出错", zap.String("run_id", run.ID.String()), zap.Error(markErr))
		}
		return
	}

	// 执行成功后保存最终回答、Token 用量、耗时、执行模式和知识状态。
	durationMs := result.EndedAt.Sub(result.StartedAt).Milliseconds()
	if markErr := w.runRepo.MarkCompleted(
		context.Background(),
		run.ID,
		result.FinalResult,
		result.Usage.InputTokens,
		result.Usage.OutputTokens,
		result.Usage.TotalTokens,
		durationMs,
		string(result.ExecutionMode),
		result.KnowledgeStatus,
	); markErr != nil {
		logger.Error("标记 Agent 运行完成状态出错", zap.String("run_id", run.ID.String()), zap.Error(markErr))
		return
	}

	// 持久化助手消息（AI 回复）
	if result.FinalResult != "" {
		msgID := uuid.New()
		runIDStr := run.ID.String()
		assistantMsg := &entity.Message{
			ID:              msgID.String(),
			ConversationID:  run.ConversationID.String(),
			UserID:          run.UserID.String(),
			KnowledgeBaseID: run.KnowledgeBaseID.String(),
			AgentRunID:      &runIDStr,
			Role:            "assistant",
			Content:         result.FinalResult,
			Status:          "completed",
			CreatedAt:       time.Now().UTC(),
		}
		if createErr := w.messageRepo.Create(context.Background(), assistantMsg); createErr != nil {
			logger.Error("持久化助手消息失败", zap.String("run_id", run.ID.String()), zap.Error(createErr))
		} else {
			// 回填 assistant_message_id
			if setErr := w.runRepo.SetAssistantMessageID(context.Background(), run.ID, msgID); setErr != nil {
				logger.Error("设置 assistant_message_id 失败", zap.String("run_id", run.ID.String()), zap.Error(setErr))
			}
		}
	}

	logger.Info("Agent 运行执行完成", zap.String("run_id", run.ID.String()))
}
