package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// MemoryManagerConfig MemoryManager 配置。
type MemoryManagerConfig struct {
	SimilarityThreshold float64 // 向量相似度阈值，超过此值触发LLM判断
	VectorTopK          int     // 相似记忆检索数量
}

// DefaultMemoryManagerConfig 返回默认配置。
func DefaultMemoryManagerConfig() *MemoryManagerConfig {
	return &MemoryManagerConfig{
		SimilarityThreshold: 0.7,
		VectorTopK:          5,
	}
}

// DedupPromptConfig 去重Prompt配置。
type DedupPromptConfig struct {
	System string `yaml:"system"`
	User   string `yaml:"user"`
}

// memoryManager 实现 contracts.MemoryManager 接口。
type memoryManager struct {
	memoryRepo   repository.MemoryRepository
	embeddingSvc contracts.EmbeddingService
	modelFactory contracts.ModelFactory
	promptConfig DedupPromptConfig
	config       *MemoryManagerConfig
}

// NewMemoryManager 创建 MemoryManager 实例。
func NewMemoryManager(
	memoryRepo repository.MemoryRepository,
	embeddingSvc contracts.EmbeddingService,
	modelFactory contracts.ModelFactory,
	promptConfigPath string,
) (contracts.MemoryManager, error) {
	// 加载Prompt配置
	config, err := loadDedupPromptConfig(promptConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load dedup prompt config: %w", err)
	}

	return &memoryManager{
		memoryRepo:   memoryRepo,
		embeddingSvc: embeddingSvc,
		modelFactory: modelFactory,
		promptConfig: *config,
		config:       DefaultMemoryManagerConfig(),
	}, nil
}

// loadDedupPromptConfig 加载去重Prompt配置。
func loadDedupPromptConfig(path string) (*DedupPromptConfig, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}

	var config DedupPromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal dedup prompt config: %w", err)
	}

	return &config, nil
}

// readFile 读取文件内容。
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return data, nil
}

// Process 处理候选记忆列表，自动去重、合并后存储。
func (m *memoryManager) Process(ctx context.Context, userID string, items []contracts.MemoryItem) error {
	for _, item := range items {
		if err := m.processOne(ctx, userID, item); err != nil {
			logger.Error("处理记忆失败",
				zap.String("content", item.Content),
				zap.Error(err),
			)
			// 单条失败不影响其他记忆
			continue
		}
	}
	return nil
}

// processOne 处理单条记忆。
func (m *memoryManager) processOne(ctx context.Context, userID string, item contracts.MemoryItem) error {
	// 1. 计算内容哈希
	contentHash := computeContentHash(item.Content)

	// 2. 精确去重：通过content_hash查找
	var scopeID *string
	if item.ScopeID != nil {
		id := string(*item.ScopeID)
		scopeID = &id
	}

	existing, err := m.memoryRepo.FindByContentHash(ctx, userID, contentHash, string(item.Scope), scopeID)
	if err != nil {
		return fmt.Errorf("find by content hash: %w", err)
	}

	// 如果已存在完全相同的内容，更新重要性
	if existing != nil {
		return m.updateImportance(ctx, existing, item.Importance)
	}

	// 3. 向量检索相似记忆
	similar, err := m.findSimilarMemories(ctx, userID, item)
	if err != nil {
		logger.Warn("检索相似记忆失败，降级为直接创建", zap.Error(err))
		// 降级：直接创建
		return m.createMemory(ctx, userID, item)
	}

	// 4. 如果没有相似记忆，直接创建
	if len(similar) == 0 {
		return m.createMemory(ctx, userID, item)
	}

	// 5. LLM判断去重方式
	decision, err := m.llmJudgeDedup(ctx, item, similar)
	if err != nil {
		logger.Warn("LLM去重判断失败，降级为直接创建", zap.Error(err))
		// 降级：直接创建
		return m.createMemory(ctx, userID, item)
	}

	// 6. 根据判断结果执行
	return m.executeDecision(ctx, userID, item, decision, similar)
}

