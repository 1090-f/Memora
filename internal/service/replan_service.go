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

// Replan 重新规划未完成的步骤。
func (s *ReplanService) Replan(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) (*contracts.Plan, error) {
	// 1. 检查是否超过重规划次数限制
	if plan.ReplanCount >= plan.MaxReplans {
		return plan, fmt.Errorf("replan limit reached: %d/%d", plan.ReplanCount, plan.MaxReplans)
	}

	// 2. 收集已完成步骤的输出
	completedSteps := plan.GetCompletedSteps()
	failedSteps := plan.GetFailedSteps()

	// 3. 构建重规划提示词
	prompt := s.buildReplanPrompt(plan, completedSteps, failedSteps, request)

	// 4. 使用 PlannerService 生成新计划
	newPlan, err := s.planner.PlanWithPrompt(ctx, request, prompt)
	if err != nil {
		return nil, fmt.Errorf("replan: %w", err)
	}

	// 5. 合并步骤：保留已完成步骤，更新未完成步骤
	mergedSteps := s.mergeSteps(plan.Steps, newPlan.Steps)
	if len(completedSteps) == 0 && len(mergedSteps) == 0 {
		return nil, fmt.Errorf("replan produced empty plan with no completed steps")
	}
	plan.Steps = mergedSteps
	plan.ReplanCount++
	plan.UpdatedAt = time.Now()

	return plan, nil
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
	sb.WriteString("- depends_on：依赖步骤的 step_number 字符串数组（如 [\"1\", \"2\"]），无依赖则为空数组 []\n")
	sb.WriteString("- final_answer：默认设为空字符串 \"\"，实际答案由步骤执行后生成；仅当所有步骤均为纯推理步骤且无工具调用时，可在此字段直接给出最终答案\n")

	return sb.String()
}

// mergeSteps 合并旧步骤和新步骤。
func (s *ReplanService) mergeSteps(oldSteps, newSteps []contracts.PlanStep) []contracts.PlanStep {
	// 创建已完成步骤的映射
	completedMap := make(map[contracts.ID]contracts.PlanStep)
	for _, step := range oldSteps {
		if step.Status == contracts.PlanStepStatusCompleted {
			completedMap[step.ID] = step
		}
	}

	// 合并步骤
	var merged []contracts.PlanStep

	// 首先添加所有已完成的步骤
	for _, step := range oldSteps {
		if step.Status == contracts.PlanStepStatusCompleted {
			merged = append(merged, step)
		}
	}

	// 然后添加新步骤（重规划后的步骤）
	stepNumber := len(merged) + 1
	for _, step := range newSteps {
		step.StepNumber = stepNumber
		step.Status = contracts.PlanStepStatusPending
		merged = append(merged, step)
		stepNumber++
	}

	return merged
}
