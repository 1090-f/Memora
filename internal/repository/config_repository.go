package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"gorm.io/gorm"
)

var (
	// ErrSearchConfigNotFound 表示未找到指定搜索配置。
	ErrSearchConfigNotFound = errors.New("搜索配置不存在")
	// ErrSearchConfigConflict 表示搜索配置已存在。
	ErrSearchConfigConflict = errors.New("搜索配置已存在")
	// ErrAgentConfigNotFound 表示未找到指定 Agent 配置。
	ErrAgentConfigNotFound = errors.New("Agent 配置不存在")
	// ErrAgentConfigConflict 表示 Agent 配置已存在。
	ErrAgentConfigConflict = errors.New("Agent 配置已存在")
	// ErrModelConfigNotFound 表示未找到指定模型配置。
	ErrModelConfigNotFound = errors.New("模型配置不存在")
)

// searchConfigRepository 是 SearchConfigRepository 接口的 GORM 实现。
type searchConfigRepository struct{ db *gorm.DB }

// NewSearchConfigRepository 创建一个新的搜索配置仓储实例。
func NewSearchConfigRepository(db *gorm.DB) SearchConfigRepository {
	return &searchConfigRepository{db: db}
}

// Create 创建搜索配置。
func (r *searchConfigRepository) Create(ctx context.Context, cfg *entity.SearchConfig) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).Create(cfg).Error
	if err != nil {
		// 唯一约束冲突说明配置已存在，转换为业务错误以便上层识别。
		if isUniqueViolation(err) {
			return ErrSearchConfigConflict
		}
		return fmt.Errorf("创建搜索配置失败: %w", err)
	}
	return nil
}

// FindByKnowledgeBase 按知识库查询搜索配置。
func (r *searchConfigRepository) FindByKnowledgeBase(ctx context.Context, kbID string) (*entity.SearchConfig, error) {
	var cfg entity.SearchConfig
	err := dbFromContext(ctx, r.db).WithContext(ctx).Where("knowledge_base_id = ?", kbID).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSearchConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询搜索配置失败: %w", err)
	}
	return &cfg, nil
}

// Update 更新搜索配置。
func (r *searchConfigRepository) Update(ctx context.Context, cfg *entity.SearchConfig) (*entity.SearchConfig, error) {
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.SearchConfig{}).
		Where("id = ?", cfg.ID).
		Updates(map[string]any{
			"keyword_top_k":             cfg.KeywordTopK,
			"vector_top_k":              cfg.VectorTopK,
			"rrf_k":                     cfg.RRFK,
			"rrf_top_k":                 cfg.RRFTopK,
			"reranker_top_k":            cfg.RerankerTopK,
			"reranker_threshold":        cfg.RerankerThreshold,
			"minimum_effective_results": cfg.MinimumEffectiveRate,
			"min_vector_score":          cfg.MinVectorScore,
			"ambiguous_score":           cfg.AmbiguousScore,
			"reranker_model_id":         cfg.RerankerModelID,
			"updated_at":                time.Now().UTC(),
		})
	if result.Error != nil {
		return nil, fmt.Errorf("更新搜索配置失败: %w", result.Error)
	}
	// RowsAffected 为 0 表示 WHERE 未命中目标行，按未找到处理。
	if result.RowsAffected == 0 {
		return nil, ErrSearchConfigNotFound
	}
	return r.FindByKnowledgeBase(ctx, cfg.KnowledgeBaseID)
}

// agentConfigRepository 是 AgentConfigRepository 接口的 GORM 实现。
type agentConfigRepository struct{ db *gorm.DB }

// NewAgentConfigRepository 创建一个新的 Agent 配置仓储实例。
func NewAgentConfigRepository(db *gorm.DB) AgentConfigRepository {
	return &agentConfigRepository{db: db}
}

