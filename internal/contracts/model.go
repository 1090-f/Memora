package contracts

import (
	"context"
	"encoding/json"
)

// ChatMessage 表示聊天对话中的单条消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 表示从 AI 模型生成聊天响应的请求。
type ChatRequest struct {
	Messages    []ChatMessage   `json:"messages"`
	Tools       json.RawMessage `json:"tools,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

// ChatResponse 表示 AI 模型聊天生成的响应。
type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     TokenUsage `json:"usage"`
}

// ChatStreamEvent 表示流式聊天响应中的单个事件。
type ChatStreamEvent struct {
	Delta    string      `json:"delta,omitempty"`
	ToolCall *ToolCall   `json:"tool_call,omitempty"`
	Usage    *TokenUsage `json:"usage,omitempty"`
	Done     bool        `json:"done"`
}

// ChatModel 定义 AI 聊天模型生成文本响应的接口。
type ChatModel interface {
	// Generate 为给定请求生成完整的聊天响应。
	Generate(ctx context.Context, request ChatRequest) (ChatResponse, error)
	// Stream 为给定请求生成流式聊天事件通道。
	Stream(ctx context.Context, request ChatRequest) (<-chan ChatStreamEvent, error)
}

// EmbeddingModel 定义 AI 模型生成文本嵌入向量的接口。
type EmbeddingModel interface {
	// Embed 为给定文本生成嵌入向量。
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension 返回嵌入向量的维度。
	Dimension() int
}

// RerankItem 表示重排序后带有相关性分数的文档。
type RerankItem struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

// Reranker 定义 AI 模型按相关性对文档进行重排序的接口。
type Reranker interface {
	// Rerank 按与查询的相关性对文档进行重排序并返回前 K 个结果。
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankItem, error)
}

// ModelFactory 通过配置 ID 提供 AI 模型实例的访问。
type ModelFactory interface {
	// GetChatModel 返回指定配置 ID 的 ChatModel 实例。
	GetChatModel(ctx context.Context, modelConfigID ID) (ChatModel, error)
	// GetEmbeddingModel 返回指定配置 ID 的 EmbeddingModel 实例。
	GetEmbeddingModel(ctx context.Context, modelConfigID ID) (EmbeddingModel, error)
	// GetReranker 返回指定配置 ID 的 Reranker 实例。
	GetReranker(ctx context.Context, modelConfigID ID) (Reranker, error)
}
