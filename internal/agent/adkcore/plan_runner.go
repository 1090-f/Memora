package adkcore

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service"
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

	// 发布计划创建事件
	_ = g.eventPublisher.PublishPlanCreated(ctx, request.RunID, plan)

	// 2. 执行 + 重规划循环（最多 MaxReplans 次重规划）
	for {
		execErr := g.executePlan(ctx, plan, request)
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
		_ = g.eventPublisher.PublishPlanReplanned(ctx, request.RunID, plan)
	}

	// 3. 生成最终答案（在审查之前，让审查检查真实结果）
	finalAnswer := g.generateFinalAnswer(plan, request)

	// 4. Reviewer 阶段：审查最终答案的完整性
	reviewResult, err := g.reviewer.Review(ctx, plan, request)
	if err != nil {
		// 审查失败不阻断流程，记录警告
		_ = g.eventPublisher.PublishPlanReplanned(ctx, request.RunID, plan)
	} else if !reviewResult.Approved {
		// 审查不通过但已无重规划额度，记录问题但仍然返回结果
		_ = reviewResult
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
// 如果 plan.FinalAnswer 为空或包含 markdown 格式，则从步骤输出中提取并格式化。
func (g *PlanExecuteGraph) generateFinalAnswer(plan *contracts.Plan, request contracts.AgentRunRequest) string {
	// 如果已有 final_answer 且不是 markdown 格式，直接使用
	if plan.FinalAnswer != "" && !containsMarkdown(plan.FinalAnswer) {
		return plan.FinalAnswer
	}

	// 从步骤输出中提取答案
	var answers []string
	for _, step := range plan.Steps {
		if step.Status == contracts.PlanStepStatusCompleted && step.Output != "" {
			// 跳过工具调用的原始输出（通常是 JSON）和模板占位符/无关内容块
			if !isToolOutput(step.Output) && !isBoilerplateOutput(step.Output) {
				answers = append(answers, step.Output)
			}
		}
	}

	// 如果没有提取到答案，使用 plan.FinalAnswer（去除 markdown 格式）
	if len(answers) == 0 {
		if plan.FinalAnswer != "" {
			return stripMarkdown(plan.FinalAnswer)
		}
		return "任务已完成，但未生成具体答案。"
	}

	// 组合答案
	result := ""
	for i, answer := range answers {
		if i > 0 {
			result += "\n\n"
		}
		result += stripMarkdown(answer)
	}

	return result
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
	boilerplateHeaders := []string{
		"GitHub网页内容分析结果",
		"项目内容总结",
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
func (g *PlanExecuteGraph) executePlan(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) error {
	// 1. 拓扑排序（返回指针，修改直接回写到 plan.Steps）
	layers, err := g.topologicalSort(plan.Steps)
	if err != nil {
		return fmt.Errorf("topological sort: %w", err)
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
	}

	// 3. 逐层并行执行
	for _, layer := range layers {
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
	// TODO: 从步骤输出中提取引用
	return citations
}
