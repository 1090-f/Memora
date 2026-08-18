package adkcore

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// PlanExecuteGraph 基于 Eino compose.Graph 的 Plan-Execute 执行引擎。
type PlanExecuteGraph struct {
	planner        *service.PlannerService
	executor       *service.PlanExecutor
	replanner      *service.ReplanService
	reviewer       *service.ReviewerService
	eventPublisher core.EventPublisher

	// 依赖图缓存
	dependencyCache map[contracts.ID][]contracts.ID
	mu              sync.RWMutex
}

// NewPlanExecuteGraph 创建 Plan-Execute Graph。
func NewPlanExecuteGraph(
	planner *service.PlannerService,
	executor *service.PlanExecutor,
	replanner *service.ReplanService,
	reviewer *service.ReviewerService,
	eventPublisher core.EventPublisher,
) *PlanExecuteGraph {
	if eventPublisher == nil {
		eventPublisher = core.NoopEventPublisher{}
	}
	return &PlanExecuteGraph{
		planner:         planner,
		executor:        executor,
		replanner:       replanner,
		reviewer:        reviewer,
		eventPublisher:  eventPublisher,
		dependencyCache: make(map[contracts.ID][]contracts.ID),
	}
}

// Run 执行完整的 Plan-Execute 流程。
// 流程：生成计划 → 执行 → (失败则重规划循环) → 审查 → 生成最终答案。
func (g *PlanExecuteGraph) Run(ctx context.Context, request contracts.AgentRunRequest) (contracts.AgentRunResult, error) {
	startedAt := time.Now()

	// 1. Planner 阶段：生成计划
	plan, err := g.planner.Plan(ctx, request)
	if err != nil {
		return contracts.AgentRunResult{}, fmt.Errorf("plan: %w", err)
	}

	if err := g.executor.PreparePlan(plan, request); err != nil {
		return contracts.AgentRunResult{}, fmt.Errorf("prepare plan: %w", err)
	}
	// 发布计划创建事件
	_ = g.eventPublisher.PublishPlanCreated(ctx, request.RunID, plan)
	toolCallBudget := contracts.NewToolCallBudget(request.Config.MaxToolCalls)

	// 2. 执行 + 重规划循环（最多 MaxReplans 次重规划）
	for {
		execErr := g.executePlan(ctx, plan, request, toolCallBudget)
		if execErr == nil {
			break // 执行成功，退出循环
		}

		// 检查是否可以重规划
		if !plan.HasFailures() || plan.ReplanCount >= plan.MaxReplans {
			return contracts.AgentRunResult{}, fmt.Errorf("execute plan: %w", execErr)
		}

		// 重规划
		plan, err = g.replanner.Replan(ctx, plan, request)
		if err != nil {
			return contracts.AgentRunResult{}, fmt.Errorf("replan: %w", err)
		}
		if err := g.executor.PreparePlan(plan, request); err != nil {
			return contracts.AgentRunResult{}, fmt.Errorf("prepare replanned plan: %w", err)
		}
		_ = g.eventPublisher.PublishPlanReplanned(ctx, request.RunID, plan)
	}

	if g.executor.RequiresExternalEvidence(request) && !g.executor.HasSuccessfulExternalEvidence(plan, request) {
		return contracts.AgentRunResult{}, fmt.Errorf("external information request completed without successful network tool evidence")
	}

	// 3. 生成最终答案（在审查之前，让审查检查真实结果）
	finalAnswer, synthErr := g.executor.SynthesizeFinalAnswer(ctx, plan, request)
	if synthErr != nil {
		finalAnswer = g.generateFinalAnswer(plan, request)
	}
	plan.FinalAnswer = finalAnswer

	// 4. Reviewer 阶段：审查最终答案的完整性
	reviewResult, err := g.reviewer.Review(ctx, plan, request)
	if err == nil && reviewResult != nil && !reviewResult.Approved {
		// 审查失败不阻断流程，记录警告
		// 审查不通过但已无重规划额度，记录问题但仍然返回结果
		if refined, refineErr := g.executor.RefineFinalAnswer(ctx, plan, request, reviewResult); refineErr == nil {
			finalAnswer = refined
			plan.FinalAnswer = refined
		}
	}

	// 发布最终答案事件
	_ = g.eventPublisher.PublishAnswerDelta(ctx, request.RunID, finalAnswer)

	// 5. 组装最终结果
	return contracts.AgentRunResult{
		RunID:         request.RunID,
		ExecutionMode: contracts.ExecutionPlanExecute,
		FinalResult:   finalAnswer,
		Citations:     extractCitationsFromPlan(plan),
		Usage:         contracts.TokenUsage{},
		StartedAt:     startedAt,
		EndedAt:       time.Now(),
	}, nil
}

