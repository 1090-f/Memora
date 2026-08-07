package ai

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/ai/encryption"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
)

// ModelFactoryImpl 是 contracts.ModelFactory 接口的实现。
type ModelFactoryImpl struct {
	configRepo      repository.AIModelConfigRepository
	encryptionSvc   encryption.Service
	providerFactory ProviderFactory
}

// NewModelFactory 创建一个新的 ModelFactory 实例。
func NewModelFactory(
	configRepo repository.AIModelConfigRepository,
	encryptionSvc encryption.Service,
	providerFactory ProviderFactory,
) contracts.ModelFactory {
	return &ModelFactoryImpl{
		configRepo:      configRepo,
		encryptionSvc:   encryptionSvc,
		providerFactory: providerFactory,
	}
}

// GetChatModel 返回指定配置 ID 的 ChatModel 实例。
func (f *ModelFactoryImpl) GetChatModel(ctx context.Context, modelConfigID contracts.ID) (contracts.ChatModel, error) {
	config, err := f.getConfig(ctx, string(modelConfigID), "chat")
	if err != nil {
		return nil, err
	}

	apiKey, err := f.decryptAPIKey(config.APIKeyCiphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}

	chatConfig := buildChatModelConfig(config, apiKey)
	return f.providerFactory.CreateChatModel(ctx, chatConfig)
}

// GetEmbeddingModel 返回指定配置 ID 的 EmbeddingModel 实例。
func (f *ModelFactoryImpl) GetEmbeddingModel(ctx context.Context, modelConfigID contracts.ID) (contracts.EmbeddingModel, error) {
	config, err := f.getConfig(ctx, string(modelConfigID), "embedding")
	if err != nil {
		return nil, err
	}

	apiKey, err := f.decryptAPIKey(config.APIKeyCiphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}

	embedConfig := buildEmbeddingModelConfig(config, apiKey)
	return f.providerFactory.CreateEmbeddingModel(ctx, embedConfig)
}

// GetReranker 返回指定配置 ID 的 Reranker 实例。
func (f *ModelFactoryImpl) GetReranker(ctx context.Context, modelConfigID contracts.ID) (contracts.Reranker, error) {
	config, err := f.getConfig(ctx, string(modelConfigID), "reranker")
	if err != nil {
		return nil, err
	}

	apiKey, err := f.decryptAPIKey(config.APIKeyCiphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}

	rerankerConfig := buildRerankerConfig(config, apiKey)
	return f.providerFactory.CreateReranker(ctx, rerankerConfig)
}

// getConfig 获取模型配置，如果 modelConfigID 为空则获取默认配置。
func (f *ModelFactoryImpl) getConfig(ctx context.Context, modelConfigID, modelType string) (*entity.AIModelConfig, error) {
	var config *entity.AIModelConfig
	var err error

	if modelConfigID != "" {
		config, err = f.configRepo.FindByID(ctx, modelConfigID)
	} else {
		// 如果未指定配置 ID，需要从上下文获取用户 ID
		// 这里暂时返回错误，实际使用时需要传入用户 ID
		return nil, fmt.Errorf("model config ID is required")
	}

	if err != nil {
		return nil, fmt.Errorf("get model config: %w", err)
	}

	// 验证模型类型
	if config.ModelType != modelType {
		return nil, fmt.Errorf("model config type mismatch: expected %s, got %s", modelType, config.ModelType)
	}

	return config, nil
}

// decryptAPIKey 解密 API Key。
func (f *ModelFactoryImpl) decryptAPIKey(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}
	return f.encryptionSvc.Decrypt(ciphertext)
}
