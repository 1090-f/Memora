package repository

import (
	"context"

	"github.com/1090-f/Memora/internal/model/entity"
)

// SearchConfigRepository 定义搜索配置数据访问接口。
// search_configs 无 user_id 列，归属通过 knowledge_base_id 关联的知识库推导。
type SearchConfigRepository interface {
	// Create 创建搜索配置。
	Create(ctx context.Context, cfg *entity.SearchConfig) error
	// FindByKnowledgeBase 按知识库查询搜索配置。
	FindByKnowledgeBase(ctx context.Context, kbID string) (*entity.SearchConfig, error)
	// Update 更新搜索配置，返回更新后的实体。
	Update(ctx context.Context, cfg *entity.SearchConfig) (*entity.SearchConfig, error)
}

// AgentConfigRepository 定义 Agent 配置数据访问接口。
// 成员一只负责默认行创建与查询，业务规则由成员二负责。
type AgentConfigRepository interface {
	// Create 创建 Agent 配置默认行。
	Create(ctx context.Context, cfg *entity.AgentConfig) error
	// FindByKnowledgeBase 按知识库查询 Agent 配置。
	FindByKnowledgeBase(ctx context.Context, userID, kbID string) (*entity.AgentConfig, error)
}

// ModelConfigRepository 定义模型配置的最小只读接口，仅供成员一校验模型归属与默认模型。
type ModelConfigRepository interface {
	// FindChatByID 查询指定用户的 Chat 模型配置，校验归属与启用状态。
	FindChatByID(ctx context.Context, userID, modelID string) (*entity.ModelConfig, error)
	// FindEnabledByID 查询指定用户的任意已启用模型配置（用于 Reranker 等模型校验）。
	FindEnabledByID(ctx context.Context, userID, modelID string) (*entity.ModelConfig, error)
	// FindDefaultChat 查询指定用户的默认 Chat 模型配置。
	FindDefaultChat(ctx context.Context, userID string) (*entity.ModelConfig, error)
}
