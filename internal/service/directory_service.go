package service

import (
	"context"
	"errors"
	"strings"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/dto/request"
	dto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
)

// 目录最大深度，与 document_directories.depth CHECK 一致。
const maxDirectoryDepth = 5

// directoryService 是 DirectoryService 接口的实现。
type directoryService struct {
	kbs  repository.KnowledgeBaseRepository
	dirs repository.DocumentDirectoryRepository
}

// NewDirectoryService 创建一个新的文档目录服务实例。
func NewDirectoryService(kbs repository.KnowledgeBaseRepository, dirs repository.DocumentDirectoryRepository) DirectoryService {
	return &directoryService{kbs: kbs, dirs: dirs}
}

// GetTree 查询知识库目录树。
func (s *directoryService) GetTree(ctx context.Context, userID, kbID string) ([]*dto.DirectoryNode, error) {
	if _, err := s.kbs.FindByID(ctx, userID, kbID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	items, err := s.dirs.ListByKB(ctx, userID, kbID)
	if err != nil {
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	nodes := make(map[string]*dto.DirectoryNode, len(items))
	roots := make([]*dto.DirectoryNode, 0)
	for _, dir := range items {
		nodes[dir.ID] = directoryNode(dir)
	}
	for _, dir := range items {
		node := nodes[dir.ID]
		if dir.ParentID != nil {
			if parent, ok := nodes[*dir.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots, nil
}

// Create 创建目录，校验归属、父目录与最大深度。
func (s *directoryService) Create(ctx context.Context, userID, kbID string, req *request.CreateDirectoryRequest) (*dto.DirectoryNode, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	if _, err := s.kbs.FindByID(ctx, userID, kbID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	depth := 1
	var parentID *string
	if req.ParentID != nil && *req.ParentID != "" {
		parent, err := s.dirs.FindByIDInKB(ctx, userID, kbID, *req.ParentID)
		if errors.Is(err, repository.ErrDirectoryNotFound) {
			return nil, apperrors.New(contracts.ErrInvalidArgument, err)
		}
		if err != nil {
			return nil, apperrors.New(contracts.ErrInternal, err)
		}
		if parent.Depth >= maxDirectoryDepth {
			return nil, apperrors.New(contracts.ErrInvalidArgument, errors.New("目录深度超过上限"))
		}
		depth = parent.Depth + 1
		id := parent.ID
		parentID = &id
	}
	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}
	dir := &entity.DocumentDirectory{
		UserID: userID, KnowledgeBaseID: kbID, ParentID: parentID,
		Name: strings.TrimSpace(req.Name), Depth: depth, SortOrder: sortOrder, IsDefault: false,
	}
	if err := s.dirs.Create(ctx, dir); err != nil {
		if errors.Is(err, repository.ErrDirectoryConflict) {
			return nil, apperrors.ErrConflict
		}
		return nil, apperrors.New(contracts.ErrInternal, err)
	}
	return directoryNode(dir), nil
}

func directoryNode(dir *entity.DocumentDirectory) *dto.DirectoryNode {
	return &dto.DirectoryNode{
		ID: dir.ID, Name: dir.Name, ParentID: dir.ParentID,
		Depth: dir.Depth, SortOrder: dir.SortOrder, IsDefault: dir.IsDefault,
		Children: make([]*dto.DirectoryNode, 0),
	}
}
