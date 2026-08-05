package entity

import "time"

// AgentConfig 表示知识库 Agent 配置实体，映射到 agent_configs 数据库表。
// 每个知识库对应一条。成员一只负责创建时写入默认行，业务规则与工具授权由成员二/三负责。
type AgentConfig struct {
	ID                    string    `gorm:"column:id" json:"id"`
	UserID                string    `gorm:"column:user_id" json:"user_id"`
	KnowledgeBaseID       string    `gorm:"column:knowledge_base_id" json:"knowledge_base_id"`
	Name                  string    `gorm:"column:name" json:"name"`
	SystemPrompt          *string   `gorm:"column:system_prompt" json:"system_prompt,omitempty"`
	ChatModelID           string    `gorm:"column:chat_model_id" json:"chat_model_id"`
	MaxReactRounds        int       `gorm:"column:max_react_rounds" json:"max_react_rounds"`
	MaxPlanSteps          int       `gorm:"column:max_plan_steps" json:"max_plan_steps"`
	MaxReplans            int       `gorm:"column:max_replans" json:"max_replans"`
	ReviewerRuns          int       `gorm:"column:reviewer_runs" json:"reviewer_runs"`
	MaxToolCalls          int       `gorm:"column:max_tool_calls" json:"max_tool_calls"`
	MaxDocumentReadTokens int       `gorm:"column:max_document_read_tokens" json:"max_document_read_tokens"`
	MaxToolResultBytes    int       `gorm:"column:max_tool_result_bytes" json:"max_tool_result_bytes"`
	MaxRunSeconds         int       `gorm:"column:max_run_seconds" json:"max_run_seconds"`
	NetworkEnabled        bool      `gorm:"column:network_enabled" json:"network_enabled"`
	MemoryEnabled         bool      `gorm:"column:memory_enabled" json:"memory_enabled"`
	MemoryTopK            int       `gorm:"column:memory_top_k" json:"memory_top_k"`
	ShowExecutionStatus   bool      `gorm:"column:show_execution_status" json:"show_execution_status"`
	Status                string    `gorm:"column:status" json:"status"`
	CreatedAt             time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 返回 Agent 配置实体对应的数据库表名。
func (AgentConfig) TableName() string { return "agent_configs" }
