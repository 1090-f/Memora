package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/1090-f/Memora/internal/service/rag/pipeline"
	ragretrieval "github.com/1090-f/Memora/internal/service/rag/retrieval"
	"github.com/cloudwego/eino/components/embedding"
)

type retrievalRunner interface {
	Run(ctx context.Context, input pipeline.RetrievalInput) (contracts.RetrievalResult, error)
}

type retrievalService struct {
	kbs      repository.KnowledgeBaseRepository
	configs  repository.SearchConfigRepository
	models   repository.AIModelConfigRepository
	factory  contracts.ModelFactory
	pipeline retrievalRunner
}

// NewRetrievalService 创建生产 RetrievalService；Graph 必须已在应用初始化阶段编译。
func NewRetrievalService(kbs repository.KnowledgeBaseRepository, configs repository.SearchConfigRepository,
	models repository.AIModelConfigRepository, factory contracts.ModelFactory, runner retrievalRunner) (contracts.RetrievalService, error) {
	if kbs == nil || configs == nil || models == nil || factory == nil || runner == nil {
		return nil, fmt.Errorf("检索服务依赖不完整")
	}
	return &retrievalService{kbs: kbs, configs: configs, models: models, factory: factory, pipeline: runner}, nil
}

func (s *retrievalService) Retrieve(ctx context.Context, request contracts.RetrievalRequest) (contracts.RetrievalResult, error) {
	request.Query = strings.TrimSpace(request.Query)
	if request.UserID == "" || request.KnowledgeBaseID == "" || request.Query == "" || len([]rune(request.Query)) > 4096 {
		return contracts.RetrievalResult{}, apperrors.ErrInvalidArgument
	}
	if request.Mode == "" {
		request.Mode = contracts.RetrievalHybrid
	}
	if request.Mode != contracts.RetrievalKeyword && request.Mode != contracts.RetrievalVector && request.Mode != contracts.RetrievalHybrid {
		return contracts.RetrievalResult{}, apperrors.ErrInvalidArgument
	}
	if request.TopK <= 0 {
		request.TopK = 8
	}
	if request.TopK > 20 || len(request.DocumentIDs) > 100 {
		return contracts.RetrievalResult{}, apperrors.ErrInvalidArgument
	}
	kb, err := s.kbs.FindByID(ctx, string(request.UserID), string(request.KnowledgeBaseID))
	if err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return contracts.RetrievalResult{}, apperrors.ErrNotFound
		}
		return contracts.RetrievalResult{}, apperrors.New(contracts.ErrInternal, err)
	}
	cfg, err := s.configs.FindByKnowledgeBase(ctx, string(request.KnowledgeBaseID))
	if err != nil {
		return contracts.RetrievalResult{}, apperrors.New(contracts.ErrInternal, err)
	}
	request.Config = contracts.SearchConfig{
		KeywordTopK: cfg.KeywordTopK, VectorTopK: cfg.VectorTopK, RRFK: cfg.RRFK, RRFTopK: cfg.RRFTopK,
		RerankerTopK: cfg.RerankerTopK, RerankerThreshold: cfg.RerankerThreshold,
		MinimumEffectiveResult: cfg.MinimumEffectiveRate, MinVectorScore: cfg.MinVectorScore,
		AmbiguousScore: cfg.AmbiguousScore,
	}
	rerankerModelID := cfg.RerankerModelID
	if rerankerModelID == nil {
		rerankerModelID = kb.DefaultRerankerModelID
	}
	if rerankerModelID != nil {
		request.Config.RerankerModelID = contracts.ID(*rerankerModelID)
	}

	var embedder embedding.Embedder
	var modelCfgID string
	if request.Mode == contracts.RetrievalVector || request.Mode == contracts.RetrievalHybrid {
		var modelID string
		if kb.DefaultEmbeddingModelID != nil {
			modelID = *kb.DefaultEmbeddingModelID
		}
		if modelID != "" {
			modelCfg, modelErr := s.models.FindByIDForUserAndType(ctx, string(request.UserID), modelID, "embedding")
			if modelErr != nil {
				return contracts.RetrievalResult{}, apperrors.New(contracts.ErrServiceUnavailable, modelErr)
			}
			modelCfgID = modelCfg.ID
		} else {
			modelCfg, modelErr := s.models.FindDefaultByUserAndType(ctx, string(request.UserID), "embedding")
			if modelErr != nil {
				return contracts.RetrievalResult{}, apperrors.New(contracts.ErrServiceUnavailable, modelErr)
			}
			modelCfgID = modelCfg.ID
		}
		model, modelErr := s.factory.GetEmbeddingModel(ctx, contracts.ID(modelCfgID))
		if modelErr != nil {
			return contracts.RetrievalResult{}, mapModelError(modelErr)
		}
		embedder, modelErr = einoadapter.NewContractsEmbeddingAdapter(model)
		if modelErr != nil {
			return contracts.RetrievalResult{}, apperrors.New(contracts.ErrModelCallFailed, modelErr)
		}
	}

	var reranker contracts.Reranker
	if rerankerModelID != nil && *rerankerModelID != "" {
		if _, modelErr := s.models.FindByIDForUserAndType(ctx, string(request.UserID), *rerankerModelID, "reranker"); modelErr == nil {
			reranker, _ = s.factory.GetReranker(ctx, contracts.ID(*rerankerModelID))
		}
		// 冻结降级策略：Reranker 配置/创建失败时继续使用 RRF。
	}
	result, err := s.pipeline.Run(ctx, pipeline.RetrievalInput{Request: request, Embedder: embedder, Reranker: reranker, EmbeddingModelID: modelCfgID})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return contracts.RetrievalResult{}, apperrors.New(contracts.ErrUpstreamTimeout, err)
		}
		if errors.Is(err, ragretrieval.ErrQueryEmbedding) {
			return contracts.RetrievalResult{}, apperrors.New(contracts.ErrModelCallFailed, err)
		}
		return contracts.RetrievalResult{}, apperrors.New(contracts.ErrInternal, err)
	}
	return result, nil
}

func mapModelError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return apperrors.New(contracts.ErrUpstreamTimeout, err)
	}
	return apperrors.New(contracts.ErrModelCallFailed, err)
}
