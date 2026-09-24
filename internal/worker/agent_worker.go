// Package worker 提供 Agent 运行的异步执行 Worker。
// Worker 通过数据库轮询模式领取排队中的 Agent 运行并异步执行。
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/metrics"
	appobservability "github.com/1090-f/Memora/pkg/observability"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	agentService    contracts.AgentRunService     // Agent 核心执行服务（负责路由和执行）
	runRepo         repository.AgentRunRepository // 运行记录 Repository（用于领取和更新状态）
	contextBuilder  contracts.ContextBuilder      // 上下文构建器（从数据库加载会话、配置等信息）
	messageRepo     repository.MessageRepository  // 消息 Repository（用于持久化助手消息）
	memoryExtractor contracts.MemoryExtractor     // 记忆提取器（从回答中提取长期记忆）
	events          contracts.EventPublisher
	config          AgentWorkerConfig // Worker 配置
	mu              sync.Mutex        // 保护 running 状态的互斥锁
	stageStarts     sync.Map          // run/stage -> time.Time，补齐阶段真实开始/结束时间
	executions      sync.WaitGroup    // 等待已领取运行退出，避免关库后仍访问依赖
	running         bool              // 是否正在运行
}

// NewAgentWorker 创建 Agent Worker 实例。
// 需要 AgentRunService（执行路由和运行）、AgentRunRepository（领取和状态更新）、
// MessageRepository（持久化助手消息）、ContextBuilder（从数据库重建上下文）
// 和 MemoryExtractor（从回答中提取长期记忆）。
func NewAgentWorker(
	agentService contracts.AgentRunService,
	runRepo repository.AgentRunRepository,
	messageRepo repository.MessageRepository,
	contextBuilder contracts.ContextBuilder,
	memoryExtractor contracts.MemoryExtractor,
	events contracts.EventPublisher,
	config AgentWorkerConfig,
) *AgentWorker {
	return &AgentWorker{
		agentService:    agentService,
		runRepo:         runRepo,
		messageRepo:     messageRepo,
		contextBuilder:  contextBuilder,
		memoryExtractor: memoryExtractor,
		events:          events,
		config:          config,
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

// RecoverStaleRuns 恢复超时的 running 状态运行。
// 服务启动时调用，将超过 MaxRunTime 的 running 状态运行标记为 failed。
func (w *AgentWorker) RecoverStaleRuns(ctx context.Context) error {
	if !w.config.Enabled {
		return nil
	}

	logger.Info("开始恢复超时的 running 状态运行")

	// 查询所有 running 状态的运行
	runningRuns, err := w.runRepo.ListRunning(ctx)
	if err != nil {
		return fmt.Errorf("list running runs: %w", err)
	}

	recoveredCount := 0
	for _, run := range runningRuns {
		// 检查运行时间是否超过 MaxRunTime
		if run.StartedAt != nil {
			elapsed := time.Since(*run.StartedAt)
			if elapsed > w.config.MaxRunTime {
				// 标记为 failed
				durationMs := time.Since(*run.StartedAt).Milliseconds()
				var execMode string
				if run.ExecutionMode != nil {
					execMode = *run.ExecutionMode
				}
				err := w.runRepo.MarkFailed(ctx, run.ID, "stale_run", fmt.Sprintf("run exceeded max execution time of %v", w.config.MaxRunTime), execMode, durationMs, run.InputTokens, run.OutputTokens, run.TotalTokens)
				if err != nil {
					logger.Error("恢复超时运行失败",
						zap.String("run_id", run.ID.String()),
						zap.Error(err),
					)
					continue
				}
				recoveredCount++
				logger.Info("已恢复超时运行",
					zap.String("run_id", run.ID.String()),
					zap.Duration("elapsed", elapsed),
				)
			}
		}
	}

	logger.Info("超时运行恢复完成", zap.Int("recovered", recoveredCount))
	return nil
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

		// 在后台 goroutine 中执行 Agent 运行；服务关闭时取消并等待所有已领取运行退出。
		w.executions.Add(1)
		go func() {
			defer w.executions.Done()
			w.executeRun(ctx, claimed)
		}()
	}
}

// Wait 等待所有已经领取的运行退出。
func (w *AgentWorker) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		w.executions.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// executeRun 在独立的 goroutine 中执行一次 Agent 运行。
// 流程：
//  1. 从已领取的运行记录中提取用户 ID、知识库 ID、会话 ID 和查询
//  2. 调用 ContextBuilder 构建完整的 Agent 执行上下文
//  3. 构造 AgentRunRequest 并调用 AgentRunService.Run 执行
//  4. 执行结果由核心服务内部处理（状态更新、Token 记录、事件发布等）
func (w *AgentWorker) executeRun(parent context.Context, run *entity.AgentRun) {
	startedAt := time.Now()
	metrics.StageFinished("agent_run", "queue_wait", "succeeded", time.Since(run.CreatedAt))

	// 创建独立的执行上下文，带超时控制
	execCtx, cancel := context.WithTimeout(parent, w.config.MaxRunTime)
	defer cancel()
	traceID, requestID := "", ""
	if run.TraceID != nil {
		traceID = *run.TraceID
	}
	if run.RequestID != nil {
		requestID = *run.RequestID
	}
	execCtx = contracts.WithCorrelation(execCtx, traceID, requestID)
	parentSpanID := ""
	if run.TraceParentSpanID != nil {
		parentSpanID = *run.TraceParentSpanID
	}
	execCtx = appobservability.ContextWithTraceParent(execCtx, traceID, parentSpanID, run.TraceSampled)
	queueEndedAt := time.Now().UTC()
	queueStartedAt := run.CreatedAt
	if queueStartedAt.IsZero() || queueStartedAt.After(queueEndedAt) {
		queueStartedAt = queueEndedAt
	}
	queueCtx, queueSpan := otel.Tracer("github.com/1090-f/Memora/worker").Start(execCtx, "queue.wait", trace.WithTimestamp(queueStartedAt))
	queueSpan.SetAttributes(attribute.String("memora.run_id", run.ID.String()), attribute.Int64("memora.queue_wait_ms", queueEndedAt.Sub(queueStartedAt).Milliseconds()))
	// Langfuse 里 queue.wait 是实际可见的根节点：它的父 Span 是触发该 run 的 HTTP 请求，
	// 而 HTTP Span 被导出白名单挡掉了，父节点缺失 → Langfuse 把 queue.wait 当作根。
	// 所以 trace 级元数据必须在这里也设一份，否则 trace 名可能退回成 "queue.wait"。
	queueSpan.SetAttributes(
		attribute.String("langfuse.trace.name", run.ID.String()),
		attribute.String("langfuse.session.id", run.ConversationID.String()),
		attribute.String("langfuse.user.id", run.UserID.String()),
	)
	// Langfuse：根节点同样要有 Input/Output。它被导出白名单当作 trace 的首屏节点，
	// 空着的话点开一条 trace 第一眼是白板。
	appobservability.SetLangfuseInput(queueSpan, run.Query)
	appobservability.SetLangfuseOutput(queueSpan, langfuseJSONPayload(map[string]any{
		"queue_wait_ms": queueEndedAt.Sub(queueStartedAt).Milliseconds(),
		"started_at":    queueStartedAt.UTC().Format(time.RFC3339Nano),
	}))
	queueSpan.End(trace.WithTimestamp(queueEndedAt))
	execCtx, span := otel.Tracer("github.com/1090-f/Memora/worker").Start(queueCtx, "agent.run")
	defer span.End()
	span.SetAttributes(
		attribute.String("memora.run_id", run.ID.String()),
		attribute.String("memora.knowledge_base_id", run.KnowledgeBaseID.String()),
		// Langfuse 的 trace 级元数据必须设在执行链的**根 Span** 上：
		// 这样一条 Trace 就是一次 Agent run，名字即 run_id，并带上会话/用户维度。
		// （此前误设在 agent.react 这类非根 Span 上，导致 trace 名不生效、被 db Span 淹没。）
		attribute.String("langfuse.trace.name", run.ID.String()),
		attribute.String("langfuse.session.id", run.ConversationID.String()),
		attribute.String("langfuse.user.id", run.UserID.String()),
	)
	// 节点类型显式声明为 agent，让 Langfuse 用 agent 语义渲染（默认会被当成普通 span）。
	appobservability.SetLangfuseObservationType(span, "agent")
	// 入参：用户问题。产出在下方每个终局分支里各自写入（见 langfuseRunOutput）。
	appobservability.SetLangfuseInput(span, run.Query)

	runID := contracts.ID(run.ID.String())
	execCtx = contracts.WithAgentStageReporter(execCtx, func(ctx context.Context, stage contracts.AgentStage, status contracts.StageStatus, durationMS int64, summary string, metadata map[string]any) {
		w.publishStage(ctx, runID, stage, status, durationMS, summary, metadata)
	})
	w.publishStage(execCtx, runID, contracts.AgentStageQueryRewrite, contracts.StageSkipped, 0, "当前查询无需独立改写")
	contextStarted := time.Now().UTC()
	w.publishStage(execCtx, runID, contracts.AgentStageContextBuild, contracts.StageRunning, 0, "正在准备会话、记忆与知识上下文")
	contextBuildCtx, contextBuildSpan := otel.Tracer("github.com/1090-f/Memora/worker").Start(execCtx, "context.build")
	appobservability.SetLangfuseInput(contextBuildSpan, run.Query)

	// 1. 构建 Agent 执行上下文（从数据库加载会话历史、Agent 配置、记忆等）
	agentCtx, err := w.contextBuilder.Build(contextBuildCtx, contracts.AgentContextRequest{
		UserID:          contracts.ID(run.UserID.String()),
		KnowledgeBaseID: contracts.ID(run.KnowledgeBaseID.String()),
		ConversationID:  contracts.ID(run.ConversationID.String()),
		RunID:           runID,
		ChatModelID:     contracts.ID(run.ChatModelID.String()),
		Query:           run.Query,
	})
	if err != nil {
		contextBuildSpan.RecordError(err)
		contextBuildSpan.SetStatus(codes.Error, "context build failed")
		appobservability.SetLangfuseOutput(contextBuildSpan, langfuseJSONPayload(map[string]any{
			"status": "failed",
			"error":  err.Error(),
		}))
		contextBuildSpan.End()
		span.RecordError(err)
		span.SetStatus(codes.Error, "context build failed")
		w.publishStage(execCtx, runID, contracts.AgentStageContextBuild, contracts.StageFailed, time.Since(contextStarted).Milliseconds(), "上下文构建失败")
		logger.Error("构建 Agent 执行上下文失败",
			zap.String("run_id", run.ID.String()),
			zap.Error(err),
		)
		// 标记运行失败，避免一直处于 running 状态
		failureMessage := fmt.Sprintf("构建上下文失败: %v", err)
		if markErr := w.runRepo.MarkFailed(execCtx, run.ID, "context_build_error", failureMessage, "", time.Since(startedAt).Milliseconds(), 0, 0, 0); markErr != nil {
			logger.Error("标记运行失败状态出错", zap.String("run_id", run.ID.String()), zap.Error(markErr))
		}
		// 此处尚未路由，执行模式未知，故不携带 mode。补发终态事件，避免前端 SSE 一直等待。
		w.publishRunFailed(execCtx, runID, "", err)
		w.updateRunObservability(execCtx, run.ID, contracts.AgentStageContextBuild, true, "请检查知识库、模型与会话配置后重试。")
		// 创建失败状态的助手消息，确保问答页面能够展示本次运行失败的结果。
		w.createFailureMessage(context.Background(), run, fmt.Sprintf("抱歉，系统在准备回答时遇到了问题。失败原因：%s", failureMessage))
		appobservability.SetLangfuseOutput(span, langfuseRunOutput("context_build_failed", "", contracts.TokenUsage{}, err))
		return
	}
	appobservability.SetLangfuseOutput(contextBuildSpan, contextBuildSummaryForLangfuse(agentCtx))
	contextBuildSpan.End()
	w.publishStage(execCtx, runID, contracts.AgentStageContextBuild, contracts.StageSucceeded, time.Since(contextStarted).Milliseconds(), "上下文准备完成")
	if agentCtx.KnowledgeStatus == "" {
		w.publishStage(execCtx, runID, contracts.AgentStageKnowledgeCheck, contracts.StageSkipped, 0, "知识检索已降级，未生成充分性结论")
	}
	w.publishStage(execCtx, runID, contracts.AgentStageRoute, contracts.StageRunning, 0, "正在选择执行路径")

	// 2. 构造运行请求（使用从数据库加载的配置）
	runRequest := contracts.AgentRunRequest{
		RunID:   runID,
		Context: agentCtx,
		Config:  buildConfigFromAgentContext(agentCtx),
	}

	// 3. 调用核心服务执行 Agent 运行
	result, err := w.agentService.Run(execCtx, runRequest)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "agent run failed")
		// 如果错误是 context 取消导致的（用户主动停止），不应覆盖 DB 中已被 Cancel 设为 cancelled 的状态。
		if errors.Is(err, context.Canceled) {
			if execCtx.Err() != nil {
				durationMS := time.Since(startedAt).Milliseconds()
				if markErr := w.runRepo.MarkFailed(context.Background(), run.ID, "worker_stopped", "服务停止，运行已中断", "", durationMS, result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.TotalTokens); markErr != nil {
					logger.Error("标记被服务停止中断的运行失败", zap.String("run_id", run.ID.String()), zap.Error(markErr))
				}
				w.publishRunFailed(execCtx, runID, "", errors.New("服务停止，运行已中断"))
				logger.Info("Agent 运行因 Worker 停止而中断", zap.String("run_id", run.ID.String()))
				appobservability.SetLangfuseOutput(span, langfuseRunOutput("worker_stopped", "", result.Usage, errors.New("服务停止，运行已中断")))
				return
			}
			logger.Info("Agent 运行已被用户取消", zap.String("run_id", run.ID.String()))
			appobservability.SetLangfuseOutput(span, langfuseRunOutput("cancelled", "", result.Usage, context.Canceled))
			return
		}

		// 执行失败时显式落库，避免运行记录一直停留在 running 状态。
		executionMode := ""
		var runErr *contracts.AgentRunError
		if errors.As(err, &runErr) {
			executionMode = string(runErr.ExecutionMode)
		}
		logger.Error("Agent 运行执行失败",
			zap.String("run_id", run.ID.String()),
			zap.String("execution_mode", executionMode),
			zap.Int("input_tokens", result.Usage.InputTokens),
			zap.Int("output_tokens", result.Usage.OutputTokens),
			zap.Int("total_tokens", result.Usage.TotalTokens),
			zap.Error(err),
		)
		if markErr := w.runRepo.MarkFailed(context.Background(), run.ID, "agent_run_error", err.Error(), executionMode, time.Since(startedAt).Milliseconds(), result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.TotalTokens); markErr != nil {
			logger.Error("标记 Agent 运行失败状态出错", zap.String("run_id", run.ID.String()), zap.Error(markErr))
		}
		w.publishRunFailed(execCtx, runID, contracts.ExecutionMode(executionMode), err)
		w.updateRunObservability(context.Background(), run.ID, contracts.AgentStageModelGenerate, true, "请重试；若仍失败，请检查模型服务状态并使用 Trace ID 诊断。")
		// 创建失败状态的助手消息，向用户反馈本次运行执行失败。
		w.createFailureMessage(context.Background(), run, w.getFriendlyErrorMessage(err))
		appobservability.SetLangfuseOutput(span, langfuseRunOutput("failed", "", result.Usage, err))
		return
	}

	// 执行成功后保存最终回答、Token 用量、耗时、执行模式和知识状态。
	// 关键检查：FinalResult 必须非空且有实际内容，否则标记为失败
	if strings.TrimSpace(result.FinalResult) == "" {
		logger.Warn("Agent 运行成功但最终回答为空，标记为失败",
			zap.String("run_id", run.ID.String()),
			zap.String("execution_mode", string(result.ExecutionMode)),
		)
		if markErr := w.runRepo.MarkFailed(
			context.Background(),
			run.ID,
			"empty_final_answer",
			"任务执行完成但未生成有效最终回答",
			string(result.ExecutionMode),
			result.EndedAt.Sub(result.StartedAt).Milliseconds(),
			result.Usage.InputTokens,
			result.Usage.OutputTokens,
			result.Usage.TotalTokens,
		); markErr != nil {
			logger.Error("标记 Agent 运行失败状态出错", zap.String("run_id", run.ID.String()), zap.Error(markErr))
		}
		// 空回答在 DB 侧一直是 failed（empty_final_answer）。P2 之前 adkcore 已抢先发过
		// agent.run.completed，形成「事件说成功、DB 说失败」；现在终态事件收回 Worker，
		// 这里必须补发 failed，两端语义才一致。
		w.publishRunFailed(execCtx, runID, result.ExecutionMode, errors.New("任务执行完成但未生成有效最终回答"))
		w.updateRunObservability(context.Background(), run.ID, contracts.AgentStageAnswer, true, "请重试或简化问题；若持续为空，请检查模型输出配置。")
		w.createFailureMessage(context.Background(), run, "任务执行完成但未生成有效最终回答，请重试或简化问题")
		appobservability.SetLangfuseOutput(span, langfuseRunOutput("empty_final_answer", result.FinalResult, result.Usage, errors.New("任务执行完成但未生成有效最终回答")))
		return
	}

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
		// 兜底：不能让运行记录永久停留在 running。MarkCompleted 未生效时显式标记失败并落一条
		// 失败消息，避免出现「事件已宣告 completed、DB 却停在 running、用户看不到任何输出」的僵尸运行。
		// 不重试 MarkCompleted —— 写失败通常是 DB 侧问题，立即重试大概率再失败且会阻塞 worker。
		if failErr := w.runRepo.MarkFailed(
			context.Background(),
			run.ID,
			"persist_error",
			"运行结果保存失败，请重试",
			string(result.ExecutionMode),
			durationMs,
			result.Usage.InputTokens,
			result.Usage.OutputTokens,
			result.Usage.TotalTokens,
		); failErr != nil {
			logger.Error("补标记运行失败状态出错", zap.String("run_id", run.ID.String()), zap.Error(failErr))
		}
		w.publishRunFailed(execCtx, runID, result.ExecutionMode, errors.New("运行结果保存失败"))
		w.updateRunObservability(context.Background(), run.ID, contracts.AgentStageAnswer, true, "请重试；若持续失败，请检查数据库状态。")
		w.createFailureMessage(context.Background(), run, "运行结果保存失败，请重试")
		appobservability.SetLangfuseOutput(span, langfuseRunOutput("persist_failed", result.FinalResult, result.Usage, errors.New("运行结果保存失败")))
		return
	}
	// MarkCompleted 已成功落库，此时才发布成功终态事件 ——
	// 「收到 agent.run.completed ⇒ agent_runs.final_result 已可读」的不变式由此成立。
	// 必须排在下面的 updateRunObservability 之前（原因见 publishRunCompleted 的注释）。
	appobservability.SetLangfuseOutput(span, langfuseRunOutput("completed", result.FinalResult, result.Usage, nil))
	w.publishRunCompleted(execCtx, runID, result)
	w.updateRunObservability(context.Background(), run.ID, "", false, "")

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
			ModelConfigID:   stringPtr(run.ChatModelID.String()),
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

	// 异步提取长期记忆（不阻塞主流程）
	if w.memoryExtractor == nil {
		logger.Info("[记忆提取] 跳过：记忆提取器未初始化",
			zap.String("run_id", run.ID.String()),
		)
	} else if result.FinalResult == "" {
		logger.Info("[记忆提取] 跳过：最终结果为空",
			zap.String("run_id", run.ID.String()),
		)
	} else {
		logger.Info("[记忆提取] 开始异步提取长期记忆",
			zap.String("run_id", run.ID.String()),
			zap.String("user_id", run.UserID.String()),
			zap.String("query", run.Query),
			zap.Int("answer_length", len(result.FinalResult)),
		)
		go func() {
			if err := w.memoryExtractor.Extract(context.Background(), agentCtx, result.FinalResult); err != nil {
				logger.Error("[记忆提取] 提取长期记忆失败",
					zap.String("run_id", run.ID.String()),
					zap.Error(err),
				)
			} else {
				logger.Info("[记忆提取] 提取长期记忆完成",
					zap.String("run_id", run.ID.String()),
				)
			}
		}()
	}

	logger.Info("Agent 运行执行完成", zap.String("run_id", run.ID.String()))
}

