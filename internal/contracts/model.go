package contracts

import (
	"context"
	"encoding/json"
)

// ChatMessage 表示对话中的单条消息。
type ChatMessage struct {
	Role    string `json:"role"`    // 角色：system / user / assistant / tool
	Content string `json:"content"` // 消息内容
}

// ChatRequest 是模型对话生成请求。
type ChatRequest struct {
	Messages    []ChatMessage   `json:"messages"`              // 对话历史
	Tools       json.RawMessage `json:"tools,omitempty"`       // 可选：工具定义（原始 JSON）
	MaxTokens   int             `json:"max_tokens,omitempty"`  // 可选：输出最大 token
	Temperature float64         `json:"temperature,omitempty"` // 可选：采样温度
}

// ChatResponse 是模型对话生成的结果。
type ChatResponse struct {
	Content   string     `json:"content"`             // 生成的文本内容
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // 可选：模型请求调用的工具
	Usage     TokenUsage `json:"usage"`               // token 消耗
}

// ChatStreamEvent 是流式对话输出的单条增量事件。
type ChatStreamEvent struct {
	Delta    string      `json:"delta,omitempty"`     // 文本增量
	ToolCall *ToolCall   `json:"tool_call,omitempty"` // 可选：工具调用
	Usage    *TokenUsage `json:"usage,omitempty"`     // 可选：结束时 token 消耗
	Done     bool        `json:"done"`                // 是否结束
}

// ChatModel 抽象对话模型（LLM），支持普通生成与流式生成。
type ChatModel interface {
	// Generate 一次性生成完整响应。
	Generate(ctx context.Context, request ChatRequest) (ChatResponse, error)
	// Stream 返回增量事件通道，用于流式输出。
	Stream(ctx context.Context, request ChatRequest) (<-chan ChatStreamEvent, error)
}

// EmbeddingModel 抽象文本向量化模型。
type EmbeddingModel interface {
	// Embed 将多段文本转为向量。
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension 返回向量维度。
	Dimension() int
}

// RerankItem 是重排（Rerank）结果中的单项。
type RerankItem struct {
	Index int     `json:"index"` // 原始文档的索引位置
	Score float64 `json:"score"` // 重排后的相关度评分
}

// Reranker 抽象重排模型，用于提升检索结果相关性。
type Reranker interface {
	// Rerank 对给定文档按查询相关性重排，返回 topK 项。
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankItem, error)
}

// ModelFactory 是模型工厂，负责按模型配置 ID 创建不同类型的模型实例。
type ModelFactory interface {
	GetChatModel(ctx context.Context, modelConfigID ID) (ChatModel, error)       // 获取对话模型
	GetEmbeddingModel(ctx context.Context, modelConfigID ID) (EmbeddingModel, error) // 获取文本模型
	GetReranker(ctx context.Context, modelConfigID ID) (Reranker, error)        // 获取重排模型
}