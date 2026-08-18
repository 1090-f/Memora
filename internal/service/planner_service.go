package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/google/uuid"
)

// PlannerService 负责使用 LLM 生成结构化执行计划。
type PlannerService struct {
	modelFactory contracts.ModelFactory
}

// NewPlannerService 创建 PlannerService 实例。
func NewPlannerService(modelFactory contracts.ModelFactory) *PlannerService {
	return &PlannerService{
		modelFactory: modelFactory,
	}
}

// Plan 根据用户查询和上下文生成执行计划。
func (s *PlannerService) Plan(ctx context.Context, request contracts.AgentRunRequest) (*contracts.Plan, error) {
	// 1. 构建提示词
	prompt := s.buildPrompt(request)

	// 2. 获取 ChatModel
	chatModel, err := s.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		return nil, fmt.Errorf("get chat model: %w", err)
	}

	// 3. 构建请求
	chatRequest := contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: request.Context.Query},
		},
	}

	// 4. 调用 LLM
	response, err := chatModel.Generate(ctx, chatRequest)
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
	}

	// 5. 提取 JSON
	jsonStr := extractJSONFromText(response.Content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	// 6. 解析计划
	var plan contracts.Plan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}

	// 7. 设置元数据
	plan.ID = contracts.ID(uuid.NewString())
	plan.RunID = request.RunID
	plan.MaxSteps = request.Config.MaxPlanSteps
	plan.MaxReplans = request.Config.MaxReplans
	plan.Status = contracts.PlanStatusPending
	// 不再强制清空 FinalAnswer：当步骤执行无法产出有效答案时（如缺少外部工具），
	// 保留 Planner LLM 基于先验知识生成的 final_answer 作为兜底，避免返回空答案。
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	// 8. 验证步骤数
	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}
	if len(plan.Steps) > plan.MaxSteps {
		return nil, fmt.Errorf("plan has %d steps, max allowed is %d", len(plan.Steps), plan.MaxSteps)
	}

	// 9. 为每个步骤设置状态和ID
	stepNumberToID := make(map[int]contracts.ID)
	for i := range plan.Steps {
		plan.Steps[i].ID = contracts.ID(uuid.NewString())
		plan.Steps[i].Status = contracts.PlanStepStatusPending
		stepNumberToID[plan.Steps[i].StepNumber] = plan.Steps[i].ID
	}

	// 10. 将 depends_on 中的步骤序号解析为对应的 step UUID
	for i := range plan.Steps {
		var resolvedDeps []contracts.ID
		for _, dep := range plan.Steps[i].DependsOn {
			if stepNum, err := strconv.Atoi(string(dep)); err == nil {
				if depID, ok := stepNumberToID[stepNum]; ok {
					resolvedDeps = append(resolvedDeps, depID)
				}
			} else {
				// 如果不是数字，可能是已经解析过的 UUID，直接保留
				resolvedDeps = append(resolvedDeps, dep)
			}
		}
		plan.Steps[i].DependsOn = resolvedDeps
	}

	return &plan, nil
}

// PlanWithPrompt 使用自定义提示词生成计划（用于重规划）。
func (s *PlannerService) PlanWithPrompt(ctx context.Context, request contracts.AgentRunRequest, customPrompt string) (*contracts.Plan, error) {
	// 1. 获取 ChatModel
	chatModel, err := s.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		return nil, fmt.Errorf("get chat model: %w", err)
	}

	// 2. 构建请求
	chatRequest := contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: customPrompt},
			{Role: "user", Content: request.Context.Query},
		},
	}

	// 3. 调用 LLM
	response, err := chatModel.Generate(ctx, chatRequest)
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
	}

	// 4. 提取 JSON
	jsonStr := extractJSONFromText(response.Content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	// 5. 解析计划
	var plan contracts.Plan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}

	// 5a. Replan 场景不允许空步骤
	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("replan produced 0 steps")
	}

	// 6. 为每个步骤设置状态和ID，并解析 depends_on
	stepNumberToID := make(map[int]contracts.ID)
	for i := range plan.Steps {
		plan.Steps[i].ID = contracts.ID(uuid.NewString())
		plan.Steps[i].Status = contracts.PlanStepStatusPending
		stepNumberToID[plan.Steps[i].StepNumber] = plan.Steps[i].ID
	}

	for i := range plan.Steps {
		var resolvedDeps []contracts.ID
		for _, dep := range plan.Steps[i].DependsOn {
			if stepNum, err := strconv.Atoi(string(dep)); err == nil {
				if depID, ok := stepNumberToID[stepNum]; ok {
					resolvedDeps = append(resolvedDeps, depID)
				}
			} else {
				resolvedDeps = append(resolvedDeps, dep)
			}
		}
		plan.Steps[i].DependsOn = resolvedDeps
	}

	return &plan, nil
}

// buildPrompt 构建 Planner 提示词。
func (s *PlannerService) buildPrompt(request contracts.AgentRunRequest) string {
	var sb strings.Builder

	sb.WriteString("你是一个任务规划专家。根据用户查询和上下文，生成结构化的执行计划。\n\n")

	// 添加可用工具信息（包含名称和描述）
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

	// 添加上下文信息
	sb.WriteString("## 上下文信息\n")
	sb.WriteString(request.Context.ToPromptWithTagsCompact())
	sb.WriteString("\n\n")

	// 添加输出格式说明（含 JSON Schema 示例）
	sb.WriteString("## 输出格式\n")
	sb.WriteString("必须输出有效的 JSON 格式，不要包含任何其他文本。\n")
	sb.WriteString("计划步骤数不超过 " + fmt.Sprintf("%d", request.Config.MaxPlanSteps) + " 步。\n\n")
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

// extractJSONFromText 从文本中提取 JSON 字符串。
func extractJSONFromText(text string) string {
	// 尝试找到 JSON 块（```json ... ```）
	startIdx := strings.Index(text, "```json")
	if startIdx != -1 {
		startIdx += len("```json")
		endIdx := strings.Index(text[startIdx:], "```")
		if endIdx != -1 {
			return strings.TrimSpace(text[startIdx : startIdx+endIdx])
		}
	}

	// 尝试找到裸 JSON（{ ... }）
	start := 0
	for i, ch := range text {
		if ch == '{' {
			start = i
			break
		}
	}

	if start == 0 && len(text) > 0 && text[0] != '{' {
		return ""
	}

	// 找到对应的闭合括号
	depth := 0
	for i := start; i < len(text); i++ {
		if text[i] == '{' {
			depth++
		} else if text[i] == '}' {
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}

	return ""
}
