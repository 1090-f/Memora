package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

// ReplanService 负责在计划执行失败时重新规划。
type ReplanService struct {
	planner *PlannerService
}

// NewReplanService 创建 ReplanService 实例。
func NewReplanService(planner *PlannerService) *ReplanService {
	return &ReplanService{
		planner: planner,
	}
}

// ReplanResult 重规划结果，包含新计划和元数据
type ReplanResult struct {
	NewPlan          *contracts.Plan               // 新计划
	Reason           string                        // 重规划原因
	FailedStepIDs    []contracts.ID                // 失败步骤 ID
	CompletedStepIDs []contracts.ID                // 已完成步骤 ID
	StepMapping      map[contracts.ID]contracts.ID // 新旧步骤映射（旧ID -> 新ID）
	Version          int                           // 新版本号
}

// Replan 重新规划未完成的步骤，创建不可变版本链。
func (s *ReplanService) Replan(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) (*ReplanResult, error) {
	// 1. 检查是否超过重规划次数限制
	if plan.ReplanCount >= plan.MaxReplans {
		return nil, fmt.Errorf("replan limit reached: %d/%d", plan.ReplanCount, plan.MaxReplans)
	}

	// 2. 收集已完成步骤的输出
	completedSteps := plan.GetCompletedSteps()
	failedSteps := plan.GetFailedSteps()

	// 3. 构建重规划原因
	reason := s.buildReplanReason(plan, completedSteps, failedSteps)

	// 4. 构建重规划提示词
	prompt := s.buildReplanPrompt(plan, completedSteps, failedSteps, request)

	// 5. 使用 PlannerService 生成新计划
	newPlan, err := s.planner.PlanWithPrompt(ctx, request, prompt)
	if err != nil {
		return nil, fmt.Errorf("replan: %w", err)
	}

	// 6. 合并步骤：保留已完成步骤，更新未完成步骤
	mergedSteps, stepMapping := s.mergeStepsWithMapping(plan.Steps, newPlan.Steps)
	if len(completedSteps) == 0 && len(mergedSteps) == 0 {
		return nil, fmt.Errorf("replan produced empty plan with no completed steps")
	}

	// 7. 创建新版本的计划
	newVersion := plan.ReplanCount + 2 // 版本从 1 开始，第一次 Replan 后是 2
	replanPlan := &contracts.Plan{
		ID:          newPlan.ID,
		RunID:       plan.RunID,
		Goal:        plan.Goal, // 保留原始目标
		FinalAnswer: newPlan.FinalAnswer,
		Steps:       mergedSteps,
		MaxSteps:    plan.MaxSteps,
		MaxReplans:  plan.MaxReplans,
		ReplanCount: plan.ReplanCount + 1,
		Status:      contracts.PlanStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 收集失败步骤 ID 和已完成步骤 ID
	var failedStepIDs, completedStepIDs []contracts.ID
	for _, step := range failedSteps {
		failedStepIDs = append(failedStepIDs, step.ID)
	}
	for _, step := range completedSteps {
		completedStepIDs = append(completedStepIDs, step.ID)
	}

	return &ReplanResult{
		NewPlan:          replanPlan,
		Reason:           reason,
		FailedStepIDs:    failedStepIDs,
		CompletedStepIDs: completedStepIDs,
		StepMapping:      stepMapping,
		Version:          newVersion,
	}, nil
}

// ReplanWithUsage 重新规划并返回 token 使用量。
func (s *ReplanService) ReplanWithUsage(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) (*ReplanResult, contracts.TokenUsage, error) {
	// 1. 检查是否超过重规划次数限制
	if plan.ReplanCount >= plan.MaxReplans {
		return nil, contracts.TokenUsage{}, fmt.Errorf("replan limit reached: %d/%d", plan.ReplanCount, plan.MaxReplans)
	}

	// 2. 收集已完成步骤的输出
	completedSteps := plan.GetCompletedSteps()
	failedSteps := plan.GetFailedSteps()

	// 3. 构建重规划原因
	reason := s.buildReplanReason(plan, completedSteps, failedSteps)

	// 4. 构建重规划提示词
	prompt := s.buildReplanPrompt(plan, completedSteps, failedSteps, request)

	// 5. 使用 PlannerService 生成新计划
	newPlan, usage, err := s.planner.PlanWithPromptUsage(ctx, request, prompt)
	if err != nil {
		return nil, usage, fmt.Errorf("replan: %w", err)
	}

	// 6. 合并步骤：保留已完成步骤，更新未完成步骤
	mergedSteps, stepMapping := s.mergeStepsWithMapping(plan.Steps, newPlan.Steps)
	if len(completedSteps) == 0 && len(mergedSteps) == 0 {
		return nil, usage, fmt.Errorf("replan produced empty plan with no completed steps")
	}

	// 7. 创建新版本的计划
	newVersion := plan.ReplanCount + 2 // 版本从 1 开始，第一次 Replan 后是 2
	replanPlan := &contracts.Plan{
		ID:          newPlan.ID,
		RunID:       plan.RunID,
		Goal:        plan.Goal, // 保留原始目标
		FinalAnswer: newPlan.FinalAnswer,
		Steps:       mergedSteps,
		MaxSteps:    plan.MaxSteps,
		MaxReplans:  plan.MaxReplans,
		ReplanCount: plan.ReplanCount + 1,
		Status:      contracts.PlanStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 收集失败步骤 ID 和已完成步骤 ID
	var failedStepIDs, completedStepIDs []contracts.ID
	for _, step := range failedSteps {
		failedStepIDs = append(failedStepIDs, step.ID)
	}
	for _, step := range completedSteps {
		completedStepIDs = append(completedStepIDs, step.ID)
	}

	return &ReplanResult{
		NewPlan:          replanPlan,
		Reason:           reason,
		FailedStepIDs:    failedStepIDs,
		CompletedStepIDs: completedStepIDs,
		StepMapping:      stepMapping,
		Version:          newVersion,
	}, usage, nil
}

// buildReplanReason 构建重规划原因
func (s *ReplanService) buildReplanReason(plan *contracts.Plan, completedSteps, failedSteps []contracts.PlanStep) string {
	var reasons []string

	if len(failedSteps) > 0 {
		var failedDescriptions []string
		for _, step := range failedSteps {
			failedDescriptions = append(failedDescriptions, fmt.Sprintf("步骤%d(%s)", step.StepNumber, step.Title))
		}
		reasons = append(reasons, fmt.Sprintf("以下步骤执行失败: %s", strings.Join(failedDescriptions, ", ")))
	}

	if plan.ReplanCount > 0 {
		reasons = append(reasons, fmt.Sprintf("这是第 %d 次重规划", plan.ReplanCount+1))
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "计划需要重新规划")
	}

	return strings.Join(reasons, "; ")
}

// buildReplanPrompt 构建重规划提示词。
func (s *ReplanService) buildReplanPrompt(plan *contracts.Plan, completedSteps, failedSteps []contracts.PlanStep, request contracts.AgentRunRequest) string {
	var sb strings.Builder

	sb.WriteString("你是一个任务规划专家。之前的计划执行失败，需要重新规划。\n\n")

	// 添加原始目标
	sb.WriteString("## 原始目标\n")
	sb.WriteString(plan.Goal + "\n\n")

	// 添加已完成步骤
	if len(completedSteps) > 0 {
		sb.WriteString("## 已完成的步骤\n")
		for _, step := range completedSteps {
			sb.WriteString(fmt.Sprintf("### 步骤 %d: %s\n", step.StepNumber, step.Title))
			sb.WriteString("状态: 已完成\n")
			if step.Output != "" {
				sb.WriteString("输出: " + step.Output + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// 添加失败步骤
	if len(failedSteps) > 0 {
		sb.WriteString("## 失败的步骤\n")
		for _, step := range failedSteps {
			sb.WriteString(fmt.Sprintf("### 步骤 %d: %s\n", step.StepNumber, step.Title))
			sb.WriteString("状态: 失败\n")
			if step.Error != "" {
				sb.WriteString("错误: " + step.Error + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// 添加可用工具（包含名称和描述）
	sb.WriteString("## 可用工具\n")
	if len(request.Context.AllowedTools) > 0 {
		for _, tool := range request.Context.AllowedTools {
			desc := request.Context.ToolDescriptions[tool]
			if desc != "" {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", tool, desc))
			} else {
				sb.WriteString("- " + tool + "\n")
			}
		}
	} else {
		sb.WriteString("无可用工具\n")
	}
	sb.WriteString("\n")
	appendPlannerToolCatalog(&sb, request.Context.AvailableTools)

	// 添加输出格式说明
	sb.WriteString("## 输出格式\n")
	sb.WriteString("请生成新的执行计划，只包含未完成的步骤。已完成的步骤将保留。\n")
	sb.WriteString("必须输出有效的 JSON 格式。\n")
	sb.WriteString("计划步骤数不超过 " + fmt.Sprintf("%d", plan.MaxSteps-len(completedSteps)) + " 步。\n\n")

	// 添加 URL 使用警告
	sb.WriteString("## 重要警告\n")
	sb.WriteString("1. **禁止编造 URL**：不要凭空生成 URL。如果需要访问网页，必须使用 web_search 工具搜索获取真实 URL。\n")
	sb.WriteString("2. **禁止使用示例 URL**：不要使用 example.com、test.com 等示例域名。\n")
	sb.WriteString("3. **依赖步骤输出**：如果需要使用其他步骤的输出（如搜索结果中的 URL），请使用结构化绑定语法：\n")
	sb.WriteString("   ```json\n")
	sb.WriteString("   \"url\": {\"$from_step\": 1, \"$path\": \"structured_data.items[0].url\"}\n")
	sb.WriteString("   ```\n")
	sb.WriteString("   其中 step 1 必须是 web_search 工具步骤，且必须在 depends_on 中列出。\n")
	sb.WriteString("4. **参数校验**：调用 fetch_url 前，确保 URL 参数是合法的 HTTP/HTTPS URL，不包含 JSON 或未解析的表达式。\n\n")

	sb.WriteString("严格按以下 JSON Schema 输出，字段类型不可更改：\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{
  "goal": "用户目标的一句话概括",
  "final_answer": "最终答案文本（字符串）",
  "steps": [
    {
      "step_number": 1,
      "title": "步骤标题",
      "description": "步骤详细描述",
      "kind": "tool or reasoning",
      "tool_policy": "required, preferred, or forbidden",
      "required_capabilities": ["web.fetch"],
      "tool_name": "工具名称（字符串，不调用工具则为空字符串）",
      "arguments": {},
      "depends_on": []
    }
  ]
}` + "\n")
	sb.WriteString("```\n\n")
	appendPlanStepContract(&sb)
	sb.WriteString("核心规划原则：\n")
	sb.WriteString("1. 任何需要外部信息的步骤（如搜索网页、读取文档、查询数据等），必须调用对应工具获取原始数据，禁止在未获取数据的情况下直接推理或生成事实性内容。\n")
	sb.WriteString("2. 所有事实性内容必须来自工具返回的结果，禁止凭空编造或基于推测生成。\n")
	sb.WriteString("3. 如果用户问题涉及 URL 或外部资源，必须使用对应工具读取完整内容后再总结。\n")
	sb.WriteString("\n")
	sb.WriteString("字段说明：\n")
	sb.WriteString("- steps 是数组，每个元素是一个步骤对象\n")
	sb.WriteString("- step_number 是整数，从 1 开始\n")
	sb.WriteString("- tool_name：需要调用工具时填写工具名称，不调用工具时设为空字符串 \"\"\n")
	sb.WriteString("- arguments：工具调用参数（JSON 对象），不调用工具时设为空对象 {}\n")
	sb.WriteString("- arguments may reference a dependency's real output with {{step_output:N}}; step N must also be listed in depends_on\n")
	sb.WriteString("- 对于搜索结果中的 URL，使用结构化绑定：{\"$from_step\": 1, \"$path\": \"structured_data.items[0].url\"}\n")
	sb.WriteString("- depends_on：依赖步骤的 step_number 字符串数组（如 [\"1\", \"2\"]），无依赖则为空数组 []\n")
	sb.WriteString("- final_answer：默认设为空字符串 \"\"，实际答案由步骤执行后生成；仅当所有步骤均为纯推理步骤且无工具调用时，可在此字段直接给出最终答案\n")

	return sb.String()
}

// mergeStepsWithMapping 合并旧步骤和新步骤，并记录新旧步骤映射。
func (s *ReplanService) mergeStepsWithMapping(oldSteps, newSteps []contracts.PlanStep) ([]contracts.PlanStep, map[contracts.ID]contracts.ID) {
	// 创建已完成步骤的映射
	completedMap := make(map[contracts.ID]contracts.PlanStep)
	for _, step := range oldSteps {
		if step.Status == contracts.PlanStepStatusCompleted {
			completedMap[step.ID] = step
		}
	}

	// 合并步骤
	var merged []contracts.PlanStep
	stepMapping := make(map[contracts.ID]contracts.ID) // 旧ID -> 新ID

	// 首先添加所有已完成的步骤（保留原始 ID）
	for _, step := range oldSteps {
		if step.Status == contracts.PlanStepStatusCompleted {
			merged = append(merged, step)
			// 已完成步骤的映射是自己
			stepMapping[step.ID] = step.ID
		}
	}

	// 然后添加新步骤（重规划后的步骤）
	stepNumber := len(merged) + 1
	for _, step := range newSteps {
		// 为新步骤生成新的 ID
		newStep := step
		newStep.StepNumber = stepNumber
		newStep.Status = contracts.PlanStepStatusPending
		merged = append(merged, newStep)
		stepNumber++
	}

	return merged, stepMapping
}
