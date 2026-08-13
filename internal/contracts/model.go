package contracts

import (
	"context"
	"encoding/json"
)

// ChatMessage 表示聊天对话中的单条消息。
type ChatMessage struct {
	Role    string `json:"role"`    // 角色：system / user / assistant / tool
	Content string `json:"content"` // 消息内容
}

// ChatRequest 表示从 AI 模型生成聊天响应的请求。
type ChatRequest struct {
	Messages    []ChatMessage   `json:"messages"`              // 对话历史
	Tools       json.RawMessage `json:"tools,omitempty"`       // 可选：工具定义（原始 JSON）
	MaxTokens   int             `json:"max_tokens,omitempty"`  // 可选：输出最大 token
	Temperature float64         `json:"temperature,omitempty"` // 可选：采样温度
}

// ChatResponse 表示 AI 模型聊天生成的响应。
type ChatResponse struct {
	Content   string     `json:"content"`              // 生成的文本内容
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // 可选：模型请求调用的工具
	Usage     TokenUsage `json:"usage"`                // token 消耗
}

// ChatStreamEvent 表示流式聊天响应中的单个事件。
type ChatStreamEvent struct {
	Delta     string      `json:"delta,omitempty"`      // 文本增量
	ToolCalls []ToolCall  `json:"tool_calls,omitempty"` // 可选：工具调用列表
	Usage     *TokenUsage `json:"usage,omitempty"`      // 可选：结束时 token 消耗
	Done      bool        `json:"done"`                 // 是否结束
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
	Index int     `json:"index"` // 原始文档的索引位置
	Score float64 `json:"score"` // 重排后的相关度评分
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
