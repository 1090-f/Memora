package contracts

import (
	"context"
	"encoding/json"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages    []ChatMessage   `json:"messages"`
	Tools       json.RawMessage `json:"tools,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     TokenUsage `json:"usage"`
}

type ChatStreamEvent struct {
	Delta    string      `json:"delta,omitempty"`
	ToolCall *ToolCall   `json:"tool_call,omitempty"`
	Usage    *TokenUsage `json:"usage,omitempty"`
	Done     bool        `json:"done"`
}

type ChatModel interface {
	Generate(ctx context.Context, request ChatRequest) (ChatResponse, error)
	Stream(ctx context.Context, request ChatRequest) (<-chan ChatStreamEvent, error)
}

type EmbeddingModel interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

type RerankItem struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

type Reranker interface {
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankItem, error)
}

type ModelFactory interface {
	GetChatModel(ctx context.Context, modelConfigID ID) (ChatModel, error)
	GetEmbeddingModel(ctx context.Context, modelConfigID ID) (EmbeddingModel, error)
	GetReranker(ctx context.Context, modelConfigID ID) (Reranker, error)
}
