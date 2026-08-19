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
func (e *PlanExecutor) ExecuteStep(ctx context.Context, step *contracts.PlanStep, toolCtx contracts.ToolContext) (contracts.TokenUsage, error) {
	// 1. 检查步骤状态（跳过已完成/已失败）
	if step.Status == contracts.PlanStepStatusCompleted || step.Status == contracts.PlanStepStatusFailed {
		return contracts.TokenUsage{}, nil
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

		// URL 工具特殊校验：确保参数是合法的 URL
		if err := validateURLToolArguments(spec, resolvedArguments); err != nil {
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
			// 检查是否是可恢复的错误（如404）
			if isRecoverableError(err) || isRecoverableToolResult(result) {
				return contracts.TokenUsage{}, &contracts.StepExecutionError{
					Kind:        contracts.StepErrorNotFound,
					Recoverable: true,
					Retryable:   false,
					Message:     err.Error(),
				}
			}
			return failPlanStep(step, fmt.Sprintf("execute tool %s: %v", step.ToolName, err), result)
		}

		// 检查工具返回的结果是否包含可恢复的错误
		if isRecoverableToolResult(result) {
			return contracts.TokenUsage{}, &contracts.StepExecutionError{
				Kind:        contracts.StepErrorNotFound,
				Recoverable: true,
				Retryable:   false,
				Message:     "tool returned not found error",
			}
		}

		var toolResult contracts.ToolResult
		if json.Unmarshal([]byte(result), &toolResult) == nil && !toolResult.Success {
			message := toolResult.ErrorMessage
			if message == "" {
				message = "tool returned an unsuccessful result"
			}
			// 检查是否是可恢复的工具结果
			if isRecoverableToolResult(result) {
				return contracts.TokenUsage{}, &contracts.StepExecutionError{
					Kind:        contracts.StepErrorNotFound,
					Recoverable: true,
					Retryable:   false,
					Message:     message,
				}
			}
			return failPlanStep(step, message, result)
		}
		step.Output = result
		step.Status = contracts.PlanStepStatusCompleted
		completedAt := time.Now()
		step.CompletedAt = &completedAt
		return contracts.TokenUsage{}, nil
	}

	// 4. 无工具步骤必须调用 LLM；缺少模型配置时不能伪装为成功。
	if step.ToolPolicy == contracts.ToolPolicyRequired {
		return failPlanStep(step, "required tool step was not executed", "")
	}
	if e.modelFactory == nil || toolCtx.ChatModelID == "" {
		return failPlanStep(step, "chat model is required for a non-tool plan step", "")
	}
	output, usage, err := e.inferWithLLM(ctx, step, toolCtx)
	if err != nil {
		return failPlanStep(step, fmt.Sprintf("LLM inference failed: %v", err), "")
	}
	step.Output = output

	// 5. 标记完成
	step.Status = contracts.PlanStepStatusCompleted
	completedAt := time.Now()
	step.CompletedAt = &completedAt
	return usage, nil
}

func failPlanStep(step *contracts.PlanStep, message, output string) (contracts.TokenUsage, error) {
	step.Status = contracts.PlanStepStatusFailed
	step.Error = message
	step.Output = output
	completedAt := time.Now()
	step.CompletedAt = &completedAt
	return contracts.TokenUsage{}, fmt.Errorf("%s", message)
}

func failPlanStepRecoverable(step *contracts.PlanStep, message, output string) {
	step.Status = contracts.PlanStepStatusFailed
	step.Error = message
	step.Output = output
	step.Recoverable = true
	completedAt := time.Now()
	step.CompletedAt = &completedAt
}

// inferWithLLM 使用 LLM 为无工具步骤生成输出。
func (e *PlanExecutor) inferWithLLM(ctx context.Context, step *contracts.PlanStep, toolCtx contracts.ToolContext) (string, contracts.TokenUsage, error) {
	chatModel, err := e.modelFactory.GetChatModel(ctx, contracts.ID(toolCtx.ChatModelID))
	if err != nil {
		return "", contracts.TokenUsage{}, fmt.Errorf("get chat model: %w", err)
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
		return "", contracts.TokenUsage{}, fmt.Errorf("generate: %w", err)
	}

	return response.Content, response.Usage, nil
}

var stepOutputReferencePattern = regexp.MustCompile(`\{\{step_output:(\d+)\}\}((?:\[\d+\])*(?:\.\w+)*)?`)

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
		// 检查是否是结构化绑定
		if binding, ok := contracts.ToStepOutputBinding(typed); ok {
			return resolveStepBinding(binding, outputs)
		}
		// 普通对象，递归解析
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
		return resolveStepOutputReference(typed, outputs)
	default:
		return value
	}
}

