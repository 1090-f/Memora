package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// ReviewerService 负责审查计划执行结果的完整性和正确性。
type ReviewerService struct {
	modelFactory contracts.ModelFactory
}

// NewReviewerService 创建 ReviewerService 实例。
func NewReviewerService(modelFactory contracts.ModelFactory) *ReviewerService {
	return &ReviewerService{
		modelFactory: modelFactory,
	}
}

// Review 审查计划执行结果。
func (s *ReviewerService) Review(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) (*contracts.ReviewerResult, error) {
	// 1. 构建审查提示词
	prompt := s.buildReviewPrompt(plan, request)

	// 2. 获取 ChatModel
	chatModel, err := s.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		logger.Error("Reviewer 获取 ChatModel 失败", zap.Error(err))
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
		logger.Error("Reviewer LLM 调用失败", zap.Error(err))
		return nil, fmt.Errorf("review: %w", err)
	}

	logger.Debug("Reviewer LLM 响应",
		zap.String("content", response.Content),
	)

	// 5. 解析结构化 JSON 输出
	result, err := parseReviewerResponse(response.Content)
	if err != nil {
		// JSON 解析失败，降级为简单 PASS/FAIL 解析
		logger.Warn("Reviewer JSON 解析失败，降级为简单解析", zap.Error(err))
		return parseSimpleReviewerResponse(response.Content), nil
	}

	return result, nil
}

// reviewerResponse LLM 返回的结构化审查结果
type reviewerResponse struct {
	Approved   bool   `json:"approved"`
	Issues     string `json:"issues"`
	Suggestion string `json:"suggestion"`
	FactCheck  struct {
		Consistent        bool     `json:"consistent"`
		InconsistentFacts []string `json:"inconsistent_facts"`
	} `json:"fact_check"`
}

// parseReviewerResponse 解析 LLM 返回的结构化 JSON
func parseReviewerResponse(content string) (*contracts.ReviewerResult, error) {
	// 尝试提取 JSON（可能被 ```json ``` 包裹）
	jsonStr := extractJSONFromResponse(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var resp reviewerResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal reviewer response: %w", err)
	}

	result := &contracts.ReviewerResult{
		Approved:   resp.Approved,
		Issues:     resp.Issues,
		Suggestion: resp.Suggestion,
	}

	if len(resp.FactCheck.InconsistentFacts) > 0 {
		result.FactCheck = &contracts.FactCheckResult{
			Consistent:        resp.FactCheck.Consistent,
			InconsistentFacts: resp.FactCheck.InconsistentFacts,
		}
	}

	return result, nil
}

// parseSimpleReviewerResponse 降级解析：简单 PASS/FAIL 格式
func parseSimpleReviewerResponse(content string) *contracts.ReviewerResult {
	content = strings.TrimSpace(content)
	contentUpper := strings.ToUpper(content)

	if strings.HasPrefix(contentUpper, "PASS") || strings.Contains(contentUpper, "PASS") {
		return &contracts.ReviewerResult{Approved: true}
	}

	if strings.HasPrefix(contentUpper, "FAIL") {
		issues := content
		if idx := strings.Index(content, ":"); idx > 0 {
			issues = strings.TrimSpace(content[idx+1:])
		}
		return &contracts.ReviewerResult{Approved: false, Issues: issues}
	}

	// 默认通过
	logger.Warn("Reviewer 返回格式未知，默认通过", zap.String("response", content))
	return &contracts.ReviewerResult{Approved: true}
}

// extractJSONFromResponse 从 LLM 响应中提取 JSON
func extractJSONFromResponse(text string) string {
	// 尝试 ```json ... ```
	if idx := strings.Index(text, "```json"); idx != -1 {
		start := idx + len("```json")
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	// 尝试 ``` ... ```
	if idx := strings.Index(text, "```"); idx != -1 {
		start := idx + len("```")
		// 跳过语言标识行
		if nl := strings.IndexByte(text[start:], '\n'); nl != -1 {
			start += nl + 1
		}
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	// 尝试裸 JSON
	depth := 0
	start := -1
	for i, ch := range text {
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 && start >= 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

// buildReviewPrompt 构建审查提示词。
func (s *ReviewerService) buildReviewPrompt(plan *contracts.Plan, request contracts.AgentRunRequest) string {
	var sb strings.Builder

	sb.WriteString("你是一个任务审查专家。请从以下维度审查执行结果。\n\n")

	// 添加计划目标
	sb.WriteString("## 计划目标\n")
	sb.WriteString(plan.Goal + "\n\n")

	// 添加执行结果（包含完整工具输出，用于事实核查）
	sb.WriteString("## 执行结果\n")
	for _, step := range plan.Steps {
		sb.WriteString(fmt.Sprintf("- 步骤%d: %s [%s]\n", step.StepNumber, step.Title, string(step.Status)))
		if step.Output != "" {
			// 保留更多内容用于事实核查（最多 500 字符）
			output := step.Output
			if len(output) > 500 {
				output = output[:500] + "..."
			}
			sb.WriteString("  输出: " + output + "\n")
		}
	}
	sb.WriteString("\n")

	// 添加最终答案
	sb.WriteString("## 最终答案\n")
	sb.WriteString(plan.FinalAnswer + "\n\n")

	// 添加用户原始问题
	sb.WriteString("## 用户原始问题\n")
	sb.WriteString(request.Context.Query + "\n\n")

	// 审查维度说明
	sb.WriteString("审查维度：\n")
	sb.WriteString("1. 完整性：是否完整回答了用户的问题\n")
	sb.WriteString("2. 事实一致性：回答中的关键事实（技术栈、功能列表、项目名称等）是否与执行结果中的源内容一致\n")
	sb.WriteString("3. 无编造：回答中是否包含源内容中不存在的信息\n")
	sb.WriteString("4. 无模板残留：回答中是否包含占位符（如 [请在此处填入...]）或模板标题\n\n")

	// 输出格式要求
	sb.WriteString("必须输出有效的 JSON 格式，不要包含任何其他文本：\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{
  "approved": true,
  "issues": "存在的问题描述（approved为true时为空字符串）",
  "suggestion": "改进建议（可选）",
  "fact_check": {
    "consistent": true,
    "inconsistent_facts": ["不一致的事实1", "不一致的事实2"]
  }
}` + "\n")
	sb.WriteString("```\n")

	return sb.String()
}