// findSimilarMemories 查找相似记忆。
func (m *memoryManager) findSimilarMemories(
	ctx context.Context,
	userID string,
	item contracts.MemoryItem,
) ([]contracts.MemoryQueryResult, error) {
	// 向量化新记忆内容
	queryVector, err := m.embeddingSvc.Embed(ctx, item.Content)
	if err != nil {
		return nil, fmt.Errorf("embed content: %w", err)
	}

	// 检索相似记忆
	var kbID contracts.ID
	if item.ScopeID != nil {
		kbID = *item.ScopeID
	}
	query := contracts.MemoryQuery{
		UserID:          contracts.ID(userID),
		KnowledgeBaseID: kbID,
		Query:           item.Content,
		TopK:            m.config.VectorTopK,
	}

	// 使用MemoryRetriever检索
	retriever := NewMemoryRetriever(m.memoryRepo, m.embeddingSvc)
	results, err := retriever.Retrieve(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("retrieve similar memories: %w", err)
	}

	// 过滤：只保留相似度超过阈值的
	var filtered []contracts.MemoryQueryResult
	for _, r := range results {
		if r.Similarity >= m.config.SimilarityThreshold {
			filtered = append(filtered, r)
		}
	}

	_ = queryVector // 用于日志或调试
	return filtered, nil
}

