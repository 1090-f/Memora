package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/agent/core"
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
	events       core.EventPublisher
	selector     *ToolSelector
}

// NewPlanExecutor 创建 PlanExecutor 实例。
// toolCallRepo 用于记录步骤中工具调用的详情到 tool_calls 表
func NewPlanExecutor(toolExecutor *agenttools.Executor, modelFactory contracts.ModelFactory, toolCallRepo repository.ToolCallRepository, events core.EventPublisher) *PlanExecutor {
	return &PlanExecutor{
		toolExecutor: toolExecutor,
		modelFactory: modelFactory,
		toolCallRepo: toolCallRepo,
		events:       events,
		selector:     NewToolSelector(toolExecutor),
	}
}

func (e *PlanExecutor) PreparePlan(plan *contracts.Plan, request contracts.AgentRunRequest) error {
	if e == nil || e.selector == nil {
		return fmt.Errorf("tool selector is unavailable")
	}
	return e.selector.PreparePlan(plan, request)
}

func (e *PlanExecutor) RequiresExternalEvidence(request contracts.AgentRunRequest) bool {
	return requestRequiresExternalEvidence(request)
}

func (e *PlanExecutor) HasSuccessfulExternalEvidence(plan *contracts.Plan, request contracts.AgentRunRequest) bool {
	if e == nil || e.selector == nil {
		return false
	}
	return planHasSuccessfulExternalEvidence(plan, e.selector.catalog(request))
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
	if step.Kind == contracts.PlanStepKindTool && step.ToolName == "" {
		return failPlanStep(step, "tool step has no selected tool", "")
	}
	if step.Kind == contracts.PlanStepKindReasoning && step.ToolName != "" {
		return failPlanStep(step, "reasoning step cannot declare a tool", "")
	}
	if step.ToolName != "" {
		if e.toolExecutor == nil {
			return failPlanStep(step, "tool executor is unavailable", "")
		}
		spec, exists := e.toolExecutor.Spec(step.ToolName)
		if !exists {
			return failPlanStep(step, fmt.Sprintf("selected tool %q is not registered", step.ToolName), "")
		}
		resolvedArguments := resolvePlanArguments(step.Arguments, toolCtx.PriorStepOutputs)
		if err := toolCtx.ToolCallBudget.Acquire(step.ToolName, spec.MaxCalls); err != nil {
			return failPlanStep(step, err.Error(), "")
		}
		if err := validateRequiredToolArguments(spec.InputSchema, resolvedArguments); err != nil {
			return failPlanStep(step, err.Error(), "")
		}
		arguments, err := json.Marshal(resolvedArguments)
		if err != nil {
			return failPlanStep(step, fmt.Sprintf("marshal arguments: %v", err), "")
		}
		call := contracts.ToolCall{CallID: contracts.ID(uuid.NewString()), ToolName: step.ToolName, Arguments: json.RawMessage(arguments)}
		startedAt := time.Now().UTC()
		e.recordToolCallStarted(ctx, toolCtx, call)

		// 记录工具调用详情（调用开始 → 结束）。
		var callID uuid.UUID
		recording := false
		if e.toolCallRepo != nil {
			callID, recording = e.createToolCall(ctx, toolCtx, &call, startedAt)
		}

		result, err := e.toolExecutor.InvokeContext(ctx, toolCtx, call)
		e.recordToolCallCompleted(ctx, toolCtx, call, result, err)
		if recording {
			e.finishToolCall(ctx, callID, startedAt, result, err)
		}

		if err != nil {
			return failPlanStep(step, fmt.Sprintf("execute tool %s: %v", step.ToolName, err), result)
		}
		var toolResult contracts.ToolResult
		if json.Unmarshal([]byte(result), &toolResult) == nil && !toolResult.Success {
			message := toolResult.ErrorMessage
			if message == "" {
				message = "tool returned an unsuccessful result"
			}
			return failPlanStep(step, message, result)
		}
		step.Output = result
		step.Status = contracts.PlanStepStatusCompleted
		completedAt := time.Now()
		step.CompletedAt = &completedAt
		return nil
	}

	// 4. 无工具步骤必须调用 LLM；缺少模型配置时不能伪装为成功。
	if step.ToolPolicy == contracts.ToolPolicyRequired {
		return failPlanStep(step, "required tool step was not executed", "")
	}
	if e.modelFactory == nil || toolCtx.ChatModelID == "" {
		return failPlanStep(step, "chat model is required for a non-tool plan step", "")
	}
	output, err := e.inferWithLLM(ctx, step, toolCtx)
	if err != nil {
		return failPlanStep(step, fmt.Sprintf("LLM inference failed: %v", err), "")
	}
	step.Output = output

	// 5. 标记完成
	step.Status = contracts.PlanStepStatusCompleted
	completedAt := time.Now()
	step.CompletedAt = &completedAt
	return nil
}

