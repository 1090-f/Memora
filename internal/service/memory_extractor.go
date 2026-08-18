package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// ExtractorPromptConfig 提取器Prompt配置。
type ExtractorPromptConfig struct {
	System string `yaml:"system"`
	User   string `yaml:"user"`
}

// memoryExtractor 实现 contracts.MemoryExtractor 接口。
type memoryExtractor struct {
	modelFactory  contracts.ModelFactory
	memoryManager contracts.MemoryManager
	promptConfig  ExtractorPromptConfig
}

// NewMemoryExtractor 创建 MemoryExtractor 实例。
func NewMemoryExtractor(
	modelFactory contracts.ModelFactory,
	memoryManager contracts.MemoryManager,
	promptConfigPath string,
) (contracts.MemoryExtractor, error) {
	// 加载Prompt配置
	config, err := loadExtractorPromptConfig(promptConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load extractor prompt config: %w", err)
	}

	return &memoryExtractor{
		modelFactory:  modelFactory,
		memoryManager: memoryManager,
		promptConfig:  *config,
	}, nil
}

// loadExtractorPromptConfig 加载提取器Prompt配置。
func loadExtractorPromptConfig(path string) (*ExtractorPromptConfig, error) {
	data, err := readFileContent(path)
	if err != nil {
		return nil, err
	}

	var config ExtractorPromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal extractor prompt config: %w", err)
	}

	return &config, nil
}

// readFileContent 读取文件内容。
func readFileContent(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return data, nil
}

// Extract 从Agent回答中提取记忆并存储。
// 注意：此函数是同步执行的，调用者应该在 goroutine 中调用以避免阻塞。
func (e *memoryExtractor) Extract(ctx context.Context, agentContext contracts.AgentContext, answer string) error {
	logger.Debug("[记忆提取-Extract] 开始提取记忆",
		zap.String("user_id", string(agentContext.UserID)),
		zap.String("chat_model_id", agentContext.ChatModelID),
		zap.String("query", agentContext.Query),
		zap.Int("answer_length", len(answer)),
	)

	defer func() {
		if r := recover(); r != nil {
			logger.Error("[记忆提取-Extract] MemoryExtractor panic", zap.Any("error", r))
		}
	}()

	if err := e.extractAndStore(ctx, agentContext, answer); err != nil {
		logger.Error("[记忆提取-Extract] 提取记忆失败",
			zap.String("query", agentContext.Query),
			zap.Error(err),
		)
		return err
	}

	logger.Debug("[记忆提取-Extract] 提取记忆成功",
		zap.String("query", agentContext.Query),
	)
	return nil
}

// extractAndStore 执行提取和存储。
func (e *memoryExtractor) extractAndStore(ctx context.Context, agentContext contracts.AgentContext, answer string) error {
	logger.Debug("[记忆提取-extractAndStore] 开始提取和存储",
		zap.String("user_id", string(agentContext.UserID)),
		zap.String("chat_model_id", agentContext.ChatModelID),
	)

	// 1. 调用LLM提取候选记忆
	logger.Debug("[记忆提取-extractAndStore] 步骤1: 调用LLM提取候选记忆")
	items, err := e.extractCandidates(ctx, agentContext.ChatModelID, agentContext.Query, answer)
	if err != nil {
		logger.Error("[记忆提取-extractAndStore] LLM提取候选记忆失败", zap.Error(err))
		return fmt.Errorf("extract candidates: %w", err)
	}

	if len(items) == 0 {
		logger.Debug("[记忆提取-extractAndStore] 未提取到候选记忆",
			zap.String("query", agentContext.Query),
		)
		return nil
	}

	logger.Debug("[记忆提取-extractAndStore] 提取到候选记忆",
		zap.String("query", agentContext.Query),
		zap.Int("count", len(items)),
	)

	// 2. 调用MemoryManager处理
	logger.Debug("[记忆提取-extractAndStore] 步骤2: 调用MemoryManager处理")
	userID := string(agentContext.UserID)
	if err := e.memoryManager.Process(ctx, userID, items); err != nil {
		logger.Error("[记忆提取-extractAndStore] MemoryManager处理失败", zap.Error(err))
		return fmt.Errorf("process memories: %w", err)
	}

	logger.Debug("[记忆提取-extractAndStore] MemoryManager处理成功")
	return nil
}

