package service

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
)

// KnowledgeBaseService 定义知识库管理业务逻辑的接口。
type KnowledgeBaseService interface {
	// Create 创建知识库，并在同一短事务中创建默认目录、搜索配置和 Agent 配置。
	Create(ctx context.Context, userID string, req *request.CreateKnowledgeBaseRequest) (*dto.KnowledgeBaseResponse, error)
	// List 分页查询用户知识库列表。
	List(ctx context.Context, userID string, page, pageSize int, keyword string) (*dto.KnowledgeBaseList, error)
	// Get 查询知识库详情。
	Get(ctx context.Context, userID, kbID string) (*dto.KnowledgeBaseResponse, error)
	// Update 修改知识库基础信息。
	Update(ctx context.Context, userID, kbID string, req *request.UpdateKnowledgeBaseRequest) (*dto.KnowledgeBaseResponse, error)
	// Delete 软删除知识库。
	Delete(ctx context.Context, userID, kbID string) error
	// GetSearchConfig 查询知识库搜索配置。
	GetSearchConfig(ctx context.Context, userID, kbID string) (*dto.SearchConfigResponse, error)
	// UpdateSearchConfig 更新知识库搜索配置并做范围校验。
	UpdateSearchConfig(ctx context.Context, userID, kbID string, req *request.UpdateSearchConfigRequest) (*dto.SearchConfigResponse, error)
}