func failPlanStep(step *contracts.PlanStep, message, output string) error {
	step.Status = contracts.PlanStepStatusFailed
	step.Error = message
	step.Output = output
	completedAt := time.Now()
	step.CompletedAt = &completedAt
	return fmt.Errorf("%s", message)
}

// inferWithLLM 使用 LLM 为无工具步骤生成输出。
func (e *PlanExecutor) inferWithLLM(ctx context.Context, step *contracts.PlanStep, toolCtx contracts.ToolContext) (string, error) {
	chatModel, err := e.modelFactory.GetChatModel(ctx, contracts.ID(toolCtx.ChatModelID))
	if err != nil {
		return "", fmt.Errorf("get chat model: %w", err)
	}

	// 构建推理提示词
	prompt := fmt.Sprintf("你是一个任务执行助手。请根据步骤描述和已完成依赖的真实输出，直接给出该步骤的执行结果。不得编造证据中没有的事实。\n\n"+
		"## 步骤标题\n%s\n\n"+
		"## 步骤描述\n%s\n\n"+
		"## 已完成依赖输出\n%s\n\n"+
		"请直接输出该步骤的执行结果，不需要额外解释。", step.Title, step.Description, formatPriorStepOutputs(toolCtx.PriorStepOutputs))

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

var stepOutputReferencePattern = regexp.MustCompile(`\{\{step_output:(\d+)\}\}`)

func resolvePlanArguments(arguments map[string]any, outputs map[int]string) map[string]any {
	if arguments == nil {
		return map[string]any{}
	}
	resolved, _ := resolvePlanValue(arguments, outputs).(map[string]any)
	return resolved
}

func resolvePlanValue(value any, outputs map[int]string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, nested := range typed {
			result[key] = resolvePlanValue(nested, outputs)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, nested := range typed {
			result[i] = resolvePlanValue(nested, outputs)
		}
		return result
	case string:
		return stepOutputReferencePattern.ReplaceAllStringFunc(typed, func(reference string) string {
			matches := stepOutputReferencePattern.FindStringSubmatch(reference)
			stepNumber, _ := strconv.Atoi(matches[1])
			return outputs[stepNumber]
		})
	default:
		return value
	}
}

func formatPriorStepOutputs(outputs map[int]string) string {
	if len(outputs) == 0 {
		return "（无）"
	}
	keys := make([]int, 0, len(outputs))
	for key := range outputs {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	var builder strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&builder, "步骤 %d：%s\n", key, outputs[key])
	}
	return builder.String()
}

// SynthesizeFinalAnswer 基于实际步骤证据回答用户问题。
func (e *PlanExecutor) SynthesizeFinalAnswer(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) (string, error) {
	if e.modelFactory == nil || request.Context.ChatModelID == "" {
		return "", fmt.Errorf("chat model is required to synthesize final answer")
	}
	evidence := buildPlanEvidence(plan)
	if evidence == "" {
		return "", fmt.Errorf("no completed step evidence")
	}
	model, err := e.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		return "", fmt.Errorf("get chat model: %w", err)
	}
	prompt := fmt.Sprintf("请只基于下面的步骤执行证据回答用户问题。整合重复信息、处理冲突；证据不足时明确说明，不得采用未被证据支持的计划草稿。保留有助于阅读的 Markdown。\n\n用户问题：\n%s\n\n执行证据：\n%s", request.Context.Query, evidence)
	response, err := model.Generate(ctx, contracts.ChatRequest{Messages: []contracts.ChatMessage{{Role: "user", Content: prompt}}})
	if err != nil {
		return "", fmt.Errorf("generate final answer: %w", err)
	}
	answer := strings.TrimSpace(response.Content)
	if answer == "" {
		return "", fmt.Errorf("model returned empty final answer")
	}
	return answer, nil
}

