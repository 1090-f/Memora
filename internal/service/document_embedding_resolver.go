package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino/components/embedding"
)

// DocumentEmbeddingResolver 按任务所有权解析 Embedding 模型，未配置时返回空模型以保留关键词链。
type DocumentEmbeddingResolver interface {
	Resolve(ctx context.Context, userID, knowledgeBaseID string) (string, embedding.Embedder, error)
}

type documentEmbeddingResolver struct {
	kbs     repository.KnowledgeBaseRepository
	models  repository.AIModelConfigRepository
	factory contracts.ModelFactory
}

func NewDocumentEmbeddingResolver(kbs repository.KnowledgeBaseRepository, models repository.AIModelConfigRepository, factory contracts.ModelFactory) (DocumentEmbeddingResolver, error) {
	if kbs == nil || models == nil || factory == nil {
		return nil, fmt.Errorf("文档 Embedding 解析器依赖不完整")
	}
	return &documentEmbeddingResolver{kbs: kbs, models: models, factory: factory}, nil
}

func (r *documentEmbeddingResolver) Resolve(ctx context.Context, userID, knowledgeBaseID string) (string, embedding.Embedder, error) {
	kb, err := r.kbs.FindByID(ctx, userID, knowledgeBaseID)
	if err != nil {
		return "", nil, err
	}
	var modelID string
	if kb.DefaultEmbeddingModelID != nil {
		modelID = *kb.DefaultEmbeddingModelID
	}
	if modelID == "" {
		cfg, findErr := r.models.FindDefaultByUserAndType(ctx, userID, "embedding")
		if errors.Is(findErr, repository.ErrModelConfigNotFound) {
			return "", nil, nil
		}
		if findErr != nil {
			return "", nil, findErr
		}
		modelID = cfg.ID
	} else if _, err := r.models.FindByIDForUserAndType(ctx, userID, modelID, "embedding"); err != nil {
		return "", nil, err
	}
	model, err := r.factory.GetEmbeddingModel(ctx, contracts.ID(modelID))
	if err != nil {
		return "", nil, err
	}
	adapter, err := einoadapter.NewContractsEmbeddingAdapter(model)
	if err != nil {
		return "", nil, err
	}
	return modelID, adapter, nil
}
