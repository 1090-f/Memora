// Package response 定义 API 响应 DTO，遵循 snake_case JSON 命名约定。
package response

import "time"

// AgentRunResponse 表示 Agent 运行详情的响应。
// 用于展示运行状态、路由决策、执行轨迹和最终结果。
type AgentRunResponse struct {
	ID              string     `json:"id"`                         // ID 运行 ID
	UserID          string     `json:"user_id"`                    // UserID 所属用户 ID
	KnowledgeBaseID string     `json:"knowledge_base_id"`          // KnowledgeBaseID 所属知识库 ID
	ConversationID  string     `json:"conversation_id"`            // ConversationID 关联会话 ID
	AgentConfigID   string     `json:"agent_config_id"`            // AgentConfigID 使用的 Agent 配置 ID
	RetryOfRunID    *string    `json:"retry_of_run_id,omitempty"`  // RetryOfRunID 重试的原始运行 ID
	Query           string     `json:"query"`                      // Query 用户查询原文
	ExecutionMode   *string    `json:"execution_mode,omitempty"`   // ExecutionMode Router 选择的执行模式
	RouterReason    string     `json:"router_reason,omitempty"`    // RouterReason Router 选择原因摘要
	KnowledgeStatus *string    `json:"knowledge_status,omitempty"` // KnowledgeStatus 知识充分性状态
	Status          string     `json:"status"`                     // Status 运行状态
	ReplanCount     int        `json:"replan_count"`               // ReplanCount 重新规划次数
	ReviewerResult  *string    `json:"reviewer_result,omitempty"`  // ReviewerResult 评审结果
	MemoryUsedCount int        `json:"memory_used_count"`          // MemoryUsedCount 使用的记忆条数
	InputTokens     int        `json:"input_tokens"`               // InputTokens 输入 Token 数
	OutputTokens    int        `json:"output_tokens"`              // OutputTokens 输出 Token 数
	TotalTokens     int        `json:"total_tokens"`               // TotalTokens 总 Token 数
	DurationMs      *int64     `json:"duration_ms,omitempty"`      // DurationMs 执行耗时（毫秒）
	FinalResult     *string    `json:"final_result,omitempty"`     // FinalResult 最终回答
	ErrorCode       *string    `json:"error_code,omitempty"`       // ErrorCode 错误码
	ErrorMessage    *string    `json:"error_message,omitempty"`    // ErrorMessage 错误信息
	StartedAt       *time.Time `json:"started_at,omitempty"`       // StartedAt 开始时间
	EndedAt         *time.Time `json:"ended_at,omitempty"`         // EndedAt 结束时间
	CreatedAt       time.Time  `json:"created_at"`                 // CreatedAt 创建时间
}

// AgentRunListItem 表示 Agent 运行列表项（不含最终结果和详细轨迹）。
type AgentRunListItem struct {
	ID            string    `json:"id"`                       // ID 运行 ID
	Query         string    `json:"query"`                    // Query 用户查询原文
	ExecutionMode *string   `json:"execution_mode,omitempty"` // ExecutionMode 执行模式
	Status        string    `json:"status"`                   // Status 运行状态
	TotalTokens   int       `json:"total_tokens"`             // TotalTokens 总 Token 数
	DurationMs    *int64    `json:"duration_ms,omitempty"`    // DurationMs 执行耗时（毫秒）
	ErrorCode     *string   `json:"error_code,omitempty"`     // ErrorCode 错误码
	CreatedAt     time.Time `json:"created_at"`               // CreatedAt 创建时间
}

// AgentRunList 表示 Agent 运行记录的分页列表响应。
type AgentRunList struct {
	Items    []*AgentRunListItem `json:"items"`     // Items 运行列表
	Page     int                 `json:"page"`      // Page 当前页码（从 1 开始）
	PageSize int                 `json:"page_size"` // PageSize 每页条数
	Total    int64               `json:"total"`     // Total 运行总数
}

// CreateAgentRunResponse 表示创建 Agent 运行成功后的响应。
type CreateAgentRunResponse struct {
	RunID          string `json:"run_id"`          // RunID 新创建的运行 ID
	ConversationID string `json:"conversation_id"` // ConversationID 所属会话 ID
	Status         string `json:"status"`          // Status 初始状态（queued）
}

// RetryAgentRunResponse 表示重试 Agent 运行成功后的响应。
type RetryAgentRunResponse struct {
	NewRunID string `json:"new_run_id"` // NewRunID 新创建的运行 ID
	Status   string `json:"status"`     // Status 初始状态（queued）
}
