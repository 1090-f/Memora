package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
// 支持超时、重试和状态记录。
func (s *ReviewerService) Review(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) (*contracts.ReviewerResult, error) {
	// 1. 构建审查提示词
	prompt := s.buildReviewPrompt(plan, request)

	// 2. 获取 ChatModel
	chatModel, err := s.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		logger.Error("Reviewer 获取 ChatModel 失败", zap.Error(err))
		return &contracts.ReviewerResult{
			Approved: false,
			Issues:   fmt.Sprintf("Reviewer 获取 ChatModel 失败: %v", err),
		}, nil
	}

	// 3. 构建请求
	chatRequest := contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: request.Context.Query},
		},
	}

	// 4. 调用 LLM（支持超时和重试）
	maxRetries := 1
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 创建带超时的 context（Reviewer 专用超时：20 秒）
		reviewerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)

		// 调用 LLM
		response, err := chatModel.Generate(reviewerCtx, chatRequest)
		cancel() // 立即释放资源

		if err != nil {
			lastErr = err
			// 如果是超时错误且还有重试次数，继续重试
			if attempt < maxRetries && (reviewerCtx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "deadline exceeded")) {
				continue
			}
			logger.Error("Reviewer LLM 调用失败", zap.Error(err))
			return &contracts.ReviewerResult{
				Approved: false,
				Issues:   fmt.Sprintf("Reviewer LLM 调用失败: %v", err),
			}, nil
		}

		logger.Debug("Reviewer LLM 响应",
			zap.String("content", response.Content),
		)

		// 5. 解析结构化 JSON 输出
		result, err := parseReviewerResponse(response.Content)
		if err != nil {
			// JSON 解析失败，降级为简单 PASS/FAIL 解析
			logger.Warn("Reviewer JSON 解析失败，降级为简单解析", zap.Error(err))
			result = parseSimpleReviewerResponse(response.Content)
		}

		return result, nil
	}

	// 如果所有重试都失败，返回错误状态
	logger.Error("Reviewer 执行失败", zap.Error(lastErr))
	return &contracts.ReviewerResult{
		Approved: false,
		Issues:   fmt.Sprintf("Reviewer 执行失败: %v", lastErr),
	}, nil
}

