package ai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/1090-f/Memora/internal/ai/encryption"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	einoembedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	einomodel "github.com/cloudwego/eino-ext/components/model/openai"
)

// ChatModelConfig 表示创建 ChatModel 所需的配置。
type ChatModelConfig struct {
	Model             string
	BaseURL           string
	APIKey            string
	TimeoutSeconds    int
	MaxTokens         int
	Temperature       float64
	SupportsStreaming bool
}

// EmbeddingModelConfig 表示创建 EmbeddingModel 所需的配置。
type EmbeddingModelConfig struct {
	Model           string
	BaseURL         string
	APIKey          string
	TimeoutSeconds  int
	VectorDimension int
}

// RerankerConfig 表示创建 Reranker 所需的配置。
type RerankerConfig struct {
	Model          string
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

// einoProviderFactory 是 ProviderFactory 接口的 Eino 实现。
type einoProviderFactory struct{}

// NewProviderFactory 创建一个新的 ProviderFactory 实例。
func NewProviderFactory() ProviderFactory {
	return &einoProviderFactory{}
}

// CreateChatModel 根据配置创建 ChatModel 客户端。
func (f *einoProviderFactory) CreateChatModel(ctx context.Context, config ChatModelConfig) (contracts.ChatModel, error) {
	// 类型转换
	maxTokens := config.MaxTokens
	temp := float32(config.Temperature)

	// 使用 Eino OpenAI 组件创建 ChatModel
	chatModel, err := einomodel.NewChatModel(ctx, &einomodel.ChatModelConfig{
		Model:       config.Model,
		APIKey:      config.APIKey,
		BaseURL:     config.BaseURL,
		Timeout:     time.Duration(config.TimeoutSeconds) * time.Second,
		MaxTokens:   &maxTokens,
		Temperature: &temp,
	})
	if err != nil {
		return nil, fmt.Errorf("create eino chat model: %w", err)
	}

	// 包装为 contracts.ChatModel
	return &einoChatModelAdapter{model: chatModel}, nil
}

// CreateEmbeddingModel 根据配置创建 EmbeddingModel 客户端。
func (f *einoProviderFactory) CreateEmbeddingModel(ctx context.Context, config EmbeddingModelConfig) (contracts.EmbeddingModel, error) {
	// 使用 Eino OpenAI 组件创建 Embedder
	embedder, err := einoembedding.NewEmbedder(ctx, &einoembedding.EmbeddingConfig{
		Model:   config.Model,
		APIKey:  config.APIKey,
		BaseURL: config.BaseURL,
		Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("create eino embedding model: %w", err)
	}

	// 包装为 contracts.EmbeddingModel
	return &einoEmbeddingModelAdapter{
		embedder:  embedder,
		dimension: config.VectorDimension,
	}, nil
}

// CreateReranker 根据配置创建 Reranker 客户端。
func (f *einoProviderFactory) CreateReranker(ctx context.Context, config RerankerConfig) (contracts.Reranker, error) {
	// 使用 go-openai 实现 Reranker
	return &goOpenAIReranker{
		apiKey: config.APIKey, baseURL: config.BaseURL, model: config.Model,
		client: &http.Client{Timeout: time.Duration(config.TimeoutSeconds) * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}, nil
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
		Model:             config.Name,
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
		Model:           config.Name,
		BaseURL:         config.BaseURL,
		APIKey:          apiKey,
		TimeoutSeconds:  config.TimeoutSeconds,
		VectorDimension: dim,
	}
}

// buildRerankerConfig 从实体配置构建 RerankerConfig。
func buildRerankerConfig(config *entity.AIModelConfig, apiKey string) RerankerConfig {
	return RerankerConfig{
		Model:          config.Name,
		BaseURL:        config.BaseURL,
		APIKey:         apiKey,
		TimeoutSeconds: config.TimeoutSeconds,
	}
}

// 确保 encryption 包被使用
var _ = encryption.Base64Encode