// extractCandidates 调用LLM提取候选记忆。
func (e *memoryExtractor) extractCandidates(ctx context.Context, chatModelID string, query, answer string) ([]contracts.MemoryItem, error) {
	logger.Debug("[记忆提取-extractCandidates] 开始提取候选记忆",
		zap.String("chat_model_id", chatModelID),
		zap.String("query", query),
	)

	// 构建Prompt
	tmpl, err := template.New("extractor").Parse(e.promptConfig.User)
	if err != nil {
		logger.Error("[记忆提取-extractCandidates] 解析模板失败", zap.Error(err))
		return nil, fmt.Errorf("parse template: %w", err)
	}

	data := struct {
		Query  string
		Answer string
	}{
		Query:  query,
		Answer: answer,
	}

	var userPrompt bytes.Buffer
	if err := tmpl.Execute(&userPrompt, data); err != nil {
		logger.Error("[记忆提取-extractCandidates] 执行模板失败", zap.Error(err))
		return nil, fmt.Errorf("execute template: %w", err)
	}

	logger.Debug("[记忆提取-extractCandidates] Prompt构建完成",
		zap.Int("prompt_length", userPrompt.Len()),
	)

	// 获取ChatModel
	logger.Debug("[记忆提取-extractCandidates] 获取ChatModel")
	model, err := e.modelFactory.GetChatModel(ctx, contracts.ID(chatModelID))
	if err != nil {
		logger.Error("[记忆提取-extractCandidates] 获取ChatModel失败", zap.Error(err))
		return nil, fmt.Errorf("get chat model: %w", err)
	}

	// 调用LLM
	logger.Debug("[记忆提取-extractCandidates] 调用LLM提取记忆")
	response, err := model.Generate(ctx, contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: e.promptConfig.System},
			{Role: "user", Content: userPrompt.String()},
		},
	})
	if err != nil {
		logger.Error("[记忆提取-extractCandidates] LLM调用失败", zap.Error(err))
		return nil, fmt.Errorf("chat with model: %w", err)
	}

	logger.Debug("[记忆提取-extractCandidates] LLM调用成功",
		zap.Int("response_length", len(response.Content)),
		zap.String("response_preview", truncateString(response.Content, 500)),
	)

	// 解析响应
	items, err := parseMemoryItems(response.Content)
	if err != nil {
		logger.Error("[记忆提取-extractCandidates] 解析响应失败",
			zap.Error(err),
			zap.String("response", response.Content),
		)
		return nil, fmt.Errorf("parse memory items: %w", err)
	}

	logger.Debug("[记忆提取-extractCandidates] 提取候选记忆完成",
		zap.Int("items_count", len(items)),
	)

	return items, nil
}

// parseMemoryItems 解析LLM返回的记忆列表。
func parseMemoryItems(response string) ([]contracts.MemoryItem, error) {
	// 提取JSON数组
	jsonStr := extractJSONArray(response)
	if jsonStr == "" {
		// 尝试提取单个JSON对象
		singleJSON := extractJSONFromText(response)
		if singleJSON == "" {
			return nil, fmt.Errorf("no JSON found in response")
		}
		// 包装为数组
		jsonStr = "[" + singleJSON + "]"
	}

	var items []contracts.MemoryItem
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return nil, fmt.Errorf("unmarshal memory items: %w", err)
	}

	// 验证和过滤
	var valid []contracts.MemoryItem
	validTypes := map[string]bool{
		"preference": true,
		"project":    true,
		"decision":   true,
		"goal":       true,
		"fact":       true,
		"progress":   true,
	}

	for _, item := range items {
		// 验证类型
		if !validTypes[string(item.Type)] {
			logger.Warn("无效的记忆类型，跳过", zap.String("type", string(item.Type)))
			continue
		}

		// 验证内容
		if item.Content == "" {
			logger.Warn("记忆内容为空，跳过")
			continue
		}

		// 验证重要性范围
		if item.Importance < 0 || item.Importance > 1 {
			item.Importance = 0.5 // 默认中等重要性
		}

		// 验证作用域
		if item.Scope != contracts.MemoryScopeUser && item.Scope != contracts.MemoryScopeKB {
			item.Scope = contracts.MemoryScopeUser // 默认用户级
		}
		// knowledge_base 作用域必须有 scope_id，否则降级为 user
		if item.Scope == contracts.MemoryScopeKB && item.ScopeID == nil {
			item.Scope = contracts.MemoryScopeUser
		}

		valid = append(valid, item)
	}

	return valid, nil
}

// extractJSONArray 从文本中提取JSON数组。
func extractJSONArray(text string) string {
	// 找到第一个 [
	start := -1
	for i, ch := range text {
		if ch == '[' {
			start = i
			break
		}
	}

	if start == -1 {
		return ""
	}

	// 找到对应的 ]
	depth := 0
	for i := start; i < len(text); i++ {
		if text[i] == '[' {
			depth++
		} else if text[i] == ']' {
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}

	return ""
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}