// resolveStepBinding 解析结构化绑定
func resolveStepBinding(binding *contracts.StepOutputBinding, outputs map[int]string) string {
	// 获取步骤输出
	output := outputs[binding.FromStep]
	if output == "" {
		return ""
	}

	// 解析字段路径
	if binding.Path == "" {
		return output
	}

	// 使用 extractFieldFromJSON 提取字段
	result, err := extractFieldFromJSONWithError(output, binding.Path)
	if err != nil {
		// 提取失败，返回空字符串（不返回原文本）
		return ""
	}
	return result
}

// resolveStepOutputReference 解析步骤输出引用，支持字段访问
// 格式：{{step_output:N}} 或 {{step_output:N}.field} 或 {{step_output:N}[0].field}
func resolveStepOutputReference(text string, outputs map[int]string) string {
	return stepOutputReferencePattern.ReplaceAllStringFunc(text, func(reference string) string {
		matches := stepOutputReferencePattern.FindStringSubmatch(reference)
		if len(matches) < 2 {
			return reference
		}

		stepNumber, _ := strconv.Atoi(matches[1])
		output := outputs[stepNumber]
		if output == "" {
			return ""
		}

		// 如果没有字段访问路径，直接返回原始输出
		fieldPath := ""
		if len(matches) > 2 {
			fieldPath = matches[2]
		}
		if fieldPath == "" {
			return output
		}

		// 解析 JSON 并提取字段
		return extractFieldFromJSON(output, fieldPath)
	})
}

// extractFieldFromJSON 从 JSON 字符串中提取指定字段
// 支持路径：.field、[0].field、[0][1].field 等
// 注意：此函数在提取失败时返回原文本（用于兼容），建议使用 extractFieldFromJSONWithError
func extractFieldFromJSON(jsonStr string, fieldPath string) string {
	result, _ := extractFieldFromJSONWithError(jsonStr, fieldPath)
	if result == "" {
		return jsonStr
	}
	return result
}

// extractFieldFromJSONWithError 从 JSON 字符串中提取指定字段，失败时返回错误
// 支持路径：.field、[0].field、[0][1].field 等
func extractFieldFromJSONWithError(jsonStr string, fieldPath string) (string, error) {
	trimmed := strings.TrimSpace(jsonStr)
	if trimmed == "" {
		return "", nil
	}

	// 尝试解析为 JSON
	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return "", fmt.Errorf("DEPENDENCY_OUTPUT_INVALID: %w", err)
	}

	// 解析字段路径
	path := parseFieldPath(fieldPath)
	if len(path) == 0 {
		return "", fmt.Errorf("DEPENDENCY_FIELD_NOT_FOUND: empty path")
	}

	// 按路径提取字段
	result := data
	for _, segment := range path {
		switch v := result.(type) {
		case map[string]any:
			if val, ok := v[segment]; ok {
				result = val
			} else {
				return "", fmt.Errorf("DEPENDENCY_FIELD_NOT_FOUND: field %q not found", segment)
			}
		case []any:
			// 尝试解析为索引
			if idx, err := strconv.Atoi(segment); err == nil && idx >= 0 && idx < len(v) {
				result = v[idx]
			} else if err != nil {
				return "", fmt.Errorf("DEPENDENCY_INDEX_OUT_OF_RANGE: invalid index %q", segment)
			} else {
				return "", fmt.Errorf("DEPENDENCY_INDEX_OUT_OF_RANGE: index %d out of range [0, %d)", idx, len(v))
			}
		default:
			return "", fmt.Errorf("DEPENDENCY_TYPE_MISMATCH: cannot navigate through %T", result)
		}
	}

	// 将结果转换为字符串
	switch v := result.(type) {
	case string:
		return v, nil
	case nil:
		return "", nil
	default:
		// 尝试序列化为 JSON
		if bytes, err := json.Marshal(v); err == nil {
			return string(bytes), nil
		}
		return fmt.Sprintf("%v", v), nil
	}
}