// Create 创建 Agent 配置默认行。
func (r *agentConfigRepository) Create(ctx context.Context, cfg *entity.AgentConfig) error {
	err := dbFromContext(ctx, r.db).WithContext(ctx).Create(cfg).Error
	if err != nil {
		// 唯一约束冲突说明默认行已存在，转换为业务错误。
		if isUniqueViolation(err) {
			return ErrAgentConfigConflict
		}
		return fmt.Errorf("创建 Agent 配置失败: %w", err)
	}
	return nil
}

// FindByKnowledgeBase 按知识库查询 Agent 配置。
func (r *agentConfigRepository) FindByKnowledgeBase(ctx context.Context, userID, kbID string) (*entity.AgentConfig, error) {
	var cfg entity.AgentConfig
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND knowledge_base_id = ?", userID, kbID).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询 Agent 配置失败: %w", err)
	}
	return &cfg, nil
}

// UpdateChatModel 更新知识库 Agent 配置的对话模型 ID。
// 问答与 Agent 运行时从 agent_configs.chat_model_id 读取模型，
// 知识库默认 Chat 模型变更时必须同步，否则实际问答仍用旧模型。
func (r *agentConfigRepository) UpdateChatModel(ctx context.Context, userID, kbID, chatModelID string) error {
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.AgentConfig{}).
		Where("user_id = ? AND knowledge_base_id = ?", userID, kbID).
		Update("chat_model_id", chatModelID)
	if result.Error != nil {
		return fmt.Errorf("更新 Agent 配置对话模型失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAgentConfigNotFound
	}
	return nil
}

// UpdateNetworkEnabled 更新知识库 Agent 配置的联网开关。
// 问答与 Agent 运行时从 agent_configs.network_enabled 读取开关，
// 知识库联网开关变更时必须同步，否则实际运行仍按旧开关拦截工具。
func (r *agentConfigRepository) UpdateNetworkEnabled(ctx context.Context, userID, kbID string, enabled bool) error {
	result := dbFromContext(ctx, r.db).WithContext(ctx).Model(&entity.AgentConfig{}).
		Where("user_id = ? AND knowledge_base_id = ?", userID, kbID).
		Update("network_enabled", enabled)
	if result.Error != nil {
		return fmt.Errorf("更新 Agent 配置联网开关失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAgentConfigNotFound
	}
	return nil
}

// modelConfigRepository 是 ModelConfigRepository 接口的 GORM 实现。
type modelConfigRepository struct{ db *gorm.DB }

// NewModelConfigRepository 创建一个新的模型配置仓储实例。
func NewModelConfigRepository(db *gorm.DB) ModelConfigRepository {
	return &modelConfigRepository{db: db}
}

// FindChatByID 查询指定用户的 Chat 模型配置。
func (r *modelConfigRepository) FindChatByID(ctx context.Context, userID, modelID string) (*entity.ModelConfig, error) {
	var cfg entity.ModelConfig
	// 同时校验归属(user_id)、模型类型、启用状态，并排除软删除行，防止越权访问停用配置。
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ? AND model_type = 'chat' AND enabled = true AND deleted_at IS NULL", modelID, userID).
		First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询模型配置失败: %w", err)
	}
	return &cfg, nil
}

// FindEnabledByID 查询指定用户的任意已启用模型配置。
func (r *modelConfigRepository) FindEnabledByID(ctx context.Context, userID, modelID string) (*entity.ModelConfig, error) {
	var cfg entity.ModelConfig
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("id = ? AND user_id = ? AND enabled = true AND deleted_at IS NULL", modelID, userID).
		First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询模型配置失败: %w", err)
	}
	return &cfg, nil
}

// FindDefaultChat 查询指定用户的默认 Chat 模型配置。
func (r *modelConfigRepository) FindDefaultChat(ctx context.Context, userID string) (*entity.ModelConfig, error) {
	var cfg entity.ModelConfig
	// 取该用户最近更新的默认 Chat 模型，避免多行默认配置时结果不确定。
	err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND model_type = 'chat' AND is_default = true AND enabled = true AND deleted_at IS NULL", userID).
		Order("updated_at DESC").First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrModelConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询默认模型配置失败: %w", err)
	}
	return &cfg, nil
}
