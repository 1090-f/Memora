package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentPlan 表示 Agent 执行计划的数据库实体。
// 与数据库 agent_plans 表结构保持一致。
type AgentPlan struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AgentRunID         uuid.UUID      `gorm:"column:agent_run_id;type:uuid;not null;index" json:"agent_run_id"`
	Version            int            `gorm:"not null;check:version BETWEEN 1 AND 2" json:"version"`
	Goal               string         `gorm:"type:text;not null" json:"goal"`
	CompletionCriteria datatypes.JSON `gorm:"type:jsonb" json:"completion_criteria"`
	Status             string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	IsCurrent          bool           `gorm:"not null;default:true" json:"is_current"`
	ReplanReason       *string        `gorm:"type:text" json:"replan_reason"`
	CreatedAt          time.Time      `gorm:"autoCreateTime" json:"created_at"`
	CompletedAt        *time.Time     `json:"completed_at"`
}

// TableName 指定 agent_plans 表名。
func (AgentPlan) TableName() string {
	return "agent_plans"
}

// AgentPlanStep 表示计划步骤的数据库实体。
// 与数据库 agent_plan_steps 表结构保持一致。
type AgentPlanStep struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID             uuid.UUID      `gorm:"type:uuid;not null;index" json:"plan_id"`
	StepNo             int            `gorm:"not null;check:step_no BETWEEN 1 AND 5" json:"step_no"`
	Title              string         `gorm:"type:varchar(255);not null" json:"title"`
	Description        string         `gorm:"type:text" json:"description"`
	DependsOn          datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"depends_on"`
	RecommendedTool    string         `gorm:"type:varchar(128)" json:"recommended_tool"`
	CompletionCriteria string         `gorm:"type:text" json:"completion_criteria"`
	Status             string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	InputSummary       string         `gorm:"type:text" json:"input_summary"`
	OutputSummary      string         `gorm:"type:text" json:"output_summary"`
	ErrorCode          string         `gorm:"type:varchar(64)" json:"error_code"`
	ErrorMessage       string         `gorm:"type:text" json:"error_message"`
	StartedAt          *time.Time     `json:"started_at"`
	EndedAt            *time.Time     `json:"ended_at"`
	CreatedAt          time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定 agent_plan_steps 表名。
func (AgentPlanStep) TableName() string {
	return "agent_plan_steps"
}

// AgentPlanExecutionLog 表示计划执行日志的数据库实体。
// 与数据库 agent_plan_execution_logs 表结构保持一致。
type AgentPlanExecutionLog struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"plan_id"`
	StepNo    *int           `json:"step_no"`
	EventType string         `gorm:"type:varchar(50);not null" json:"event_type"`
	OldStatus *string        `gorm:"type:varchar(20)" json:"old_status"`
	NewStatus *string        `gorm:"type:varchar(20)" json:"new_status"`
	Message   string         `gorm:"type:text" json:"message"`
	Metadata  datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定 agent_plan_execution_logs 表名。
func (AgentPlanExecutionLog) TableName() string {
	return "agent_plan_execution_logs"
}
