// Package entity 定义与数据库表一一对应的持久化实体。
package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ToolCall 表示 Agent 执行过程中单次工具调用的数据库实体。
// 对应 000005_conversation_agent 中的 tool_calls 表，
// 记录 KnowledgeSearchTool、DocumentReadTool 和 MCP 工具的所有调用详情。
type ToolCall struct {
	ID                uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AgentRunID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"agent_run_id"`                                                                   // 所属 Agent 运行 ID
	PlanStepID        *uuid.UUID     `gorm:"type:uuid" json:"plan_step_id,omitempty"`                                                                        // 可选：所属计划步骤 ID（Plan-Execute 模式下）
	ReactRoundNo      *int           `gorm:"check:react_round_no > 0" json:"react_round_no,omitempty"`                                                       // 可选：ReAct 执行轮次编号
	ToolName          string         `gorm:"type:varchar(128);not null" json:"tool_name"`                                                                    // 工具名称（如 knowledge_search、document_read、mcp 工具名）
	ToolType          string         `gorm:"type:varchar(20);not null;check:tool_type IN ('internal','mcp')" json:"tool_type"`                               // 工具类型：internal（内置工具）/ mcp
	MCPServerID       *uuid.UUID     `gorm:"type:uuid" json:"mcp_server_id,omitempty"`                                                                       // 可选：MCP Server ID（仅 MCP 工具）
	MCPToolID         *uuid.UUID     `gorm:"type:uuid" json:"mcp_tool_id,omitempty"`                                                                         // 可选：MCP 工具 ID（仅 MCP 工具）
	Status            string         `gorm:"type:varchar(20);not null;check:status IN ('running','succeeded','failed','timeout','cancelled')" json:"status"` // 调用状态
	ArgumentsRedacted datatypes.JSON `gorm:"type:jsonb" json:"arguments_redacted,omitempty"`                                                                 // 工具参数的脱敏快照（不保存原始完整参数）
	InputSummary      string         `gorm:"type:text" json:"input_summary,omitempty"`                                                                       // 输入参数摘要（面向展示和审计）
	OutputSummary     string         `gorm:"type:text" json:"output_summary,omitempty"`                                                                      // 输出结果摘要（面向展示和审计）
	ResultMeta        datatypes.JSON `gorm:"type:jsonb" json:"result_meta,omitempty"`                                                                        // 结果元数据（如返回条数、截断标志等结构化信息）
	ResponseBytes     *int64         `gorm:"check:response_bytes >= 0" json:"response_bytes,omitempty"`                                                      // 原始响应字节数（用于审计和容量规划）
	IsTruncated       bool           `gorm:"not null;default:false" json:"is_truncated"`                                                                     // 结果是否因超限被截断
	ErrorCode         *string        `gorm:"type:varchar(64)" json:"error_code,omitempty"`                                                                   // 失败时的错误码
	ErrorMessage      *string        `gorm:"type:text" json:"error_message,omitempty"`                                                                       // 失败时的错误信息
	DurationMs        *int64         `gorm:"check:duration_ms >= 0" json:"duration_ms,omitempty"`                                                            // 调用耗时（毫秒）
	StartedAt         time.Time      `gorm:"not null;default:now()" json:"started_at"`                                                                       // 调用开始时间
	EndedAt           *time.Time     `json:"ended_at,omitempty"`                                                                                             // 调用结束时间
}

// TableName 指定 tool_calls 表名。
func (ToolCall) TableName() string {
	return "tool_calls"
}
