// Package request 定义 API 请求 DTO，遵循 snake_case JSON 命名约定。
package request

// CreateAgentRunRequest 表示创建 Agent 运行（发起智能问答）的请求。
type CreateAgentRunRequest struct {
	KnowledgeBaseID string `json:"knowledge_base_id" binding:"required"` // 知识库 ID，必填
	ConversationID  string `json:"conversation_id" binding:"required"`   // 会话 ID，必填
	Query           string `json:"query" binding:"required,max=10000"`   // 用户问题原文，必填，最长 10000 字符
}

// AgentRunListFilter 表示 Agent 运行记录列表的查询过滤条件。
type AgentRunListFilter struct {
	Keyword string // 按查询关键词模糊搜索
}

// RetryAgentRunRequest 表示重试 Agent 运行的请求。
type RetryAgentRunRequest struct {
	RunID string `json:"run_id" binding:"required"` // 需要重试的失败运行 ID
}