// RefineFinalAnswer 使用 Reviewer 的问题和建议修订一次答案。
func (e *PlanExecutor) RefineFinalAnswer(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest, review *contracts.ReviewerResult) (string, error) {
	if e.modelFactory == nil || request.Context.ChatModelID == "" {
		return "", fmt.Errorf("chat model is required to refine final answer")
	}
	model, err := e.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		return "", err
	}
	prompt := fmt.Sprintf("请根据审查意见修订最终回答，仍然只能使用执行证据中的事实。\n\n用户问题：%s\n\n当前回答：\n%s\n\n审查问题：%s\n\n改进建议：%s\n\n执行证据：\n%s", request.Context.Query, plan.FinalAnswer, review.Issues, review.Suggestion, buildPlanEvidence(plan))
	response, err := model.Generate(ctx, contracts.ChatRequest{Messages: []contracts.ChatMessage{{Role: "user", Content: prompt}}})
	if err != nil {
		return "", err
	}
	answer := strings.TrimSpace(response.Content)
	if answer == "" {
		return "", fmt.Errorf("model returned empty refined answer")
	}
	return answer, nil
}

func buildPlanEvidence(plan *contracts.Plan) string {
	var builder strings.Builder
	for _, step := range plan.Steps {
		if step.Status != contracts.PlanStepStatusCompleted || strings.TrimSpace(step.Output) == "" {
			continue
		}
		fmt.Fprintf(&builder, "### 步骤 %d：%s\n%s\n\n", step.StepNumber, step.Title, planStepOutputText(step.Output))
	}
	return strings.TrimSpace(builder.String())
}

func planStepOutputText(output string) string {
	var result contracts.ToolResult
	if json.Unmarshal([]byte(output), &result) == nil && (result.CallID != "" || result.ToolName != "") {
		if result.Text != "" {
			return result.Text
		}
		if len(result.StructuredData) > 0 {
			return string(result.StructuredData)
		}
	}
	return output
}

func (e *PlanExecutor) recordToolCallStarted(ctx context.Context, toolCtx contracts.ToolContext, call contracts.ToolCall) {
	if e.events != nil {
		displayName := agenttools.ShortToolName(call.ToolName)
		_ = e.events.PublishToolCallStarted(ctx, toolCtx.AgentRunID, displayName, call.CallID)
	}
}

func (e *PlanExecutor) recordToolCallCompleted(ctx context.Context, toolCtx contracts.ToolContext, call contracts.ToolCall, output string, toolErr error) {
	displayName := agenttools.ShortToolName(call.ToolName)
	status := "succeeded"
	var result contracts.ToolResult
	if json.Unmarshal([]byte(output), &result) == nil {
		if !result.Success {
			status = "failed"
		}
	}
	if toolErr != nil {
		status = "failed"
	}
	if e.events != nil {
		_ = e.events.PublishToolCallCompleted(ctx, toolCtx.AgentRunID, call.CallID, displayName, status == "succeeded", truncatePlanSummary(output))
	}
}

func truncatePlanSummary(value string) string {
	const maxSummaryBytes = 2000
	if len(value) <= maxSummaryBytes {
		return value
	}
	return value[:maxSummaryBytes]
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

	callID, err := uuid.Parse(string(call.CallID))
	if err != nil {
		callID = uuid.New()
		call.CallID = contracts.ID(callID.String())
	}
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
	if err := e.toolCallRepo.Create(ctx, toolCall); err != nil {
		return uuid.Nil, false
	}

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
