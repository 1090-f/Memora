package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentPlan 表示 Agent 计划的数据库实体。
// 对应 000005_conversation_agent 中的 agent_plans 表。
type AgentPlan struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AgentRunID         uuid.UUID      `gorm:"type:uuid;not null;column:agent_run_id" json:"agent_run_id"`
	Version            int            `gorm:"not null;check:version BETWEEN 1 AND 2" json:"version"`
	Goal               string         `gorm:"type:text;not null" json:"goal"`
	CompletionCriteria datatypes.JSON `gorm:"type:jsonb" json:"completion_criteria"`
	Status             string         `gorm:"type:varchar(20);not null;check:status IN ('pending', 'executing', 'replanning', 'reviewing', 'completed', 'failed', 'cancelled')" json:"status"`
	IsCurrent          bool           `gorm:"not null;default:true" json:"is_current"`
	ReplanReason       string         `gorm:"type:text" json:"replan_reason,omitempty"`
	CreatedAt          time.Time      `gorm:"autoCreateTime" json:"created_at"`
	CompletedAt        *time.Time     `json:"completed_at,omitempty"`

	// 关联
	Steps []AgentPlanStep `gorm:"foreignKey:PlanID" json:"steps,omitempty"`
}

// TableName 指定表名。
func (AgentPlan) TableName() string {
	return "agent_plans"
}

// AgentPlanStep 表示计划步骤的数据库实体。
// 对应 000005_conversation_agent 中的 agent_plan_steps 表。
type AgentPlanStep struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID             uuid.UUID      `gorm:"type:uuid;not null;index" json:"plan_id"`
	StepNo             int            `gorm:"not null;check:step_no BETWEEN 1 AND 5" json:"step_no"`
	Title              string         `gorm:"type:varchar(255);not null" json:"title"`
	Description        string         `gorm:"type:text" json:"description,omitempty"`
	DependsOn          datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'::jsonb" json:"depends_on"`
	RecommendedTool    string         `gorm:"type:varchar(128)" json:"recommended_tool,omitempty"`
	CompletionCriteria string         `gorm:"type:text" json:"completion_criteria,omitempty"`
	Status             string         `gorm:"type:varchar(20);not null;default:pending;check:status IN ('pending', 'running', 'completed', 'failed', 'skipped', 'cancelled')" json:"status"`
	InputSummary       string         `gorm:"type:text" json:"input_summary,omitempty"`
	OutputSummary      string         `gorm:"type:text" json:"output_summary,omitempty"`
	ErrorCode          string         `gorm:"type:varchar(64)" json:"error_code,omitempty"`
	ErrorMessage       string         `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt          *time.Time     `json:"started_at,omitempty"`
	EndedAt            *time.Time     `json:"ended_at,omitempty"`
	CreatedAt          time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名。
func (AgentPlanStep) TableName() string {
	return "agent_plan_steps"
}

// AgentPlanExecutionLog 表示计划执行日志的数据库实体。
// 对应 000014_agent_plans 中的 agent_plan_execution_logs 表。
type AgentPlanExecutionLog struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"plan_id"`
	StepNo    *int           `json:"step_no,omitempty"`
	EventType string         `gorm:"type:varchar(50);not null" json:"event_type"`
	OldStatus string         `gorm:"type:varchar(20)" json:"old_status,omitempty"`
	NewStatus string         `gorm:"type:varchar(20)" json:"new_status,omitempty"`
	Message   string         `gorm:"type:text" json:"message,omitempty"`
	Metadata  datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名。
func (AgentPlanExecutionLog) TableName() string {
	return "agent_plan_execution_logs"
}
