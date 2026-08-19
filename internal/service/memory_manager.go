package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
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
	memoryRepo     repository.MemoryRepository
	embeddingSvc   contracts.EmbeddingService
	modelFactory   contracts.ModelFactory
	systemTemplate *template.Template // 预编译的 system Prompt 模板
	userTemplate   *template.Template // 预编译的 user Prompt 模板
	config         *MemoryManagerConfig
}

// NewMemoryManager 创建 MemoryManager 实例。
// 在启动时预编译 Prompt 模板，启用 missingkey=error 以确保字段错误在启动时暴露。
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

	// 预编译 system Prompt 模板，启用 missingkey=error
	systemTemplate, err := template.New("memory-dedup-system").
		Option("missingkey=error").
		Parse(config.System)
	if err != nil {
		return nil, fmt.Errorf("parse memory dedup system prompt: %w", err)
	}

	// 预编译 user Prompt 模板，启用 missingkey=error
	userTemplate, err := template.New("memory-dedup-user").
		Option("missingkey=error").
		Parse(config.User)
	if err != nil {
		return nil, fmt.Errorf("parse memory dedup user prompt: %w", err)
	}

	return &memoryManager{
		memoryRepo:     memoryRepo,
		embeddingSvc:   embeddingSvc,
		modelFactory:   modelFactory,
		systemTemplate: systemTemplate,
		userTemplate:   userTemplate,
		config:         DefaultMemoryManagerConfig(),
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
// chatModelID 用于 LLM 去重判断，必须传入有效的模型 ID。
func (m *memoryManager) Process(ctx context.Context, userID string, items []contracts.MemoryItem, chatModelID string) error {
	logger.Debug("[记忆处理-Process] 开始处理候选记忆",
		zap.String("user_id", userID),
		zap.Int("items_count", len(items)),
		zap.String("chat_model_id", chatModelID),
	)

	// 验证 chatModelID 必须有效
	if strings.TrimSpace(chatModelID) == "" {
		return fmt.Errorf("memory dedup chat model ID is required")
	}

	// 收集所有错误
	var errs []error
	for i, item := range items {
		logger.Debug("[记忆处理-Process] 处理第"+fmt.Sprintf("%d", i+1)+"条记忆",
			zap.String("type", string(item.Type)),
			zap.String("content", item.Content),
			zap.Float64("importance", item.Importance),
		)
		if err := m.processOne(ctx, userID, item, chatModelID); err != nil {
			logger.Error("[记忆处理-Process] 处理记忆失败",
				zap.String("content", item.Content),
				zap.Error(err),
			)
			errs = append(errs, fmt.Errorf("process memory item %d: %w", i, err))
			// 继续处理其他记忆
			continue
		}
		logger.Debug("[记忆处理-Process] 处理记忆成功",
			zap.String("content", item.Content),
		)
	}

	if len(errs) > 0 {
		logger.Error("[记忆处理-Process] 部分记忆处理失败",
			zap.Int("failed_count", len(errs)),
			zap.Int("total_count", len(items)),
		)
		return errors.Join(errs...)
	}

	logger.Debug("[记忆处理-Process] 所有记忆处理完成")
	return nil
}

// processOne 处理单条记忆。
func (m *memoryManager) processOne(ctx context.Context, userID string, item contracts.MemoryItem, modelID string) error {
	logger.Debug("[记忆处理-processOne] 开始处理单条记忆",
		zap.String("user_id", userID),
		zap.String("type", string(item.Type)),
		zap.String("content", item.Content),
		zap.String("model_id", modelID),
	)

	// 1. 计算内容哈希
	contentHash := computeContentHash(item.Content)
	logger.Debug("[记忆处理-processOne] 步骤1: 计算内容哈希",
		zap.String("content_hash", contentHash),
	)

	// 2. 精确去重：通过content_hash查找
	logger.Debug("[记忆处理-processOne] 步骤2: 精确去重检查")
	var scopeID *string
	if item.ScopeID != nil {
		id := string(*item.ScopeID)
		scopeID = &id
	}

	existing, err := m.memoryRepo.FindByContentHash(ctx, userID, contentHash, string(item.Scope), scopeID)
	if err != nil {
		logger.Error("[记忆处理-processOne] 精确去重检查失败", zap.Error(err))
		return fmt.Errorf("memory semantic dedup unavailable: find by content hash: %w", err)
	}

	// 如果已存在完全相同的内容，更新重要性
	if existing != nil {
		logger.Debug("[记忆处理-processOne] 发现重复记忆，更新重要性",
			zap.String("existing_id", existing.ID),
		)
		return m.updateImportance(ctx, existing, item.Importance)
	}
	logger.Debug("[记忆处理-processOne] 未发现重复记忆")

	// 3. 向量检索相似记忆
	logger.Debug("[记忆处理-processOne] 步骤3: 向量检索相似记忆")
	similar, err := m.findSimilarMemories(ctx, userID, item)
	if err != nil {
		logger.Error("[记忆处理-processOne] 向量检索失败", zap.Error(err))
		// 返回可重试错误，不直接创建
		return fmt.Errorf("memory semantic dedup unavailable: vector search: %w", err)
	}

	// 4. 如果没有相似记忆，直接创建
	if len(similar) == 0 {
		logger.Debug("[记忆处理-processOne] 未找到相似记忆，直接创建")
		return m.createMemory(ctx, userID, item)
	}

	logger.Debug("[记忆处理-processOne] 找到相似记忆",
		zap.Int("similar_count", len(similar)),
	)

	// 5. LLM判断去重方式
	logger.Debug("[记忆处理-processOne] 步骤5: LLM判断去重方式")
	decision, err := m.llmJudgeDedup(ctx, item, similar, modelID)
	if err != nil {
		logger.Error("[记忆处理-processOne] LLM去重判断失败", zap.Error(err))
		// 返回可重试错误，不直接创建
		return fmt.Errorf("memory semantic dedup unavailable: LLM judge: %w", err)
	}

	logger.Debug("[记忆处理-processOne] LLM去重判断完成",
		zap.String("action", decision.Action),
		zap.String("reason", decision.Reason),
	)

	// 6. 根据判断结果执行
	logger.Debug("[记忆处理-processOne] 步骤6: 执行去重决策")
	return m.executeDecision(ctx, userID, item, decision, similar)
}

// findSimilarMemories 查找相似记忆（用于去重判断）。
// 直接使用向量检索，返回原始 cosine similarity，不经过 RRF 融合。
func (m *memoryManager) findSimilarMemories(
	ctx context.Context,
	userID string,
	item contracts.MemoryItem,
) ([]contracts.MemoryQueryResult, error) {
	logger.Debug("[记忆处理-findSimilarMemories] 开始查找相似记忆",
		zap.String("user_id", userID),
		zap.String("content", item.Content),
	)

	// 向量化新记忆内容，同时获取使用的模型 ID
	logger.Debug("[记忆处理-findSimilarMemories] 步骤1: 向量化查询内容")
	queryVector, embeddingModelID, err := m.embeddingSvc.EmbedWithModelID(ctx, userID, item.Content)
	if err != nil {
		logger.Error("[记忆处理-findSimilarMemories] 向量化查询内容失败", zap.Error(err))
		return nil, fmt.Errorf("embed content: %w", err)
	}
	logger.Debug("[记忆处理-findSimilarMemories] 向量化查询内容成功",
		zap.Int("vector_dim", len(queryVector)),
		zap.String("embedding_model_id", embeddingModelID),
	)

	// 构建知识库 ID
	var kbID *string
	if item.ScopeID != nil {
		id := string(*item.ScopeID)
		kbID = &id
	}

	// 直接使用向量检索，返回原始 cosine similarity
	// 传递 EmbeddingModelID 以确保只检索相同模型生成的向量
	searchReq := repository.VectorSearchRequest{
		UserID:           userID,
		KnowledgeBaseID:  kbID,
		QueryVector:      formatPgVectorFromFloat64(queryVector),
		EmbeddingDim:     len(queryVector),
		EmbeddingModelID: embeddingModelID, // 过滤相同模型生成的向量
		TopK:             m.config.VectorTopK,
		MinImportance:    0.0, // 去重场景不限制重要性
	}

	logger.Debug("[记忆处理-findSimilarMemories] 步骤2: 向量检索相似记忆",
		zap.String("embedding_model_id", embeddingModelID),
	)
	results, err := m.memoryRepo.SearchByVector(ctx, searchReq)
	if err != nil {
		logger.Error("[记忆处理-findSimilarMemories] 向量检索失败", zap.Error(err))
		return nil, fmt.Errorf("vector search: %w", err)
	}
	logger.Debug("[记忆处理-findSimilarMemories] 向量检索成功",
		zap.Int("results_count", len(results)),
	)

	// 转换为 MemoryQueryResult，保留原始 cosine similarity
	var queryResults []contracts.MemoryQueryResult
	for _, r := range results {
		var scopeID *contracts.ID
		if r.Memory.ScopeID != nil {
			id := contracts.ID(*r.Memory.ScopeID)
			scopeID = &id
		}
		queryResults = append(queryResults, contracts.MemoryQueryResult{
			MemoryID:   contracts.ID(r.Memory.ID),
			MemoryType: contracts.MemoryType(r.Memory.MemoryType),
			ScopeType:  contracts.MemoryScope(r.Memory.ScopeType),
			ScopeID:    scopeID,
			Content:    r.Memory.Content,
			Similarity: r.Similarity, // 保留原始 cosine similarity
			Importance: r.Memory.Importance,
			UpdatedAt:  r.Memory.UpdatedAt,
		})
	}

	// 过滤：只保留相似度超过阈值的
	var filtered []contracts.MemoryQueryResult
	for _, r := range queryResults {
		if r.Similarity >= m.config.SimilarityThreshold {
			filtered = append(filtered, r)
		}
	}

	logger.Debug("[记忆处理-findSimilarMemories] 过滤后相似记忆数量",
		zap.Int("filtered_count", len(filtered)),
		zap.Float64("threshold", m.config.SimilarityThreshold),
	)

	return filtered, nil
}

// llmJudgeDedup 使用LLM判断去重方式。
func (m *memoryManager) llmJudgeDedup(
	ctx context.Context,
	newItem contracts.MemoryItem,
	similar []contracts.MemoryQueryResult,
	modelID string,
) (*DedupDecision, error) {
	logger.Debug("[记忆处理-llmJudgeDedup] 开始LLM去重判断",
		zap.String("model_id", modelID),
		zap.Int("similar_count", len(similar)),
	)

	// 构建Prompt数据
	data := struct {
		NewMemory        contracts.MemoryItem
		ExistingMemories []contracts.MemoryQueryResult
	}{
		NewMemory:        newItem,
		ExistingMemories: similar,
	}

	// 使用预编译的 system Prompt 模板
	var systemPrompt bytes.Buffer
	if err := m.systemTemplate.Execute(&systemPrompt, data); err != nil {
		return nil, fmt.Errorf("render memory dedup system prompt: %w", err)
	}

	// 使用预编译的 user Prompt 模板
	var userPrompt bytes.Buffer
	if err := m.userTemplate.Execute(&userPrompt, data); err != nil {
		return nil, fmt.Errorf("render memory dedup user prompt: %w", err)
	}

	logger.Debug("[记忆处理-llmJudgeDedup] Prompt渲染完成",
		zap.Int("system_prompt_length", systemPrompt.Len()),
		zap.Int("user_prompt_length", userPrompt.Len()),
	)

	// 获取ChatModel
	model, err := m.modelFactory.GetChatModel(ctx, contracts.ID(modelID))
	if err != nil {
		return nil, fmt.Errorf("get chat model: %w", err)
	}

	// 调用LLM
	response, err := model.Generate(ctx, contracts.ChatRequest{
		Messages: []contracts.ChatMessage{
			{Role: "system", Content: systemPrompt.String()},
			{Role: "user", Content: userPrompt.String()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("chat with model: %w", err)
	}

	logger.Debug("[记忆处理-llmJudgeDedup] LLM调用完成",
		zap.Int("response_length", len(response.Content)),
	)

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
	// 首先校验决策
	if err := validateDedupDecision(decision); err != nil {
		logger.Error("[记忆处理-executeDecision] 决策校验失败", zap.Error(err))
		return fmt.Errorf("invalid dedup decision: %w", err)
	}

	switch decision.Action {
	case "skip":
		// skip 需要验证目标存在于候选集中且作用域一致
		candidate, ok := findCandidate(decision.TargetID, similar)
		if !ok {
			return fmt.Errorf("skip target is not in similar memory candidates")
		}
		// 校验作用域一致性
		if err := validateScopeConsistency(candidate, item); err != nil {
			return fmt.Errorf("scope mismatch: %w", err)
		}
		logger.Info("跳过重复记忆",
			zap.String("target_id", decision.TargetID),
			zap.String("content", item.Content),
			zap.String("reason", decision.Reason),
		)
		return nil

	case "update":
		// 查找目标候选记忆
		candidate, ok := findCandidate(decision.TargetID, similar)
		if !ok {
			return fmt.Errorf("dedup target is not in similar memory candidates")
		}
		// 校验作用域一致性
		if err := validateScopeConsistency(candidate, item); err != nil {
			return fmt.Errorf("scope mismatch: %w", err)
		}
		return m.updateMemory(ctx, userID, decision.TargetID, item)

	case "merge":
		// 查找目标候选记忆
		candidate, ok := findCandidate(decision.TargetID, similar)
		if !ok {
			return fmt.Errorf("dedup target is not in similar memory candidates")
		}
		// 校验作用域一致性
		if err := validateScopeConsistency(candidate, item); err != nil {
			return fmt.Errorf("scope mismatch: %w", err)
		}
		return m.mergeMemory(ctx, userID, decision.TargetID, decision.MergedContent, item)

	case "supersede":
		// 新决策替代旧决策：将旧记忆置为 inactive，创建新记忆
		candidate, ok := findCandidate(decision.TargetID, similar)
		if !ok {
			return fmt.Errorf("dedup target is not in similar memory candidates")
		}
		// 校验作用域一致性
		if err := validateScopeConsistency(candidate, item); err != nil {
			return fmt.Errorf("scope mismatch: %w", err)
		}
		logger.Info("新决策替代旧记忆",
			zap.String("old_memory_id", decision.TargetID),
			zap.String("reason", decision.Reason),
		)
		return m.supersedeMemory(ctx, userID, decision.TargetID, item)

	case "invalidate":
		// 明确撤销：将旧记忆置为 inactive
		// 验证目标存在于候选集中
		candidate, ok := findCandidate(decision.TargetID, similar)
		if !ok {
			return fmt.Errorf("dedup target is not in similar memory candidates")
		}
		// 校验作用域一致性
		if err := validateScopeConsistency(candidate, item); err != nil {
			return fmt.Errorf("scope mismatch: %w", err)
		}
		logger.Info("撤销记忆",
			zap.String("memory_id", decision.TargetID),
			zap.String("reason", decision.Reason),
		)
		return m.invalidateMemory(ctx, userID, decision.TargetID)

	case "keep_both":
		// 无法确定冲突关系，保留两条
		logger.Info("保留两条记忆",
			zap.String("new_content", item.Content),
			zap.String("reason", decision.Reason),
		)
		return m.createMemory(ctx, userID, item)

	case "create":
		// 创建新记忆
		return m.createMemory(ctx, userID, item)

	default:
		// 未知操作，返回错误
		return fmt.Errorf("unsupported dedup action: %s", decision.Action)
	}
}

// validateDedupDecision 严格校验 LLM 返回的去重决策。
func validateDedupDecision(d *DedupDecision) error {
	d.Action = strings.ToLower(strings.TrimSpace(d.Action))
	d.TargetID = strings.TrimSpace(d.TargetID)
	d.MergedContent = strings.TrimSpace(d.MergedContent)
	d.Reason = strings.TrimSpace(d.Reason)

	// 限制 reason 长度
	if len(d.Reason) > 500 {
		d.Reason = d.Reason[:500]
	}

	// 限制 merged_content 长度
	if len(d.MergedContent) > 2000 {
		d.MergedContent = d.MergedContent[:2000]
	}

	switch d.Action {
	case "create":
		// create 不应携带 target_id
		if d.TargetID != "" {
			return fmt.Errorf("create decision must not contain target_id")
		}
	case "skip":
		// skip 必须有 target_id
		if d.TargetID == "" {
			return fmt.Errorf("skip decision requires target_id")
		}
	case "update":
		// update 必须有 target_id
		if d.TargetID == "" {
			return fmt.Errorf("update decision requires target_id")
		}
	case "merge":
		// merge 必须有 target_id 和非空 merged_content
		if d.TargetID == "" {
			return fmt.Errorf("merge decision requires target_id")
		}
		if d.MergedContent == "" {
			return fmt.Errorf("merge decision requires merged_content")
		}
	case "supersede":
		// supersede 表示新决策替代旧决策，必须有 target_id
		if d.TargetID == "" {
			return fmt.Errorf("supersede decision requires target_id")
		}
	case "invalidate":
		// invalidate 表示明确撤销，必须有 target_id
		if d.TargetID == "" {
			return fmt.Errorf("invalidate decision requires target_id")
		}
	case "keep_both":
		// keep_both 表示保留两条，不应携带 target_id
		if d.TargetID != "" {
			return fmt.Errorf("keep_both decision must not contain target_id")
		}
	default:
		return fmt.Errorf("unsupported action %q", d.Action)
	}

	return nil
}

// findCandidate 查找目标候选记忆。
func findCandidate(
	targetID string,
	similar []contracts.MemoryQueryResult,
) (contracts.MemoryQueryResult, bool) {
	for _, candidate := range similar {
		if string(candidate.MemoryID) == targetID {
			return candidate, true
		}
	}
	return contracts.MemoryQueryResult{}, false
}

// validateScopeConsistency 校验作用域一致性。
func validateScopeConsistency(candidate contracts.MemoryQueryResult, item contracts.MemoryItem) error {
	// 校验作用域类型
	if candidate.ScopeType != item.Scope {
		return fmt.Errorf("scope type mismatch: candidate=%s, item=%s", candidate.ScopeType, item.Scope)
	}

	// 校验作用域 ID
	if item.Scope == contracts.MemoryScopeKB {
		// 知识库级必须完全匹配
		if candidate.ScopeID == nil && item.ScopeID == nil {
			return nil
		}
		if candidate.ScopeID == nil || item.ScopeID == nil {
			return fmt.Errorf("scope_id mismatch: one is nil")
		}
		if string(*candidate.ScopeID) != string(*item.ScopeID) {
			return fmt.Errorf("scope_id mismatch: candidate=%s, item=%s", *candidate.ScopeID, *item.ScopeID)
		}
	}

	return nil
}

// prepareMemory 准备新记忆实体（向量化和构建实体，但不写入数据库）。
// 此方法在事务外调用，避免长时间占用数据库连接。
func (m *memoryManager) prepareMemory(
	ctx context.Context,
	userID string,
	item contracts.MemoryItem,
) (*entity.Memory, error) {
	logger.Debug("[记忆处理-prepareMemory] 开始准备记忆",
		zap.String("user_id", userID),
		zap.String("type", string(item.Type)),
		zap.String("content", item.Content),
	)

	// 向量化并获取使用的模型ID
	vector, modelID, err := m.embeddingSvc.EmbedWithModelID(ctx, userID, item.Content)
	if err != nil {
		logger.Error("[记忆处理-prepareMemory] 向量化失败", zap.Error(err))
		return nil, fmt.Errorf("embed content: %w", err)
	}
	logger.Debug("[记忆处理-prepareMemory] 向量化成功",
		zap.Int("vector_dim", len(vector)),
		zap.String("model_id", modelID),
	)

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
		Embedding:        formatPgVectorFromFloat64(vector), // 转换为 PostgreSQL 向量格式
		EmbeddingDim:     len(vector),
		EmbeddingModelID: modelID,
		Status:           "active",
	}

	return memory, nil
}

// createMemory 创建新记忆。
func (m *memoryManager) createMemory(ctx context.Context, userID string, item contracts.MemoryItem) error {
	logger.Debug("[记忆处理-createMemory] 开始创建新记忆",
		zap.String("user_id", userID),
		zap.String("type", string(item.Type)),
		zap.String("content", item.Content),
	)

	// 准备记忆实体
	memory, err := m.prepareMemory(ctx, userID, item)
	if err != nil {
		return err
	}

	// 存储到数据库
	logger.Debug("[记忆处理-createMemory] 步骤2: 存储到数据库")
	if err := m.memoryRepo.Create(ctx, memory); err != nil {
		logger.Error("[记忆处理-createMemory] 存储到数据库失败", zap.Error(err))
		return fmt.Errorf("create memory: %w", err)
	}

	logger.Debug("[记忆处理-createMemory] 创建新记忆成功",
		zap.String("memory_id", memory.ID),
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

	// 重新向量化并获取模型ID
	vector, modelID, err := m.embeddingSvc.EmbedWithModelID(ctx, userID, item.Content)
	if err != nil {
		return fmt.Errorf("embed content: %w", err)
	}
	existing.Embedding = formatPgVectorFromFloat64(vector) // 转换为 PostgreSQL 向量格式
	existing.EmbeddingDim = len(vector)
	existing.EmbeddingModelID = modelID
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

	// 重新向量化并获取模型ID
	vector, modelID, err := m.embeddingSvc.EmbedWithModelID(ctx, userID, mergedContent)
	if err != nil {
		return fmt.Errorf("embed content: %w", err)
	}
	existing.Embedding = formatPgVectorFromFloat64(vector) // 转换为 PostgreSQL 向量格式
	existing.EmbeddingDim = len(vector)
	existing.EmbeddingModelID = modelID
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

// supersedeMemory 新决策替代旧决策：将旧记忆置为 inactive，创建新记忆。
// 事务外生成 Embedding，事务内完成状态转换和新记忆创建，确保原子性。
func (m *memoryManager) supersedeMemory(ctx context.Context, userID, oldMemoryID string, item contracts.MemoryItem) error {
	logger.Debug("[记忆处理-supersedeMemory] 开始新决策替代旧记忆",
		zap.String("user_id", userID),
		zap.String("old_memory_id", oldMemoryID),
		zap.String("new_content", item.Content),
	)

	// 事务外：准备新记忆实体（生成 Embedding）
	newMemory, err := m.prepareMemory(ctx, userID, item)
	if err != nil {
		return fmt.Errorf("prepare replacement memory: %w", err)
	}

	// 事务内：原子替代
	if err := m.memoryRepo.Supersede(ctx, oldMemoryID, userID, newMemory); err != nil {
		return fmt.Errorf("supersede memory: %w", err)
	}

	logger.Info("新记忆已替代旧记忆",
		zap.String("old_memory_id", oldMemoryID),
		zap.String("new_memory_id", newMemory.ID),
	)
	return nil
}

// invalidateMemory 明确撤销：将 active 状态的记忆置为 inactive。
func (m *memoryManager) invalidateMemory(ctx context.Context, userID, memoryID string) error {
	logger.Debug("[记忆处理-invalidateMemory] 开始撤销记忆",
		zap.String("user_id", userID),
		zap.String("memory_id", memoryID),
	)

	if err := m.memoryRepo.InvalidateActive(ctx, memoryID, userID); err != nil {
		return fmt.Errorf("invalidate memory: %w", err)
	}

	logger.Info("记忆已置为 inactive",
		zap.String("memory_id", memoryID),
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
	Action        string `json:"action"`         // create/update/merge/skip/supersede/invalidate/keep_both
	Reason        string `json:"reason"`         // 判断理由
	TargetID      string `json:"target_id"`      // 目标记忆ID（update/merge/supersede/invalidate时）
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
	validActions := map[string]bool{
		"create":     true,
		"update":     true,
		"merge":      true,
		"skip":       true,
		"supersede":  true,
		"invalidate": true,
		"keep_both":  true,
	}
	if !validActions[decision.Action] {
		return nil, fmt.Errorf("invalid action: %s", decision.Action)
	}

	return &decision, nil
}

// extractJSON 从文本中提取JSON。
// 能够正确处理各种情况，包括直接返回的JSON。
func extractJSON(text string) string {
	// 尝试直接解析整个文本
	var obj interface{}
	if err := json.Unmarshal([]byte(text), &obj); err == nil {
		// 整个文本就是有效的 JSON
		jsonBytes, _ := json.Marshal(obj)
		return string(jsonBytes)
	}

	// 使用逐字符查找并尝试解析
	start := -1
	for i, ch := range text {
		if ch == '{' || ch == '[' {
			start = i
			break
		}
	}

	if start == -1 {
		return ""
	}

	// 找到对应的闭合括号
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(text); i++ {
		ch := text[i]

		if escape {
			escape = false
			continue
		}

		if ch == '\\' && inString {
			escape = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if ch == '{' || ch == '[' {
			depth++
		} else if ch == '}' || ch == ']' {
			depth--
			if depth == 0 {
				jsonStr := text[start : i+1]
				// 验证提取的 JSON 是否有效
				var obj interface{}
				if err := json.Unmarshal([]byte(jsonStr), &obj); err == nil {
					return jsonStr
				}
				// 如果无效，继续查找
			}
		}
	}

	return ""
}

// formatPgVectorFromFloat64 将 float64 切片转换为 PostgreSQL 向量格式字符串
func formatPgVectorFromFloat64(vec []float64) string {
	strs := make([]string, len(vec))
	for i, v := range vec {
		strs[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(strs, ",") + "]"
}