// parseFieldPath 解析字段路径
// 输入：".field"、"[0].field"、"[0][1].field"
// 输出：["field"]、["0", "field"]、["0", "1", "field"]
func parseFieldPath(path string) []string {
	var segments []string
	// 移除开头的点号
	path = strings.TrimPrefix(path, ".")

	for path != "" {
		// 检查是否是数组索引 [N]
		if strings.HasPrefix(path, "[") {
			endIdx := strings.Index(path, "]")
			if endIdx == -1 {
				break
			}
			// 提取索引值（去掉括号）
			index := path[1:endIdx]
			segments = append(segments, index)
			path = path[endIdx+1:]
			// 跳过点号
			if strings.HasPrefix(path, ".") {
				path = path[1:]
			}
		} else {
			// 字段名
			endIdx := strings.IndexAny(path, ".[")
			if endIdx == -1 {
				segments = append(segments, path)
				break
			}
			segments = append(segments, path[:endIdx])
			path = path[endIdx:]
			if strings.HasPrefix(path, ".") {
				path = path[1:]
			}
		}
	}

	return segments
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

// SynthesizeFinalAnswerWithUsage 基于实际步骤证据回答用户问题并返回 token 使用量。
func (e *PlanExecutor) SynthesizeFinalAnswerWithUsage(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) (string, contracts.TokenUsage, error) {
	if e.modelFactory == nil || request.Context.ChatModelID == "" {
		return "", contracts.TokenUsage{}, fmt.Errorf("chat model is required to synthesize final answer")
	}
	evidence := buildPlanEvidence(plan)
	if evidence == "" {
		return "", contracts.TokenUsage{}, fmt.Errorf("no completed step evidence")
	}
	model, err := e.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		return "", contracts.TokenUsage{}, fmt.Errorf("get chat model: %w", err)
	}
	prompt := fmt.Sprintf("请只基于下面的步骤执行证据回答用户问题。整合重复信息、处理冲突；证据不足时明确说明，不得采用未被证据支持的计划草稿。保留有助于阅读的 Markdown。\n\n用户问题：\n%s\n\n执行证据：\n%s", request.Context.Query, evidence)
	response, err := model.Generate(ctx, contracts.ChatRequest{Messages: []contracts.ChatMessage{{Role: "user", Content: prompt}}})
	if err != nil {
		return "", response.Usage, fmt.Errorf("generate final answer: %w", err)
	}
	answer := strings.TrimSpace(response.Content)
	if answer == "" {
		return "", response.Usage, fmt.Errorf("model returned empty final answer")
	}
	return answer, response.Usage, nil
}

// RefineFinalAnswerWithUsage 使用 Reviewer 的问题和建议修订一次答案并返回 token 使用量。
func (e *PlanExecutor) RefineFinalAnswerWithUsage(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest, review *contracts.ReviewerResult) (string, contracts.TokenUsage, error) {
	if e.modelFactory == nil || request.Context.ChatModelID == "" {
		return "", contracts.TokenUsage{}, fmt.Errorf("chat model is required to refine final answer")
	}
	model, err := e.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		return "", contracts.TokenUsage{}, err
	}
	prompt := fmt.Sprintf("请根据审查意见修订最终回答，仍然只能使用执行证据中的事实。\n\n用户问题：%s\n\n当前回答：\n%s\n\n审查问题：%s\n\n改进建议：%s\n\n执行证据：\n%s", request.Context.Query, plan.FinalAnswer, review.Issues, review.Suggestion, buildPlanEvidence(plan))
	response, err := model.Generate(ctx, contracts.ChatRequest{Messages: []contracts.ChatMessage{{Role: "user", Content: prompt}}})
	if err != nil {
		return "", response.Usage, err
	}
	answer := strings.TrimSpace(response.Content)
	if answer == "" {
		return "", response.Usage, fmt.Errorf("model returned empty refined answer")
	}
	return answer, response.Usage, nil
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
func (e *PlanExecutor) ExecuteSteps(ctx context.Context, steps []contracts.PlanStep, toolCtx contracts.ToolContext) ([]error, contracts.TokenUsage) {
	if len(steps) == 0 {
		return nil, contracts.TokenUsage{}
	}

	// 如果只有一个步骤，直接执行
	if len(steps) == 1 {
		_, err := e.ExecuteStep(ctx, &steps[0], toolCtx)
		return []error{err}, contracts.TokenUsage{}
	}

	// 多个步骤并行执行
	type stepResult struct {
		err   error
		usage contracts.TokenUsage
	}
	results := make([]stepResult, len(steps))
	done := make(chan struct{}, len(steps))

	for i := range steps {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			usage, err := e.ExecuteStep(ctx, &steps[idx], toolCtx)
			results[idx] = stepResult{err: err, usage: usage}
		}(i)
	}

	// 等待所有步骤完成
	for range done {
	}

	// 汇总结果
	errs := make([]error, len(steps))
	totalUsage := contracts.TokenUsage{}
	for i, r := range results {
		errs[i] = r.err
		totalUsage.Add(r.usage)
	}

	return errs, totalUsage
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