// generateFinalAnswer 生成最终答案，确保是纯文本格式。
// 优先级：plan.FinalAnswer > 步骤输出提取的文本 > Plan Goal > 兜底提示。
// 工具步骤的 Output 是 ToolResult JSON，需要解析出其中的 text 字段，
// 而非像旧逻辑那样直接跳过所有 JSON 输出。
func (g *PlanExecuteGraph) generateFinalAnswer(plan *contracts.Plan, request contracts.AgentRunRequest) string {
	// 1. 优先使用 Planner 生成的 final_answer（去除 markdown 格式）
	if plan.FinalAnswer != "" && !hasCompletedOutput(plan) {
		answer := stripMarkdown(plan.FinalAnswer)
		// 检查答案是否仍然是 JSON 格式，如果是则尝试提取文本
		// 这种情况可能发生在 Planner 基于先验知识生成了包含原始 JSON 的答案
		if isToolOutput(answer) {
			extracted := extractTextFromNestedJSON(answer)
			if extracted != answer {
				logger.Debug("Planner final_answer 是 JSON 格式，已提取文本",
					zap.String("run_id", string(request.RunID)),
					zap.Int("original_len", len(answer)),
					zap.Int("extracted_len", len(extracted)))
				return extracted
			}
		}
		return answer
	}

	// 2. 从步骤输出中提取有效文本
	var answers []string
	var failedSteps []string
	for _, step := range plan.Steps {
		if step.Status != contracts.PlanStepStatusCompleted || step.Output == "" {
			if step.Status == contracts.PlanStepStatusFailed {
				failedSteps = append(failedSteps, fmt.Sprintf("步骤%d(%s): %s", step.StepNumber, step.Title, step.Error))
			}
			continue
		}
		// 提取步骤输出中的有效文本（工具步骤解析 ToolResult JSON 的 text 字段）
		text := extractStepOutputText(step.Output)
		if text == "" {
			logger.Debug("步骤输出提取文本为空，跳过",
				zap.Int("step_number", step.StepNumber),
				zap.String("title", step.Title),
				zap.String("tool_name", step.ToolName))
			continue
		}
		if isBoilerplateOutput(text) {
			logger.Debug("步骤输出被判定为模板占位符，跳过",
				zap.Int("step_number", step.StepNumber),
				zap.String("title", step.Title),
				zap.Int("text_len", len(text)))
			continue
		}
		answers = append(answers, text)
	}

	// 3. 如果有有效答案，组合并返回
	if len(answers) > 0 {
		result := ""
		for i, answer := range answers {
			if i > 0 {
				result += "\n\n"
			}
			result += stripMarkdown(answer)
		}

		// 最终检查：确保答案不包含 JSON 格式
		// 这是最后一道防线，防止任何遗漏的 JSON 格式内容出现在最终答案中
		if isToolOutput(result) {
			extracted := extractTextFromNestedJSON(result)
			if extracted != result {
				logger.Debug("最终答案是 JSON 格式，已提取文本",
					zap.String("run_id", string(request.RunID)),
					zap.Int("original_len", len(result)),
					zap.Int("extracted_len", len(extracted)))
				return extracted
			}
		}

		return result
	}

	// 4. 当有失败步骤时，明确返回失败信息，不使用 Goal 兜底
	// Goal 兜底仅在无失败步骤（如步骤未执行）时使用
	if len(failedSteps) > 0 {
		logger.Error("计划执行失败，存在失败步骤",
			zap.String("run_id", string(request.RunID)),
			zap.Int("total_steps", len(plan.Steps)),
			zap.Strings("failed_steps", failedSteps))
		return fmt.Sprintf("任务执行失败：%s", strings.Join(failedSteps, "；"))
	}

	// 5. 兜底方案：使用 Plan Goal 作为答案
	// 当所有步骤输出都为空时（可能是工具调用失败或步骤未执行），
	// 使用 Planner 生成的 Goal 作为答案，而不是返回无意义的"任务已完成，但未生成具体答案。"
	if plan.Goal != "" {
		logger.Warn("所有步骤输出均为空，使用 Plan Goal 作为答案",
			zap.String("run_id", string(request.RunID)),
			zap.String("goal", plan.Goal),
			zap.Int("total_steps", len(plan.Steps)),
			zap.Int("failed_steps", len(failedSteps)))
		return plan.Goal
	}

	// 6. 最终兜底：返回错误信息
	logger.Error("Plan 无有效输出且无 Goal",
		zap.String("run_id", string(request.RunID)),
		zap.Int("total_steps", len(plan.Steps)),
		zap.Strings("failed_steps", failedSteps))
	if len(failedSteps) > 0 {
		return fmt.Sprintf("任务执行失败：%s", strings.Join(failedSteps, "；"))
	}
	return "任务执行过程中未生成有效答案，请重试。"
}