func (w *AgentWorker) publishStage(ctx context.Context, runID contracts.ID, stage contracts.AgentStage, status contracts.StageStatus, durationMS int64, summary string, metadata ...map[string]any) {
	if w.events == nil {
		return
	}
	if durationMS > 0 && status != contracts.StageRunning {
		metrics.StageFinished("agent_run", string(stage), string(status), time.Duration(durationMS)*time.Millisecond)
	}
	var safeMetadata map[string]any
	if len(metadata) > 0 {
		safeMetadata = metadata[0]
	}
	now := time.Now().UTC()
	stageKey := string(runID) + "/" + string(stage)
	var startedAt, endedAt *time.Time
	var observedDuration *int64
	if status == contracts.StageRunning {
		w.stageStarts.Store(stageKey, now)
		startedAt = &now
	} else {
		started := now
		if value, ok := w.stageStarts.LoadAndDelete(stageKey); ok {
			if stored, valid := value.(time.Time); valid {
				started = stored
			}
		} else if durationMS > 0 {
			started = now.Add(-time.Duration(durationMS) * time.Millisecond)
		}
		if durationMS <= 0 {
			durationMS = now.Sub(started).Milliseconds()
		}
		startedAt, endedAt, observedDuration = &started, &now, &durationMS
	}
	payload, err := json.Marshal(contracts.StageObservation{Stage: string(stage), Status: status, StartedAt: startedAt, EndedAt: endedAt, DurationMS: observedDuration, Summary: summary, Metadata: safeMetadata})
	if err != nil {
		return
	}
	traceID, requestID := contracts.CorrelationFromContext(ctx)
	if err := w.events.Publish(ctx, contracts.AgentEvent{RunID: runID, TraceID: traceID, RequestID: requestID, Stage: stage, Status: status, EventType: contracts.EventStageUpdated, Data: payload}); err != nil {
		logger.Warn("发布问答阶段事件失败，运行继续", zap.String("run_id", string(runID)), zap.String("stage", string(stage)), zap.Error(err))
	}
}