// ReviewWithUsage 审查计划并返回 token 使用量。
func (s *ReviewerService) ReviewWithUsage(ctx context.Context, plan *contracts.Plan, request contracts.AgentRunRequest) (*contracts.ReviewerResult, contracts.TokenUsage, error) {
	// 1. 构建审查提示词
	prompt := s.buildReviewPrompt(plan, request)

	// 2. 获取 ChatModel
	chatModel, err := s.modelFactory.GetChatModel(ctx, contracts.ID(request.Context.ChatModelID))
	if err != nil {
		logger.Error("Reviewer 获取 ChatModel 失败", zap.Error(err))
		return &contracts.ReviewerResult{
			Approved: false,
			Issues:   fmt.Sprintf("Reviewer 获取 ChatModel 失败: %v", err),
		}, contracts.TokenUsage{}, nil
	}

	// 3. 构建请求
	chatRequest := contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: request.Context.Query},
		},
	}

	// 4. 调用 LLM（支持超时和重试）
	maxRetries := 1
	var lastErr error
	totalUsage := contracts.TokenUsage{}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 创建带超时的 context（Reviewer 专用超时：20 秒）
		reviewerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)

		// 调用 LLM
		response, err := chatModel.Generate(reviewerCtx, chatRequest)
		cancel() // 立即释放资源

		if err != nil {
			lastErr = err
			// 如果是超时错误且还有重试次数，继续重试
			if attempt < maxRetries && (reviewerCtx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "deadline exceeded")) {
				continue
			}
			logger.Error("Reviewer LLM 调用失败", zap.Error(err))
			return &contracts.ReviewerResult{
				Approved: false,
				Issues:   fmt.Sprintf("Reviewer LLM 调用失败: %v", err),
			}, totalUsage, nil
		}

		// 累加 token 使用量
		totalUsage.Add(response.Usage)

		logger.Debug("Reviewer LLM 响应",
			zap.String("content", response.Content),
		)

		// 5. 解析结构化 JSON 输出
		result, err := parseReviewerResponse(response.Content)
		if err != nil {
			// JSON 解析失败，降级为简单 PASS/FAIL 解析
			logger.Warn("Reviewer JSON 解析失败，降级为简单解析", zap.Error(err))
			result = parseSimpleReviewerResponse(response.Content)
		}

		return result, totalUsage, nil
	}

	// 如果所有重试都失败，返回错误状态
	logger.Error("Reviewer 执行失败", zap.Error(lastErr))
	return &contracts.ReviewerResult{
		Approved: false,
		Issues:   fmt.Sprintf("Reviewer 执行失败: %v", lastErr),
	}, totalUsage, nil
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

	// 添加执行结果（使用结构化完整摘要，而不是截断内容）
	sb.WriteString("## 执行结果\n")
	for _, step := range plan.Steps {
		sb.WriteString(fmt.Sprintf("- 步骤%d: %s [%s]\n", step.StepNumber, step.Title, string(step.Status)))
		if step.Output != "" {
			// 提取步骤输出的结构化摘要
			summary := extractStepSummary(step)
			sb.WriteString("  摘要: " + summary + "\n")
		}
		if step.Error != "" {
			sb.WriteString("  错误: " + step.Error + "\n")
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
	sb.WriteString("4. 无模板残留：回答中是否包含占位符（如 [请在此处填入...]）或模板标题\n")
	sb.WriteString("5. 证据支持：回答中的主要结论是否有对应的工具执行证据支持\n\n")

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

// extractStepSummary 提取步骤的结构化摘要
func extractStepSummary(step contracts.PlanStep) string {
	var sb strings.Builder

	// 添加工具名称（如果有）
	if step.ToolName != "" {
		sb.WriteString(fmt.Sprintf("工具: %s, ", step.ToolName))
	}

	// 提取输出的文本内容
	if step.Output != "" {
		text := extractStepOutputTextFromJSON(step.Output)
		if text != "" {
			// 截取前 1000 字符作为摘要（比之前的 500 字符更多）
			if len(text) > 1000 {
				text = text[:1000] + "..."
			}
			sb.WriteString(text)
		} else {
			sb.WriteString("输出为空或无法解析")
		}
	} else {
		sb.WriteString("无输出")
	}

	return sb.String()
}

// extractStepOutputTextFromJSON 从步骤输出中提取文本内容
func extractStepOutputTextFromJSON(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}

	// 如果不是 JSON 格式，直接返回
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return output
	}

	// 尝试解析为 ToolResult JSON
	var toolResult contracts.ToolResult
	if err := json.Unmarshal([]byte(trimmed), &toolResult); err == nil {
		// 检查是否真的是 ToolResult 格式（有 call_id 或 tool_name 字段）
		isToolResult := toolResult.CallID != "" || toolResult.ToolName != ""
		if isToolResult {
			if toolResult.Text != "" {
				return toolResult.Text
			}
			if len(toolResult.StructuredData) > 0 {
				return string(toolResult.StructuredData)
			}
			return ""
		}
	}

	// 尝试解析为其他 JSON 格式
	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &jsonResult); err == nil {
		// 尝试提取常见的文本字段
		if text, ok := jsonResult["text"].(string); ok && text != "" {
			return text
		}
		if content, ok := jsonResult["content"].(string); ok && content != "" {
			return content
		}
		if message, ok := jsonResult["message"].(string); ok && message != "" {
			return message
		}
	}

	// 如果无法解析，返回原文本（截取前 200 字符）
	if len(trimmed) > 200 {
		return trimmed[:200] + "..."
	}
	return trimmed
}
