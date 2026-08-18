package entity

import (
	"time"

	"github.com/google/uuid"
)

// AgentPlan 表示 Agent 执行计划的数据库实体。
type AgentPlan struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	RunID       uuid.UUID `gorm:"column:agent_run_id;type:uuid;not null;index" json:"run_id"`
	Goal        string    `gorm:"type:text;not null" json:"goal"`
	MaxSteps    int       `gorm:"not null;default:5" json:"max_steps"`
	ReplanCount int       `gorm:"not null;default:0" json:"replan_count"`
	MaxReplans  int       `gorm:"not null;default:1" json:"max_replans"`
	Status      string    `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	FinalAnswer string    `gorm:"type:text" json:"final_answer"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定 agent_plans 表名。
func (AgentPlan) TableName() string {
	return "agent_plans"
}

// AgentPlanStep 表示计划步骤的数据库实体。
type AgentPlanStep struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"plan_id"`
	StepNumber  int        `gorm:"not null" json:"step_number"`
	Title       string     `gorm:"type:varchar(500);not null" json:"title"`
	Description string     `gorm:"type:text" json:"description"`
	ToolName    string     `gorm:"type:varchar(100)" json:"tool_name"`
	Arguments   string     `gorm:"type:jsonb" json:"arguments"`
	DependsOn   string     `gorm:"type:jsonb" json:"depends_on"`
	Status      string     `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Output      string     `gorm:"type:text" json:"output"`
	Error       string     `gorm:"type:text" json:"error"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定 agent_plan_steps 表名。
func (AgentPlanStep) TableName() string {
	return "agent_plan_steps"
}

// AgentPlanExecutionLog 表示计划执行日志的数据库实体。
type AgentPlanExecutionLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID    uuid.UUID `gorm:"type:uuid;not null;index" json:"plan_id"`
	StepID    uuid.UUID `gorm:"type:uuid" json:"step_id"`
	Action    string    `gorm:"type:varchar(50);not null" json:"action"`
	Details   string    `gorm:"type:jsonb" json:"details"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定 agent_plan_execution_logs 表名。
func (AgentPlanExecutionLog) TableName() string {
	return "agent_plan_execution_logs"
}
