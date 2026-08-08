package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
)

// llmCoreferenceResolver 使用 LLM 进行指代消解。
type llmCoreferenceResolver struct {
	modelFactory contracts.ModelFactory
	modelID      contracts.ID
}

// NewLLMCoreferenceResolver 创建 LLM 指代消解器。
func NewLLMCoreferenceResolver(modelFactory contracts.ModelFactory, modelID contracts.ID) contracts.CoreferenceResolver {
	return &llmCoreferenceResolver{
		modelFactory: modelFactory,
		modelID:      modelID,
	}
}

// Resolve 对消息列表进行指代消解。
func (r *llmCoreferenceResolver) Resolve(ctx context.Context, messages []contracts.ConversationMessage) ([]contracts.ConversationMessage, error) {
	if len(messages) <= 1 {
		return messages, nil
	}

	// 构建 Prompt
	prompt := r.buildPrompt(messages)

	// 调用 LLM
	model, err := r.modelFactory.GetChatModel(ctx, r.modelID)
	if err != nil {
		return nil, fmt.Errorf("get chat model: %w", err)
	}

	resp, err := model.Generate(ctx, contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: coreferenceSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	// 解析结果
	rewritten, err := r.parseResponse(resp.Content, messages)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return rewritten, nil
}

const coreferenceSystemPrompt = `你是一个对话指代消解助手。请将用户消息中的代词和省略主语替换为明确的指代。

规则：
1. 保持消息顺序和角色不变
2. 只改写用户消息中的指代
3. 不要添加新信息
4. 输出 JSON 格式，每条消息包含 role 和 content 字段

示例输出：
[
  {"role": "user", "content": "我在做一个知识库系统"},
  {"role": "assistant", "content": "好的，知识库系统"},
  {"role": "user", "content": "用什么技术栈开发知识库系统？"}
]`

// buildPrompt 构建指代消解 Prompt。
func (r *llmCoreferenceResolver) buildPrompt(messages []contracts.ConversationMessage) string {
	var sb strings.Builder

	sb.WriteString("对话历史：\n")

	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
	}

	sb.WriteString("\n请对每条用户消息进行指代消解，输出改写后的完整消息列表（JSON 格式）：")

	return sb.String()
}

// parseResponse 解析 LLM 响应。
func (r *llmCoreferenceResolver) parseResponse(resp string, original []contracts.ConversationMessage) ([]contracts.ConversationMessage, error) {
	// 提取 JSON 部分
	start := strings.Index(resp, "[")
	end := strings.LastIndex(resp, "]")
	if start == -1 || end == -1 {
		return original, nil
	}

	jsonStr := resp[start : end+1]

	// 简单解析 JSON（项目中可使用 encoding/json）
	var rewritten []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	// 使用正则提取
	rolePattern := regexp.MustCompile(`"role"\s*:\s*"([^"]+)"`)
	contentPattern := regexp.MustCompile(`"content"\s*:\s*"([^"]*)"`)

	roles := rolePattern.FindAllStringSubmatch(jsonStr, -1)
	contents := contentPattern.FindAllStringSubmatch(jsonStr, -1)

	for i := 0; i < len(roles) && i < len(contents); i++ {
		rewritten = append(rewritten, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			Role:    roles[i][1],
			Content: contents[i][1],
		})
	}

	// 合并结果
	result := make([]contracts.ConversationMessage, len(original))
	for i, msg := range original {
		result[i] = msg
		if msg.Role == "user" && i < len(rewritten) && rewritten[i].Role == "user" {
			result[i].Content = rewritten[i].Content
		}
	}

	return result, nil
}

// ruleBasedResolver 使用规则进行指代消解。
type ruleBasedResolver struct {
	pronounPatterns []*regexp.Regexp
}

// NewRuleBasedResolver 创建规则指代消解器。
func NewRuleBasedResolver() contracts.CoreferenceResolver {
	return &ruleBasedResolver{
		pronounPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(它|这个|那个|这|那)(是|表示|指|代表)`),
			regexp.MustCompile(`(用|使用|采用)(它|这个|那个)`),
			regexp.MustCompile(`(关于|对于)(它|这个|那个)`),
		},
	}
}

// Resolve 对消息列表进行指代消解。
func (r *ruleBasedResolver) Resolve(ctx context.Context, messages []contracts.ConversationMessage) ([]contracts.ConversationMessage, error) {
	if len(messages) <= 1 {
		return messages, nil
	}

	// 提取历史实体
	entities := r.extractEntities(messages)

	// 替换指代
	result := make([]contracts.ConversationMessage, len(messages))
	for i, msg := range messages {
		result[i] = msg
		if msg.Role == "user" && len(entities) > 0 {
			result[i].Content = r.replacePronouns(msg.Content, entities)
		}
	}

	return result, nil
}

// extractEntities 提取消息中的实体。
func (r *ruleBasedResolver) extractEntities(messages []contracts.ConversationMessage) []string {
	var entities []string

	for _, msg := range messages {
		if msg.Role == "assistant" {
			// 从助手回复中提取可能的实体（简单实现）
			words := strings.Fields(msg.Content)
			for _, word := range words {
				if len(word) > 2 && !strings.ContainsAny(word, "，。！？、") {
					entities = append(entities, word)
				}
			}
		}
	}

	return entities
}

// replacePronouns 替换代词为实体。
func (r *ruleBasedResolver) replacePronouns(text string, entities []string) string {
	if len(entities) == 0 {
		return text
	}

	// 使用最近的实体替换代词
	latestEntity := entities[len(entities)-1]

	for _, pattern := range r.pronounPatterns {
		text = pattern.ReplaceAllStringFunc(text, func(match string) string {
			// 替换代词部分
			return strings.Replace(match, "它", latestEntity, 1)
		})
	}

	return text
}