// extractStepOutputText 从步骤输出中提取有效文本。
func hasCompletedOutput(plan *contracts.Plan) bool {
	for _, step := range plan.Steps {
		if step.Status == contracts.PlanStepStatusCompleted && strings.TrimSpace(step.Output) != "" {
			return true
		}
	}
	return false
}

// 工具调用步骤的 Output 是 ToolResult JSON 序列化字符串，
// 需要解析出其中的 text 字段；纯 LLM 推理步骤的 Output 是纯文本，直接返回。
// 注意：MCP 工具返回的 Text 字段可能是嵌套的 JSON（如 {"content":[{"type":"text","text":"..."}]}），
// 需要进一步解析提取其中的文本内容。
func extractStepOutputText(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}
	// 非 JSON 文本（纯 LLM 步骤输出），直接返回原文本
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return output
	}
	// 尝试解析为 ToolResult JSON，提取 text 字段
	var toolResult contracts.ToolResult
	if err := json.Unmarshal([]byte(trimmed), &toolResult); err == nil {
		// 检查是否真的是 ToolResult 格式（有 call_id 或 tool_name 字段）
		isToolResult := toolResult.CallID != "" || toolResult.ToolName != ""
		if isToolResult {
			if toolResult.Text != "" {
				// 进一步解析 toolResult.Text，可能是嵌套的 JSON（如 MCP 工具返回格式）
				return extractTextFromNestedJSON(toolResult.Text)
			}
			// text 为空但 StructuredData 有内容时，降级使用 StructuredData
			if len(toolResult.StructuredData) > 0 {
				return extractTextFromNestedJSON(string(toolResult.StructuredData))
			}
			return ""
		}
	}
	// 非标准 ToolResult 格式或解析失败，尝试从原始 JSON 提取文本
	return extractTextFromNestedJSON(trimmed)
}

// extractTextFromNestedJSON 从嵌套的 JSON 字符串中提取文本内容。
// 支持格式：
// - {"content":[{"type":"text","text":"..."}]}  (MCP 工具返回格式)
// - {"text":"..."}
// - {"content":"..."}
// 如果无法提取，返回原文本。
func extractTextFromNestedJSON(jsonStr string) string {
	trimmed := strings.TrimSpace(jsonStr)
	if trimmed == "" {
		return ""
	}

	// 如果不是 JSON 格式，直接返回
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return jsonStr
	}

	// 尝试解析为通用 JSON 对象
	var genericObj map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &genericObj); err != nil {
		// 解析失败，返回原文本
		return jsonStr
	}

	// 格式1: {"content":[{"type":"text","text":"..."}]}
	if content, ok := genericObj["content"].([]interface{}); ok {
		var texts []string
		for _, item := range content {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	}

	// 格式2: {"text":"..."}
	if text, ok := genericObj["text"].(string); ok {
		return text
	}

	// 格式3: {"content":"..."}
	if content, ok := genericObj["content"].(string); ok {
		return content
	}

	// 无法提取文本，返回原文本
	return jsonStr
}

