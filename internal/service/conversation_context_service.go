package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

// 会话上下文服务的默认配置。
const (
	DefaultMaxTokens   = 6000 // 会话上下文最大 Token 数
	DefaultWindowSize  = 25   // 保留的近期消息数
	DefaultMaxMessages = 50   // 读取的历史消息上限
)

// conversationContextService 实现 ConversationContextService 接口。
type conversationContextService struct {
	messageRepo  repository.MessageRepository
	resolver     contracts.CoreferenceResolver
	tokenCounter contracts.TokenCounter
	maxTokens    int
	windowSize   int
	maxMessages  int
}

// NewConversationContextService 创建会话上下文服务。
func NewConversationContextService(
	messageRepo repository.MessageRepository,
	resolver contracts.CoreferenceResolver,
	tokenCounter contracts.TokenCounter,
) contracts.ConversationContextService {
	return &conversationContextService{
		messageRepo:  messageRepo,
		resolver:     resolver,
		tokenCounter: tokenCounter,
		maxTokens:    DefaultMaxTokens,
		windowSize:   DefaultWindowSize,
		maxMessages:  DefaultMaxMessages,
	}
}

// Build 为指定的会话构建 ConversationContext。
func (s *conversationContextService) Build(
	ctx context.Context,
	userID, knowledgeBaseID, conversationID contracts.ID,
) (contracts.ConversationContext, error) {
	// 1. 读取会话历史消息
	messages, err := s.messageRepo.ListByConversation(ctx, string(conversationID), s.maxMessages, 0)
	if err != nil {
		return contracts.ConversationContext{}, fmt.Errorf("list messages: %w", err)
	}

	if len(messages) == 0 {
		return contracts.ConversationContext{
			ConversationID: conversationID,
			Messages:       []contracts.ConversationMessage{},
			TokenCount:     0,
		}, nil
	}

	// 2. 转换为 contracts 格式
	contractMessages := make([]contracts.ConversationMessage, len(messages))
	for i, msg := range messages {
		contractMessages[i] = contracts.ConversationMessage{
			ID:        contracts.ID(msg.ID),
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		}
	}

	// 3. 滑动窗口截断
	recentMessages := s.truncateByWindow(contractMessages, s.windowSize)

	// 4. 被截断的消息生成摘要
	truncatedMessages := contractMessages[:len(contractMessages)-len(recentMessages)]
	summary := ""
	if len(truncatedMessages) > 0 {
		summary = s.generateSummary(truncatedMessages)
	}

	// 5. 指代补全
	if s.resolver != nil && len(recentMessages) > 1 {
		resolved, err := s.resolver.Resolve(ctx, recentMessages)
		if err != nil {
			// 指代补全失败，降级使用原始消息
			logger.Warn("coreference resolution failed, using raw messages", zap.Error(err))
		} else {
			recentMessages = resolved
		}
	}

	// 6. Token 截断
	recentMessages = s.truncateByToken(recentMessages)

	// 7. 计算总 Token 数
	totalTokens := s.calculateTokenCount(recentMessages, summary)

	return contracts.ConversationContext{
		ConversationID: conversationID,
		Messages:       recentMessages,
		Summary:        summary,
		TokenCount:     totalTokens,
	}, nil
}

// truncateByWindow 按窗口大小截断消息，保留最新的消息。
func (s *conversationContextService) truncateByWindow(messages []contracts.ConversationMessage, windowSize int) []contracts.ConversationMessage {
	if len(messages) <= windowSize {
		return messages
	}
	return messages[len(messages)-windowSize:]
}

// truncateByToken 按 Token 上限截断消息。
func (s *conversationContextService) truncateByToken(messages []contracts.ConversationMessage) []contracts.ConversationMessage {
	if len(messages) == 0 {
		return messages
	}

	var result []contracts.ConversationMessage
	totalTokens := 0

	// 从最新消息开始，向前遍历
	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := s.tokenCounter.Count(messages[i].Content)

		if totalTokens+msgTokens > s.maxTokens {
			// 超过限制，停止添加
			break
		}

		result = append(result, messages[i])
		totalTokens += msgTokens
	}

	// 反转，使消息按时间顺序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result
}

// generateSummary 生成被截断消息的摘要。
func (s *conversationContextService) generateSummary(messages []contracts.ConversationMessage) string {
	if len(messages) == 0 {
		return ""
	}

	// 简单实现：提取关键信息
	var sb strings.Builder
	sb.WriteString("历史对话摘要：")

	// 提取用户的问题和助手的回答
	userQuestions := []string{}
	assistantAnswers := []string{}

	for _, msg := range messages {
		if msg.Role == "user" {
			// 截取前50个字符
			content := msg.Content
			if len(content) > 50 {
				content = content[:50] + "..."
			}
			userQuestions = append(userQuestions, content)
		} else if msg.Role == "assistant" {
			// 截取前30个字符
			content := msg.Content
			if len(content) > 30 {
				content = content[:30] + "..."
			}
			assistantAnswers = append(assistantAnswers, content)
		}
	}

	if len(userQuestions) > 0 {
		sb.WriteString("用户询问了")
		sb.WriteString(strings.Join(userQuestions, "、"))
		sb.WriteString("。")
	}

	if len(assistantAnswers) > 0 {
		sb.WriteString("助手回答了")
		sb.WriteString(strings.Join(assistantAnswers, "、"))
		sb.WriteString("。")
	}

	return sb.String()
}

// calculateTokenCount 计算消息列表和摘要的总 Token 数。
func (s *conversationContextService) calculateTokenCount(messages []contracts.ConversationMessage, summary string) int {
	total := 0

	// 计算摘要的 Token
	if summary != "" {
		total += s.tokenCounter.Count(summary)
	}

	// 计算消息的 Token
	for _, msg := range messages {
		total += s.tokenCounter.Count(msg.Content)
	}

	return total
}