// isRecoverableError 检查错误是否是可恢复的（如 HTTP 404）
func isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()

	// 检查常见的可恢复错误模式
	recoverablePatterns := []string{
		"404 Not Found",
		"404",
		"not found",
		"Not Found",
		"page not found",
		"resource not found",
		"URL not found",
		"HTTP 404",
		"status code 404",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// isRecoverableToolResult 检查工具结果是否包含可恢复的错误
func isRecoverableToolResult(result string) bool {
	if result == "" {
		return false
	}

	// 解析工具结果
	var toolResult contracts.ToolResult
	if err := json.Unmarshal([]byte(result), &toolResult); err != nil {
		return false
	}

	// 检查错误消息中的可恢复模式
	if toolResult.ErrorMessage != "" {
		return isRecoverableError(fmt.Errorf("%s", toolResult.ErrorMessage))
	}

	// 检查文本内容中的可恢复模式
	if toolResult.Text != "" {
		recoverablePatterns := []string{
			"404 Not Found",
			"404",
			"not found",
			"Not Found",
			"page not found",
		}
		for _, pattern := range recoverablePatterns {
			if strings.Contains(toolResult.Text, pattern) {
				return true
			}
		}
	}

	return false
}

// validateURLToolArguments 校验 URL 工具的参数
// 确保参数是合法的 HTTP/HTTPS URL，不包含 JSON 内容或未解析的表达式
func validateURLToolArguments(toolSpec contracts.ToolSpec, arguments map[string]any) error {
	// 使用能力判断，而不是工具名称
	if !hasCapability(toolSpec.Capabilities, contracts.CapabilityWebFetch) {
		return nil
	}

	// 提取 URL 参数
	urlValue, ok := arguments["url"]
	if !ok {
		return fmt.Errorf("URL tool %q missing required 'url' parameter", toolSpec.Name)
	}

	urlStr, ok := urlValue.(string)
	if !ok {
		return fmt.Errorf("URL tool %q 'url' parameter must be a string, got %T", toolSpec.Name, urlValue)
	}

	// 校验 URL 长度
	if len(urlStr) > 2048 {
		return fmt.Errorf("URL too long (%d chars), expected a valid HTTP/HTTPS URL, not JSON content", len(urlStr))
	}

	// 校验是否包含 JSON 内容
	if strings.Contains(urlStr, "{") || strings.Contains(urlStr, "[") {
		return fmt.Errorf("URL parameter contains JSON content: %s...", urlStr[:min(100, len(urlStr))])
	}

	// 校验是否包含未解析的表达式
	if strings.Contains(urlStr, "{{step_output:") || strings.Contains(urlStr, "[0].") || strings.Contains(urlStr, "].") {
		return fmt.Errorf("URL parameter contains unresolved expression: %s", urlStr)
	}

	// 校验是否是合法的 HTTP/HTTPS URL
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return fmt.Errorf("URL must start with http:// or https://, got: %s", urlStr)
	}

	// 校验 URL 格式
	if !isValidURL(urlStr) {
		return fmt.Errorf("invalid URL format: %s", urlStr)
	}

	return nil
}

// isURLTool 检查是否是 URL 相关工具（保留用于兼容）
func isURLTool(toolName string) bool {
	urlTools := []string{
		"fetch_url",
		"web_fetch",
		"http_request",
		"get_page",
		"read_url",
	}
	for _, t := range urlTools {
		if strings.EqualFold(toolName, t) {
			return true
		}
	}
	return false
}

// hasCapability 检查工具是否具有指定能力
func hasCapability(capabilities []string, required string) bool {
	for _, cap := range capabilities {
		if cap == required {
			return true
		}
	}
	return false
}

// isValidURL 校验 URL 格式
func isValidURL(urlStr string) bool {
	// 简单的 URL 格式校验
	if !strings.Contains(urlStr, "://") {
		return false
	}

	// 校验域名部分
	parts := strings.SplitN(urlStr, "://", 2)
	if len(parts) != 2 {
		return false
	}

	scheme := parts[0]
	if scheme != "http" && scheme != "https" {
		return false
	}

	host := parts[1]
	if host == "" {
		return false
	}

	// 移除路径部分
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	// 校验域名格式（简单校验）
	if strings.Contains(host, " ") || strings.Contains(host, "..") {
		return false
	}

	return true
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