// containsMarkdown 检查文本是否包含 markdown 格式
func containsMarkdown(text string) bool {
	// 检查常见的 markdown 标记
	markers := []string{"##", "**", "```", "- ", "* ", "1. ", "> "}
	for _, marker := range markers {
		if len(text) >= 2 {
			for i := 0; i <= len(text)-2; i++ {
				if text[i:i+2] == marker || (len(marker) == 1 && text[i:i+1] == marker) {
					return true
				}
			}
		}
	}
	return false
}

// isToolOutput 检查是否是工具调用的原始输出（JSON 格式）
func isToolOutput(text string) bool {
	trimmed := strings.TrimSpace(text)
	return (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
}

// isBoilerplateOutput 检查是否是模板占位符或无关内容块。
// MCP 工具可能返回包含占位符的 Markdown 模板，这些不应作为最终答案。
func isBoilerplateOutput(text string) bool {
	// 1. 检测模板占位符（中括号包裹的提示文本）
	placeholderPatterns := []string{
		"[请在此处填入",
		"[通常位于",
		"[一句话描述",
		"[具体解决",
		"[在此处填写",
		"[例如：",
		"[详细背景",
		"[主要能力",
		"[依赖的系统",
		"[pip/npm/docker",
		"[代码片段或",
		"[关键配置",
		"[接口列表",
		"[MVC/MVVM",
		"[核心功能",
		"[典型使用",
		"[插件/模块",
		"[是否持续维护",
		"[相关工具",
		"[相关教程",
		"[具体解决的问题或满足的需求]",
		"[最新发布版本号",
		"[最后更新时间",
		"[详细背景和目标]",
	}
	for _, p := range placeholderPatterns {
		if strings.Contains(text, p) {
			return true
		}
	}

	// 2. 检测通用模板标题（MCP 工具返回的标准化报告模板）
	// 注意：不要把 LLM 总结时常见的自然语言标题（如 "项目内容总结"、"GitHub网页内容分析结果"）
	// 列入此处，否则会误杀 LLM 生成的有效答案。只保留真正的模板标记。
	boilerplateHeaders := []string{
		"任务执行结果：结构化报告",
		"标准化分析模板",
		"注：实际分析需根据",
		"注：以上为总结模板",
		"（注：以上为总结模板",
		"实际内容需根据具体项目填充",
		"此处为标准化分析模板",
	}
	for _, h := range boilerplateHeaders {
		if strings.Contains(text, h) {
			return true
		}
	}

	// 3. 检测占位符密度过高（超过30%的行是占位符）
	lines := strings.Split(text, "\n")
	if len(lines) > 5 {
		placeholderCount := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// 检查行内是否包含占位符标记
			if strings.Contains(trimmed, "[") && strings.Contains(trimmed, "]") &&
				(strings.Contains(trimmed, "填写") || strings.Contains(trimmed, "描述") ||
					strings.Contains(trimmed, "例如") || strings.Contains(trimmed, "位于")) {
				placeholderCount++
			}
		}
		if float64(placeholderCount)/float64(len(lines)) > 0.3 {
			return true
		}
	}

	return false
}

