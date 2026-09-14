package app

import (
	"crypto/sha256"
	"fmt"

	"github.com/1090-f/Memora/internal/ai"
	"github.com/1090-f/Memora/internal/ai/encryption"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	ragpipeline "github.com/1090-f/Memora/internal/service/rag/pipeline"
	ragretrieval "github.com/1090-f/Memora/internal/service/rag/retrieval"
	"github.com/1090-f/Memora/pkg/config"
	"gorm.io/gorm"
)

// NewRetrievalServiceForDatabase 为离线工具装配与服务端相同的生产检索链路，
// 但不启动 HTTP、Redis、MinIO、文档解析器或后台 Worker。
func NewRetrievalServiceForDatabase(cfg *config.Config, db *gorm.DB) (contracts.RetrievalService, error) {
	if cfg == nil || db == nil {
		return nil, fmt.Errorf("检索装配缺少配置或数据库")
	}
	kbs := repository.NewKnowledgeBaseRepository(db)
	searchConfigs := repository.NewSearchConfigRepository(db)
	aiModelConfigs := repository.NewAIModelConfigRepository(db)

	keyMaterial := cfg.AI.EncryptionKey
	if keyMaterial == "" {
		keyMaterial = cfg.MCP.EncryptionKey
	}
	if keyMaterial == "" {
		keyMaterial = cfg.JWT.Secret
	}
	key := sha256.Sum256([]byte(keyMaterial))
	aiEncryption, err := encryption.NewService(key[:])
	if err != nil {
		return nil, fmt.Errorf("初始化 AI 凭证加密失败: %w", err)
	}
	modelFactory := ai.NewModelFactory(aiModelConfigs, aiEncryption, ai.NewProviderFactory())

	keywordRetriever, err := ragretrieval.NewParadeDBKeywordRetriever(repository.NewKeywordSearchRepository(db))
	if err != nil {
		return nil, err
	}
	vectorRetriever, err := ragretrieval.NewPgVectorRetriever(repository.NewVectorRepository(db))
	if err != nil {
		return nil, err
	}
	retrievalPipeline, err := ragpipeline.NewRetrievalPipeline(keywordRetriever, vectorRetriever, service.NewCitationService())
	if err != nil {
		return nil, err
	}
	return service.NewRetrievalService(kbs, searchConfigs, aiModelConfigs, modelFactory, retrievalPipeline)
}
