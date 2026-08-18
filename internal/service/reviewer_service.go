package service

import (
	"context"
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
			{Role: "user", Content: plan.FinalAnswer},
		},
	}

	// 4. 调用 LLM
	response, err := chatModel.Generate(ctx, chatRequest)
	if err != nil {
		logger.Error("Reviewer LLM 调用失败", zap.Error(err))
		return nil, fmt.Errorf("review: %w", err)
	}

	logger.Info("Reviewer LLM 响应",
		zap.String("content", response.Content),
	)

	// 5. 解析简单格式：PASS 或 FAIL
	content := strings.TrimSpace(response.Content)
	contentUpper := strings.ToUpper(content)

	if strings.HasPrefix(contentUpper, "PASS") || strings.Contains(contentUpper, "PASS") {
		return &contracts.ReviewerResult{
			Approved:   true,
			Issues:     "",
			Suggestion: "",
		}, nil
	}

	// FAIL 格式：FAIL: 具体问题
	if strings.HasPrefix(contentUpper, "FAIL") {
		issues := content
		if idx := strings.Index(content, ":"); idx > 0 {
			issues = strings.TrimSpace(content[idx+1:])
		}
		return &contracts.ReviewerResult{
			Approved:   false,
			Issues:     issues,
			Suggestion: "",
		}, nil
	}

	// 默认通过
	logger.Warn("Reviewer 返回格式未知，默认通过",
		zap.String("response", content),
	)
	return &contracts.ReviewerResult{
		Approved:   true,
		Issues:     "",
		Suggestion: "",
	}, nil
}

// buildReviewPrompt 构建审查提示词。
func (s *ReviewerService) buildReviewPrompt(plan *contracts.Plan, request contracts.AgentRunRequest) string {
	var sb strings.Builder

	sb.WriteString("你是一个任务审查专家。请审查计划执行结果。\n\n")

	// 添加计划目标
	sb.WriteString("## 计划目标\n")
	sb.WriteString(plan.Goal + "\n\n")

	// 添加执行结果（简化版，只包含关键信息）
	sb.WriteString("## 执行结果\n")
	for _, step := range plan.Steps {
		sb.WriteString(fmt.Sprintf("- 步骤%d: %s [%s]\n", step.StepNumber, step.Title, string(step.Status)))
		if step.Output != "" {
			// 截断过长的输出
			output := step.Output
			if len(output) > 200 {
				output = output[:200] + "..."
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

	sb.WriteString("请判断结果是否完整回答了问题。")

	return sb.String()
}