// stripMarkdown 去除 markdown 格式，返回纯文本，并规范化格式。
func stripMarkdown(text string) string {
	result := text

	// 去除标题标记
	for i := 0; i < 6; i++ {
		prefix := strings.Repeat("#", i+1) + " "
		result = strings.ReplaceAll(result, prefix, "")
	}

	// 去除加粗标记
	result = strings.ReplaceAll(result, "**", "")
	result = strings.ReplaceAll(result, "__", "")

	// 去除斜体标记
	result = strings.ReplaceAll(result, "*", "")
	result = strings.ReplaceAll(result, "_", "")

	// 去除代码块标记
	result = strings.ReplaceAll(result, "```", "")

	// 去除分隔线
	result = strings.ReplaceAll(result, "---", "")
	result = strings.ReplaceAll(result, "***", "")
	result = strings.ReplaceAll(result, "___", "")

	// 去除列表标记和冗余内容
	lines := strings.Split(result, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 跳过空行（后续会合并连续空行）
		if trimmed == "" {
			cleanLines = append(cleanLines, "")
			continue
		}

		// 去除冗余引导语前缀
		trimmed = stripRedundantPrefix(trimmed)
		if trimmed == "" {
			continue
		}

		// 去除无序列表标记
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			trimmed = trimmed[2:]
		}
		// 去除有序列表标记（简单处理）
		if len(trimmed) > 2 {
			for j := 1; j <= 9; j++ {
				prefix := fmt.Sprintf("%d. ", j)
				if strings.HasPrefix(trimmed, prefix) {
					trimmed = trimmed[len(prefix):]
					break
				}
			}
		}
		// 去除引用标记
		if strings.HasPrefix(trimmed, "> ") {
			trimmed = trimmed[2:]
		}
		cleanLines = append(cleanLines, trimmed)
	}

	// 合并连续空行为单个空行
	result = strings.Join(cleanLines, "\n")
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}

// stripRedundantPrefix 去除冗余的引导语前缀。
func stripRedundantPrefix(text string) string {
	redundantPrefixes := []string{
		"检索结果：",
		"检索结果:",
		"注：",
		"注:",
		"备注：",
		"备注:",
	}
	for _, prefix := range redundantPrefixes {
		if strings.HasPrefix(text, prefix) {
			text = strings.TrimPrefix(text, prefix)
			text = strings.TrimSpace(text)
		}
	}
	return text
}

// executePlan 执行计划中的所有步骤（DAG 并行）。
func (g *PlanExecuteGraph) executePlan(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest, toolCallBudget *contracts.ToolCallBudget) error {
	// 1. 拓扑排序（返回指针，修改直接回写到 plan.Steps）
	layers, err := g.topologicalSort(plan.Steps)
	if err != nil {
		return fmt.Errorf("topological sort: %w", err)
	}

	// 2a. 零步骤计划直接视为执行失败
	if len(plan.Steps) == 0 {
		return fmt.Errorf("execute plan: plan has no steps to execute")
	}

	// 2. 构建 ToolContext
	toolCtx := contracts.ToolContext{
		UserID:           request.Context.UserID,
		KnowledgeBaseID:  request.Context.KnowledgeBaseID,
		AgentRunID:       request.RunID,
		AllowedToolNames: request.Context.AllowedTools,
		NetworkEnabled:   request.Context.NetworkEnabled,
		MaxResultBytes:   request.Config.MaxToolResultBytes,
		ChatModelID:      request.Context.ChatModelID,
		ToolCallBudget:   toolCallBudget,
	}

	// 3. 逐层并行执行
	for _, layer := range layers {
		toolCtx.PriorStepOutputs = completedPlanStepOutputs(plan)
		if err := g.executeParallelLayer(ctx, layer, toolCtx); err != nil {
			return err
		}
	}

	return nil
}

