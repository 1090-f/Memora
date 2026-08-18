package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agenttools "github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/google/uuid"
)

// PlanExecutor 负责执行计划中的步骤。
type PlanExecutor struct {
	toolExecutor *agenttools.Executor
	modelFactory contracts.ModelFactory
	toolCallRepo repository.ToolCallRepository
}

// NewPlanExecutor 创建 PlanExecutor 实例。
// toolCallRepo 用于记录步骤中工具调用的详情到 tool_calls 表（可空）。
func NewPlanExecutor(toolExecutor *agenttools.Executor, modelFactory contracts.ModelFactory, toolCallRepo repository.ToolCallRepository) *PlanExecutor {
	return &PlanExecutor{
		toolExecutor: toolExecutor,
		modelFactory: modelFactory,
		toolCallRepo: toolCallRepo,
	}
}

// ExecuteStep 执行单个计划步骤。
func (e *PlanExecutor) ExecuteStep(ctx context.Context, step *contracts.PlanStep, toolCtx contracts.ToolContext) error {
	// 1. 检查步骤状态（跳过已完成/已失败）
	if step.Status == contracts.PlanStepStatusCompleted || step.Status == contracts.PlanStepStatusFailed {
		return nil
	}

	// 2. 更新状态为运行中
	step.Status = contracts.PlanStepStatusRunning
	now := time.Now()
	step.StartedAt = &now

	// 3. 如果指定了 ToolName，使用 ToolExecutor 调用工具
	if step.ToolName != "" {
		arguments, err := json.Marshal(step.Arguments)
		if err != nil {
			step.Status = contracts.PlanStepStatusFailed
			step.Error = fmt.Sprintf("marshal arguments: %v", err)
			return err
		}

		call := contracts.ToolCall{
			ToolName:  step.ToolName,
			Arguments: json.RawMessage(arguments),
		}

		// 记录工具调用详情（调用开始 → 结束）。
		callStartedAt := time.Now().UTC()
		var callID uuid.UUID
		recording := false
		if e.toolCallRepo != nil {
			callID, recording = e.createToolCall(ctx, toolCtx, &call, callStartedAt)
		}

		result, err := e.toolExecutor.InvokeContext(ctx, toolCtx, call)
		if recording {
			e.finishToolCall(ctx, callID, callStartedAt, result, err)
		}

		if err != nil {
			step.Status = contracts.PlanStepStatusFailed
			step.Error = err.Error()
			completedAt := time.Now()
			step.CompletedAt = &completedAt
			return fmt.Errorf("execute tool %s: %w", step.ToolName, err)
		}

		step.Output = result
		step.Status = contracts.PlanStepStatusCompleted
		completedAt := time.Now()
		step.CompletedAt = &completedAt
		return nil
	}

	// 4. 无工具步骤：调用 LLM 进行推理/分析/汇总
	if e.modelFactory != nil && toolCtx.ChatModelID != "" {
		output, err := e.inferWithLLM(ctx, step, toolCtx)
		if err != nil {
			step.Status = contracts.PlanStepStatusFailed
			step.Error = fmt.Sprintf("LLM inference failed: %v", err)
			completedAt := time.Now()
			step.CompletedAt = &completedAt
			return fmt.Errorf("LLM inference for step %d: %w", step.StepNumber, err)
		}
		step.Output = output
	}

	// 5. 标记完成
	step.Status = contracts.PlanStepStatusCompleted
	completedAt := time.Now()
	step.CompletedAt = &completedAt
	return nil
}

// inferWithLLM 使用 LLM 为无工具步骤生成输出。
func (e *PlanExecutor) inferWithLLM(ctx context.Context, step *contracts.PlanStep, toolCtx contracts.ToolContext) (string, error) {
	chatModel, err := e.modelFactory.GetChatModel(ctx, contracts.ID(toolCtx.ChatModelID))
	if err != nil {
		return "", fmt.Errorf("get chat model: %w", err)
	}

	// 构建推理提示词
	prompt := fmt.Sprintf("你是一个任务执行助手。请根据以下步骤描述，直接给出该步骤的执行结果。\n\n"+
		"## 步骤标题\n%s\n\n"+
		"## 步骤描述\n%s\n\n"+
		"请直接输出该步骤的执行结果，不需要额外解释。", step.Title, step.Description)

	chatRequest := contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "user", Content: prompt},
		},
	}

	response, err := chatModel.Generate(ctx, chatRequest)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return response.Content, nil
}

// ExecuteSteps 并行执行多个步骤（同一层级）。
func (e *PlanExecutor) ExecuteSteps(ctx context.Context, steps []contracts.PlanStep, toolCtx contracts.ToolContext) []error {
	if len(steps) == 0 {
		return nil
	}

	// 如果只有一个步骤，直接执行
	if len(steps) == 1 {
		err := e.ExecuteStep(ctx, &steps[0], toolCtx)
		return []error{err}
	}

	// 多个步骤并行执行
	errs := make([]error, len(steps))
	done := make(chan struct{}, len(steps))

	for i := range steps {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			errs[idx] = e.ExecuteStep(ctx, &steps[idx], toolCtx)
		}(i)
	}

	// 等待所有步骤完成
	for range done {
	}

	return errs
}

// createToolCall 创建一条 status = running 的工具调用记录。
// 返回调用 ID 与是否成功创建；仓库缺失或运行 ID 非法时不创建记录并返回 false。
func (e *PlanExecutor) createToolCall(ctx context.Context, toolCtx contracts.ToolContext, call *contracts.ToolCall, startedAt time.Time) (uuid.UUID, bool) {
	runID, err := uuid.Parse(string(toolCtx.AgentRunID))
	if err != nil {
		return uuid.Nil, false
	}

	toolType := "internal"
	var mcpServerID, mcpToolID *uuid.UUID
	if spec, ok := e.toolExecutor.Spec(call.ToolName); ok && spec.Type == contracts.ToolTypeMCP {
		toolType = "mcp"
		if serverID, parseErr := uuid.Parse(spec.SourceID); parseErr == nil {
			mcpServerID = &serverID
		}
		if toolID, parseErr := uuid.Parse(spec.MCPToolID); parseErr == nil {
			mcpToolID = &toolID
		}
	}

	callID := uuid.New()
	toolCall := &entity.ToolCall{
		ID:           callID,
		AgentRunID:   runID,
		ToolName:     call.ToolName,
		ToolType:     toolType,
		MCPServerID:  mcpServerID,
		MCPToolID:    mcpToolID,
		Status:       "running",
		InputSummary: string(call.Arguments),
		StartedAt:    startedAt,
	}
	_ = e.toolCallRepo.Create(ctx, toolCall) // 创建失败不阻塞工具调用

	return callID, true
}

// finishToolCall 更新工具调用记录的执行结果。
func (e *PlanExecutor) finishToolCall(ctx context.Context, callID uuid.UUID, startedAt time.Time, result string, toolErr error) {
	if e.toolCallRepo == nil {
		return
	}

	endedAt := time.Now().UTC()
	durationMs := endedAt.Sub(startedAt).Milliseconds()

	status := "succeeded"
	var errorCode, errorMessage string
	if toolErr != nil {
		status = "failed"
		errorCode = "tool_error"
		errorMessage = toolErr.Error()
	}

	_ = e.toolCallRepo.UpdateResult(ctx, callID, status, result, errorCode, errorMessage, durationMs, false)
}