// llmJudgeDedup 使用LLM判断去重方式。
func (m *memoryManager) llmJudgeDedup(
	ctx context.Context,
	newItem contracts.MemoryItem,
	similar []contracts.MemoryQueryResult,
) (*DedupDecision, error) {
	// 构建Prompt数据
	data := struct {
		NewMemory        contracts.MemoryItem
		ExistingMemories []contracts.MemoryQueryResult
	}{
		NewMemory:        newItem,
		ExistingMemories: similar,
	}

	// 渲染Prompt
	tmpl, err := template.New("dedup").Parse(m.promptConfig.User)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var userPrompt bytes.Buffer
	if err := tmpl.Execute(&userPrompt, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	// 获取ChatModel
	model, err := m.modelFactory.GetChatModel(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("get chat model: %w", err)
	}

	// 调用LLM
	response, err := model.Generate(ctx, contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: m.promptConfig.System},
			{Role: "user", Content: userPrompt.String()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("chat with model: %w", err)
	}

	// 解析响应
	decision, err := parseDedupDecision(response.Content)
	if err != nil {
		return nil, fmt.Errorf("parse decision: %w", err)
	}

	return decision, nil
}

// executeDecision 执行去重判断结果。
func (m *memoryManager) executeDecision(
	ctx context.Context,
	userID string,
	item contracts.MemoryItem,
	decision *DedupDecision,
	similar []contracts.MemoryQueryResult,
) error {
	switch decision.Action {
	case "skip":
		logger.Info("跳过重复记忆",
			zap.String("content", item.Content),
			zap.String("reason", decision.Reason),
		)
		return nil

	case "update":
		// 更新已有记忆
		targetID := decision.TargetID
		if targetID == "" && len(similar) > 0 {
			targetID = string(similar[0].MemoryID)
		}
		return m.updateMemory(ctx, userID, targetID, item)

	case "merge":
		// 合并记忆
		targetID := decision.TargetID
		if targetID == "" && len(similar) > 0 {
			targetID = string(similar[0].MemoryID)
		}
		mergedContent := decision.MergedContent
		if mergedContent == "" {
			mergedContent = item.Content
		}
		return m.mergeMemory(ctx, userID, targetID, mergedContent, item)

	case "create":
		// 创建新记忆
		return m.createMemory(ctx, userID, item)

	default:
		// 未知操作，默认创建
		logger.Warn("未知的去重操作，默认创建", zap.String("action", decision.Action))
		return m.createMemory(ctx, userID, item)
	}
}

// createMemory 创建新记忆。
func (m *memoryManager) createMemory(ctx context.Context, userID string, item contracts.MemoryItem) error {
	// 向量化
	vector, err := m.embeddingSvc.Embed(ctx, item.Content)
	if err != nil {
		return fmt.Errorf("embed content: %w", err)
	}

	// 构建实体
	memory := &entity.Memory{
		UserID:           userID,
		MemoryType:       string(item.Type),
		ScopeType:        string(item.Scope),
		ScopeID:          (*string)(item.ScopeID),
		Content:          item.Content,
		Summary:          item.Content, // 简化处理，summary与content相同
		Importance:       item.Importance,
		ContentHash:      computeContentHash(item.Content),
		Embedding:        float64SliceToBytes(vector),
		EmbeddingDim:     len(vector),
		EmbeddingModelID: "", // 需要从embeddingSvc获取
		Status:           "active",
	}

	if err := m.memoryRepo.Create(ctx, memory); err != nil {
		return fmt.Errorf("create memory: %w", err)
	}

	logger.Info("创建新记忆",
		zap.String("content", item.Content),
		zap.String("type", string(item.Type)),
	)
	return nil
}

// updateMemory 更新已有记忆。
func (m *memoryManager) updateMemory(ctx context.Context, userID, memoryID string, item contracts.MemoryItem) error {
	// 查找现有记忆
	existing, err := m.memoryRepo.FindByID(ctx, memoryID, userID)
	if err != nil {
		return fmt.Errorf("find memory: %w", err)
	}

	// 更新内容
	existing.Content = item.Content
	existing.Summary = item.Content
	existing.Importance = item.Importance
	existing.MemoryType = string(item.Type)

	// 重新向量化
	vector, err := m.embeddingSvc.Embed(ctx, item.Content)
	if err != nil {
		return fmt.Errorf("embed content: %w", err)
	}
	existing.Embedding = float64SliceToBytes(vector)
	existing.EmbeddingDim = len(vector)
	existing.ContentHash = computeContentHash(item.Content)

	if err := m.memoryRepo.Update(ctx, existing); err != nil {
		return fmt.Errorf("update memory: %w", err)
	}

	logger.Info("更新记忆",
		zap.String("memory_id", memoryID),
		zap.String("content", item.Content),
	)
	return nil
}

// mergeMemory 合并记忆。
func (m *memoryManager) mergeMemory(ctx context.Context, userID, targetID, mergedContent string, item contracts.MemoryItem) error {
	// 查找目标记忆
	existing, err := m.memoryRepo.FindByID(ctx, targetID, userID)
	if err != nil {
		return fmt.Errorf("find memory: %w", err)
	}

	// 合并内容
	existing.Content = mergedContent
	existing.Summary = mergedContent
	// 重要性取最高
	if item.Importance > existing.Importance {
		existing.Importance = item.Importance
	}

	// 重新向量化
	vector, err := m.embeddingSvc.Embed(ctx, mergedContent)
	if err != nil {
		return fmt.Errorf("embed content: %w", err)
	}
	existing.Embedding = float64SliceToBytes(vector)
	existing.EmbeddingDim = len(vector)
	existing.ContentHash = computeContentHash(mergedContent)

	if err := m.memoryRepo.Update(ctx, existing); err != nil {
		return fmt.Errorf("update memory: %w", err)
	}

	logger.Info("合并记忆",
		zap.String("target_id", targetID),
		zap.String("merged_content", mergedContent),
	)
	return nil
}

// updateImportance 更新记忆重要性（取最高）。
func (m *memoryManager) updateImportance(ctx context.Context, memory *entity.Memory, newImportance float64) error {
	if newImportance > memory.Importance {
		memory.Importance = newImportance
		if err := m.memoryRepo.Update(ctx, memory); err != nil {
			return fmt.Errorf("update importance: %w", err)
		}
	}
	return nil
}

// computeContentHash 计算内容哈希。
func computeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// DedupDecision 去重判断结果。
type DedupDecision struct {
	Action        string `json:"action"`         // create/update/merge/skip
	Reason        string `json:"reason"`         // 判断理由
	TargetID      string `json:"target_id"`      // 目标记忆ID（update/merge时）
	MergedContent string `json:"merged_content"` // 合并后内容（merge时）
}

// parseDedupDecision 解析LLM返回的去重判断。
func parseDedupDecision(response string) (*DedupDecision, error) {
	// 提取JSON
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var decision DedupDecision
	if err := json.Unmarshal([]byte(jsonStr), &decision); err != nil {
		return nil, fmt.Errorf("unmarshal decision: %w", err)
	}

	// 验证action
	validActions := map[string]bool{"create": true, "update": true, "merge": true, "skip": true}
	if !validActions[decision.Action] {
		return nil, fmt.Errorf("invalid action: %s", decision.Action)
	}

	return &decision, nil
}

// extractJSON 从文本中提取JSON（复用planner_service.go中的函数）。
func extractJSONFromText(text string) string {
	// 尝试找到JSON块
	start := 0
	for i, ch := range text {
		if ch == '{' {
			start = i
			break
		}
	}

	if start == 0 {
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