// topologicalSort 对步骤进行拓扑排序，返回可并行执行的指针层级。
// 返回的 *PlanStep 直接指向 plan.Steps 中的元素，修改会回写到原始 Plan。
func (g *PlanExecuteGraph) topologicalSort(steps []contracts.PlanStep) ([][]*contracts.PlanStep, error) {
	if len(steps) == 0 {
		return nil, nil
	}

	// 构建依赖图和入度表
	dependencyMap := make(map[contracts.ID][]contracts.ID)
	inDegree := make(map[contracts.ID]int)
	stepMap := make(map[contracts.ID]*contracts.PlanStep)

	for i := range steps {
		step := &steps[i]
		stepMap[step.ID] = step
		inDegree[step.ID] = len(step.DependsOn)
		for _, depID := range step.DependsOn {
			dependencyMap[depID] = append(dependencyMap[depID], step.ID)
		}
	}

	// 校验：依赖的 ID 必须存在于步骤中
	for _, step := range stepMap {
		for _, depID := range step.DependsOn {
			if _, ok := stepMap[depID]; !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", step.ID, depID)
			}
		}
	}

	// BFS 层次遍历
	var layers [][]*contracts.PlanStep
	visited := make(map[contracts.ID]bool)

	// 找出入度为 0 的节点
	var queue []contracts.ID
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		var layer []*contracts.PlanStep
		var nextQueue []contracts.ID

		for _, id := range queue {
			if visited[id] {
				continue
			}
			visited[id] = true
			if step, ok := stepMap[id]; ok {
				layer = append(layer, step) // 指针，不是值拷贝
			}

			for _, dependentID := range dependencyMap[id] {
				inDegree[dependentID]--
				if inDegree[dependentID] == 0 {
					nextQueue = append(nextQueue, dependentID)
				}
			}
		}

		if len(layer) > 0 {
			layers = append(layers, layer)
		}
		queue = nextQueue
	}

	// 校验：所有步骤都必须被访问（检测环和孤立节点）
	if len(visited) != len(stepMap) {
		var missing []contracts.ID
		for id := range stepMap {
			if !visited[id] {
				missing = append(missing, id)
			}
		}
		return nil, fmt.Errorf("dependency cycle or unreachable steps detected: %v steps not processed", len(missing))
	}

	return layers, nil
}

// executeParallelLayer 并行执行同一层级的步骤（指针）。
func (g *PlanExecuteGraph) executeParallelLayer(ctx context.Context, layer []*contracts.PlanStep, toolCtx contracts.ToolContext) error {
	if len(layer) == 0 {
		return nil
	}

	// 如果只有一个步骤，直接执行
	if len(layer) == 1 {
		step := layer[0]
		_ = g.eventPublisher.PublishStepStarted(ctx, toolCtx.AgentRunID, step.StepNumber, step.Title)
		err := g.executor.ExecuteStep(ctx, step, toolCtx)
		_ = g.eventPublisher.PublishStepCompleted(ctx, toolCtx.AgentRunID, step.StepNumber, step.Title, err == nil)
		return err
	}

	// 多个步骤并行执行
	var wg sync.WaitGroup
	errs := make([]error, len(layer))

	for i := range layer {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			step := layer[idx]
			_ = g.eventPublisher.PublishStepStarted(ctx, toolCtx.AgentRunID, step.StepNumber, step.Title)
			errs[idx] = g.executor.ExecuteStep(ctx, step, toolCtx)
			_ = g.eventPublisher.PublishStepCompleted(ctx, toolCtx.AgentRunID, step.StepNumber, step.Title, errs[idx] == nil)
		}(i)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

// extractCitationsFromPlan 从计划中提取引用。
func extractCitationsFromPlan(plan *contracts.Plan) []contracts.Citation {
	var citations []contracts.Citation
	seen := make(map[string]struct{})
	for _, step := range plan.Steps {
		if step.Status != contracts.PlanStepStatusCompleted || step.Output == "" {
			continue
		}
		var result contracts.ToolResult
		if err := json.Unmarshal([]byte(step.Output), &result); err != nil {
			continue
		}
		for _, citation := range result.Citations {
			encoded, err := json.Marshal(citation)
			if err != nil {
				continue
			}
			key := string(encoded)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			citations = append(citations, citation)
		}
	}
	// TODO: 从步骤输出中提取引用
	return citations
}

func completedPlanStepOutputs(plan *contracts.Plan) map[int]string {
	outputs := make(map[int]string)
	for _, step := range plan.Steps {
		if step.Status == contracts.PlanStepStatusCompleted && strings.TrimSpace(step.Output) != "" {
			outputs[step.StepNumber] = dependencyStepOutput(step.Output)
		}
	}
	return outputs
}

func dependencyStepOutput(output string) string {
	var result contracts.ToolResult
	if json.Unmarshal([]byte(output), &result) == nil {
		if result.Text != "" {
			return result.Text
		}
		if len(result.StructuredData) > 0 {
			return string(result.StructuredData)
		}
	}
	return output
}
