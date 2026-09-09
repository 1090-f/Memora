package service

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/chunking"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino/components/embedding"
)

// ResolvedDocumentEmbedding 是单次文档任务使用的模型与 tokenizer 快照。
type ResolvedDocumentEmbedding struct {
	ModelID        string
	Provider       string
	ModelName      string
	Embedder       embedding.Embedder
	Tokenizer      chunking.Tokenizer
	TokenizerExact bool
}

// DocumentEmbeddingResolver 按知识库不可变绑定解析 Embedding 模型及其 tokenizer。
type DocumentEmbeddingResolver interface {
	Resolve(ctx context.Context, userID, knowledgeBaseID string) (*ResolvedDocumentEmbedding, error)
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

func (r *documentEmbeddingResolver) Resolve(ctx context.Context, userID, knowledgeBaseID string) (*ResolvedDocumentEmbedding, error) {
	kb, err := r.kbs.FindByID(ctx, userID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	modelID := kb.EmbeddingModelID
	if modelID == "" {
		return nil, fmt.Errorf("知识库 %s 缺少绑定的 Embedding 模型", knowledgeBaseID)
	}
	modelConfig, err := r.models.FindByIDForUserAndType(ctx, userID, modelID, "embedding")
	if err != nil {
		return nil, err
	}
	tokenizerResolution, err := chunking.ResolveEmbeddingTokenizer(modelConfig.Provider, modelConfig.Name)
	if err != nil {
		return nil, err
	}
	model, err := r.factory.GetEmbeddingModel(ctx, contracts.ID(modelID))
	if err != nil {
		return nil, err
	}
	adapter, err := einoadapter.NewContractsEmbeddingAdapter(model)
	if err != nil {
		return nil, err
	}
	return &ResolvedDocumentEmbedding{
		ModelID: modelID, Provider: tokenizerResolution.Provider, ModelName: tokenizerResolution.Model,
		Embedder: adapter, Tokenizer: tokenizerResolution.Tokenizer, TokenizerExact: tokenizerResolution.Exact,
	}, nil
}
