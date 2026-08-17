package entity

import "time"

// AgentConfig 表示知识库 Agent 配置实体，映射到 agent_configs 数据库表。
// 每个知识库对应一条。成员一只负责创建时写入默认行，业务规则与工具授权由成员二/三负责。
// Status 取值：active（启用）/ disabled（禁用）。
type AgentConfig struct {
	ID                    string    `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()" json:"id"` // ID 主键（UUID）
	UserID                string    `gorm:"column:user_id" json:"user_id"`                                      // UserID 所属用户 ID
	KnowledgeBaseID       string    `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`                  // KnowledgeBaseID 关联的知识库 ID
	Name                  string    `gorm:"column:name" json:"name"`                                            // Name Agent 名称
	SystemPrompt          *string   `gorm:"column:system_prompt" json:"system_prompt,omitempty"`                // SystemPrompt 系统提示词，可选
	ChatModelID           string    `gorm:"column:chat_model_id" json:"chat_model_id"`                          // ChatModelID 关联的对话模型配置 ID
	MaxReactRounds        int       `gorm:"column:max_react_rounds" json:"max_react_rounds"`                    // MaxReactRounds ReAct 模式最大轮数
	MaxPlanSteps          int       `gorm:"column:max_plan_steps" json:"max_plan_steps"`                        // MaxPlanSteps Plan-Execute 最大步骤数
	MaxReplans            int       `gorm:"column:max_replans" json:"max_replans"`                              // MaxReplans Plan-Execute 最大重规划次数
	ReviewerRuns          int       `gorm:"column:reviewer_runs" json:"reviewer_runs"`                          // ReviewerRuns Plan-Execute 审查次数
	MaxToolCalls          int       `gorm:"column:max_tool_calls" json:"max_tool_calls"`                        // MaxToolCalls 单次运行最大工具调用次数
	MaxDocumentReadTokens int       `gorm:"column:max_document_read_tokens" json:"max_document_read_tokens"`    // MaxDocumentReadTokens 读取文档的最大 token 数
	MaxToolResultBytes    int       `gorm:"column:max_tool_result_bytes" json:"max_tool_result_bytes"`          // MaxToolResultBytes 工具返回结果的最大字节数
	MaxRunSeconds         int       `gorm:"column:max_run_seconds" json:"max_run_seconds"`                      // MaxRunSeconds 单次运行的超时秒数
	NetworkEnabled        bool      `gorm:"column:network_enabled" json:"network_enabled"`                      // NetworkEnabled 是否允许联网搜索
	MemoryEnabled         bool      `gorm:"column:memory_enabled" json:"memory_enabled"`                        // MemoryEnabled 是否启用记忆功能
	MemoryTopK            int       `gorm:"column:memory_top_k" json:"memory_top_k"`                            // MemoryTopK 记忆检索返回的最大条数
	ShowExecutionStatus   bool      `gorm:"column:show_execution_status" json:"show_execution_status"`          // ShowExecutionStatus 是否向前端展示执行状态
	Status                string    `gorm:"column:status" json:"status"`                                        // Status 配置状态：active/disabled
	CreatedAt             time.Time `gorm:"column:created_at" json:"created_at"`                                // CreatedAt 创建时间
	UpdatedAt             time.Time `gorm:"column:updated_at" json:"updated_at"`                                // UpdatedAt 更新时间
}

// TableName 返回 Agent 配置实体对应的数据库表名。
func (AgentConfig) TableName() string { return "agent_configs" }
