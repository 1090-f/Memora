package ai

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/ai/encryption"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
)

// ChatModelConfig 表示创建 ChatModel 所需的配置。
type ChatModelConfig struct {
	BaseURL           string
	APIKey            string
	TimeoutSeconds    int
	MaxTokens         int
	Temperature       float64
	SupportsStreaming bool
}

// EmbeddingModelConfig 表示创建 EmbeddingModel 所需的配置。
type EmbeddingModelConfig struct {
	BaseURL         string
	APIKey          string
	TimeoutSeconds  int
	VectorDimension int
}

// RerankerConfig 表示创建 Reranker 所需的配置。
type RerankerConfig struct {
	BaseURL        string
	APIKey         string
	TimeoutSeconds int
}

// ProviderFactory 定义创建模型客户端的接口。
type ProviderFactory interface {
	// CreateChatModel 根据配置创建 ChatModel 客户端。
	CreateChatModel(ctx context.Context, config ChatModelConfig) (contracts.ChatModel, error)
	// CreateEmbeddingModel 根据配置创建 EmbeddingModel 客户端。
	CreateEmbeddingModel(ctx context.Context, config EmbeddingModelConfig) (contracts.EmbeddingModel, error)
	// CreateReranker 根据配置创建 Reranker 客户端。
	CreateReranker(ctx context.Context, config RerankerConfig) (contracts.Reranker, error)
}

// openAICompatibleFactory 是 ProviderFactory 接口的 OpenAI 兼容实现。
type openAICompatibleFactory struct{}

// NewProviderFactory 创建一个新的 ProviderFactory 实例。
func NewProviderFactory() ProviderFactory {
	return &openAICompatibleFactory{}
}

// CreateChatModel 根据配置创建 ChatModel 客户端。
func (f *openAICompatibleFactory) CreateChatModel(ctx context.Context, config ChatModelConfig) (contracts.ChatModel, error) {
	// TODO: 根据 provider 实现具体的模型客户端创建逻辑
	// 这里返回一个占位实现，后续需要接入实际的 LLM SDK
	return nil, fmt.Errorf("chat model provider not implemented")
}

// CreateEmbeddingModel 根据配置创建 EmbeddingModel 客户端。
func (f *openAICompatibleFactory) CreateEmbeddingModel(ctx context.Context, config EmbeddingModelConfig) (contracts.EmbeddingModel, error) {
	// TODO: 根据 provider 实现具体的嵌入模型客户端创建逻辑
	return nil, fmt.Errorf("embedding model provider not implemented")
}

// CreateReranker 根据配置创建 Reranker 客户端。
func (f *openAICompatibleFactory) CreateReranker(ctx context.Context, config RerankerConfig) (contracts.Reranker, error) {
	// TODO: 根据 provider 实现具体的重排器客户端创建逻辑
	return nil, fmt.Errorf("reranker provider not implemented")
}

// buildChatModelConfig 从实体配置构建 ChatModelConfig。
func buildChatModelConfig(config *entity.AIModelConfig, apiKey string) ChatModelConfig {
	maxTokens := 4096
	if config.MaxTokens != nil {
		maxTokens = *config.MaxTokens
	}
	temp := 0.7
	if config.Temperature != nil {
		temp = *config.Temperature
	}
	return ChatModelConfig{
		BaseURL:           config.BaseURL,
		APIKey:            apiKey,
		TimeoutSeconds:    config.TimeoutSeconds,
		MaxTokens:         maxTokens,
		Temperature:       temp,
		SupportsStreaming: config.SupportsStreaming,
	}
}

// buildEmbeddingModelConfig 从实体配置构建 EmbeddingModelConfig。
func buildEmbeddingModelConfig(config *entity.AIModelConfig, apiKey string) EmbeddingModelConfig {
	dim := 1024
	if config.VectorDimension != nil {
		dim = *config.VectorDimension
	}
	return EmbeddingModelConfig{
		BaseURL:         config.BaseURL,
		APIKey:          apiKey,
		TimeoutSeconds:  config.TimeoutSeconds,
		VectorDimension: dim,
	}
}

// buildRerankerConfig 从实体配置构建 RerankerConfig。
func buildRerankerConfig(config *entity.AIModelConfig, apiKey string) RerankerConfig {
	return RerankerConfig{
		BaseURL:        config.BaseURL,
		APIKey:         apiKey,
		TimeoutSeconds: config.TimeoutSeconds,
	}
}

// 确保 encryption 包被使用
var _ = encryption.Base64Encode