// --- Langfuse 节点正文辅助 ---

// langfuseJSONPayload 把结构压成 JSON 供 Langfuse 渲染。
// 序列化失败返回空串 —— observability.SetLangfuse* 对空串是 no-op，埋点不会因此把主流程带崩。
func langfuseJSONPayload(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

// langfuseRunOutput 汇总 agent.run 节点的产出。
// status 取「终局分支名」，便于在 Langfuse 上直接筛「哪些 run 是空回答 / 落库失败」。
func langfuseRunOutput(status, answer string, usage contracts.TokenUsage, err error) string {
	payload := map[string]any{
		"status":        status,
		"answer":        answer,
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
		"total_tokens":  usage.TotalTokens,
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	return langfuseJSONPayload(payload)
}

// contextBuildSummaryForLangfuse 汇总上下文构建结果。
// 只放结构性指标、不放完整提示词 —— 提示词正文已在每次 generation 节点里出现，
// 在这里再写一遍只是让 Span 变肥（同一份内容外发两遍）。
func contextBuildSummaryForLangfuse(ctx contracts.AgentContext) string {
	return langfuseJSONPayload(map[string]any{
		"status":              "succeeded",
		"conversation_msgs":   len(ctx.Conversation.Messages),
		"memories":            len(ctx.Memories),
		"allowed_tools":       ctx.AllowedTools,
		"knowledge_status":    ctx.KnowledgeStatus,
		"memory_enabled":      ctx.MemoryEnabled,
		"network_enabled":     ctx.NetworkEnabled,
		"system_prompt_chars": len([]rune(ctx.SystemPrompt)),
	})
}

func (w *AgentWorker) updateRunObservability(ctx context.Context, runID uuid.UUID, failureStage contracts.AgentStage, retryable bool, recoveryAdvice string) {
	update := repository.AgentRunObservabilityUpdate{}
	if provider, ok := w.events.(contracts.AgentRunTimingProvider); ok {
		timing := provider.AgentRunTiming(contracts.ID(runID.String()))
		update.FirstTokenAt = timing.FirstTokenAt
		update.FirstTokenLatencyMS = timing.FirstTokenLatencyMS
		update.ModelGenerateDurationMS = timing.ModelGenerateDurationMS
		if timing.FirstTokenLatencyMS != nil {
			metrics.StageFinished("agent_run", "first_token", "succeeded", time.Duration(*timing.FirstTokenLatencyMS)*time.Millisecond)
		}
	}
	if failureStage != "" {
		stage := string(failureStage)
		update.FailureStage = &stage
		update.Retryable = &retryable
		update.RecoveryAdvice = &recoveryAdvice
	}
	if err := w.runRepo.UpdateObservability(ctx, runID, update); err != nil {
		logger.Warn("更新 Agent 可观测摘要失败", zap.String("run_id", runID.String()), zap.Error(err))
	}
}

// agentRunTerminalPublisher 是 Worker 侧对「终态事件发布能力」的最小抽象。
// contracts.EventPublisher 只声明了 Publish，终态事件需要类型断言取用；
// 不把 AgentWorker.events 的字段类型换成 core.EventPublisher —— 那会牵动构造签名与既有测试的 fake。
type agentRunTerminalPublisher interface {
	PublishRunCompleted(ctx context.Context, runID contracts.ID, result contracts.AgentRunResult) error
	PublishRunFailed(ctx context.Context, runID contracts.ID, mode contracts.ExecutionMode, err error) error
}

// terminalPublishContext 构造发布终态事件所用的 context。
// 用 background 而非 execCtx：运行失败/服务停止时 execCtx 可能已被取消，一旦取消事件就发不出去，
// 前端 SSE 会一直等不到终态。同时把 correlation 从 execCtx 搬过来，否则事件会丢 trace_id，
// 排查时无法与日志 / Trace 关联。
func terminalPublishContext(execCtx context.Context) context.Context {
	base := context.Background()
	traceID, requestID := contracts.CorrelationFromContext(execCtx)
	if traceID == "" && requestID == "" {
		return base
	}
	return contracts.WithCorrelation(base, traceID, requestID)
}

// publishRunCompleted 在 MarkCompleted 落库成功之后发布成功终态事件，
// 使「收到终态事件 ⇒ agent_runs 已可读」成为不变式（P2 顺序根治）。
//
// ⚠️ 必须排在 updateRunObservability 之前：后者会调用 AgentRunTiming() 并消费掉 timings，
// 一旦被提前消费，本事件里的 first_token_at 与 model_generate_duration 会静默丢失（不报错）。
func (w *AgentWorker) publishRunCompleted(execCtx context.Context, runID contracts.ID, result contracts.AgentRunResult) {
	pub, ok := w.events.(agentRunTerminalPublisher)
	if !ok {
		return
	}
	if err := pub.PublishRunCompleted(terminalPublishContext(execCtx), runID, result); err != nil {
		logger.Error("发布运行完成事件失败", zap.String("run_id", string(runID)), zap.Error(err))
	}
}

// publishRunFailed 在 MarkFailed 落库之后补发失败终态事件。
// 终态事件原先由 adkcore 在落库前发布，P2 收回 Worker 后必须覆盖每一条终止分支：
// 漏掉任何一条，前端 SSE 都会一直等待终态事件（加载态永远不结束）。
func (w *AgentWorker) publishRunFailed(execCtx context.Context, runID contracts.ID, mode contracts.ExecutionMode, runErr error) {
	pub, ok := w.events.(agentRunTerminalPublisher)
	if !ok {
		return
	}
	if err := pub.PublishRunFailed(terminalPublishContext(execCtx), runID, mode, runErr); err != nil {
		logger.Error("发布运行失败事件失败", zap.String("run_id", string(runID)), zap.Error(err))
	}
}

func stringPtr(value string) *string { return &value }

// createFailureMessage 创建失败状态的助手消息，确保失败运行也能在问答页面留下可见结果。
func (w *AgentWorker) createFailureMessage(ctx context.Context, run *entity.AgentRun, content string) {
	msgID := uuid.New()
	runIDStr := run.ID.String()
	failureMsg := &entity.Message{
		ID:              msgID.String(),
		ConversationID:  run.ConversationID.String(),
		UserID:          run.UserID.String(),
		KnowledgeBaseID: run.KnowledgeBaseID.String(),
		AgentRunID:      &runIDStr,
		Role:            "assistant",
		Content:         content,
		Status:          "failed",
		CreatedAt:       time.Now().UTC(),
	}
	if createErr := w.messageRepo.Create(ctx, failureMsg); createErr != nil {
		logger.Error("持久化 Agent 失败消息出错", zap.String("run_id", run.ID.String()), zap.Error(createErr))
		return
	}

	// 回填运行记录中的助手消息 ID，建立运行记录和失败消息之间的关联。
	if setErr := w.runRepo.SetAssistantMessageID(ctx, run.ID, msgID); setErr != nil {
		logger.Error("设置失败消息 ID 出错", zap.String("run_id", run.ID.String()), zap.Error(setErr))
	}
}

// getFriendlyErrorMessage 将底层执行错误转换为用户可读的聊天提示。
func (w *AgentWorker) getFriendlyErrorMessage(err error) string {
	if err == nil {
		return "抱歉，AI 暂时无法完成回答，请稍后重试。"
	}
	return fmt.Sprintf("抱歉，AI 在处理您的问题时遇到了错误：%s", err.Error())
}

// buildConfigFromAgentContext 从 AgentContext 构建 AgentConfig。
// 使用从数据库加载的配置，而不是硬编码的默认值。
func buildConfigFromAgentContext(ctx contracts.AgentContext) contracts.AgentConfig {
	config := contracts.DefaultAgentConfig()

	// 使用从数据库加载的 Plan-Execute 配置
	if ctx.MaxPlanSteps > 0 {
		config.MaxPlanSteps = ctx.MaxPlanSteps
	}
	if ctx.MaxReplans > 0 {
		config.MaxReplans = ctx.MaxReplans
	}
	if ctx.ReviewerRuns > 0 {
		config.ReviewerRuns = ctx.ReviewerRuns
	}

	// 使用从数据库加载的其他配置
	if ctx.MaxReactRounds > 0 {
		config.MaxReactRounds = ctx.MaxReactRounds
	}

	logger.Info("Agent 配置加载完成",
		zap.Int("max_plan_steps", config.MaxPlanSteps),
		zap.Int("max_replans", config.MaxReplans),
		zap.Int("reviewer_runs", config.ReviewerRuns),
	)

	return config
}
