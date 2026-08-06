package service

import (
	"context"

	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
)

// DirectoryService 定义文档目录管理业务逻辑的接口。
type DirectoryService interface {
	// GetTree 查询知识库目录树。
	GetTree(ctx context.Context, userID, kbID string) ([]*dto.DirectoryNode, error)
	// Create 创建目录，校验归属、父目录与最大深度。
	Create(ctx context.Context, userID, kbID string, req *request.CreateDirectoryRequest) (*dto.DirectoryNode, error)
}
