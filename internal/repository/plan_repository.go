package repository

import (
	"context"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlanRepository 定义 Plan 持久化的接口。
type PlanRepository interface {
	// Create 创建新的 Plan。
	Create(ctx context.Context, plan *entity.AgentPlan) error
	// GetByID 根据 ID 获取 Plan。
	GetByID(ctx context.Context, planID uuid.UUID) (*entity.AgentPlan, error)
	// GetByRunID 根据 RunID 获取 Plan。
	GetByRunID(ctx context.Context, runID uuid.UUID) (*entity.AgentPlan, error)
	// Update 更新 Plan。
	Update(ctx context.Context, plan *entity.AgentPlan) error
	// UpdateStatus 更新 Plan 状态。
	UpdateStatus(ctx context.Context, planID uuid.UUID, status string) error
	// UpdateStepStatus 更新步骤状态。
	UpdateStepStatus(ctx context.Context, planID uuid.UUID, stepNo int, status string) error
	// UpdateStepResult 更新步骤执行结果。
	UpdateStepResult(ctx context.Context, planID uuid.UUID, stepNo int, inputSummary string, outputSummary string, errorCode string, errorMessage string) error
	// UpdateStepError 更新步骤错误信息。
	UpdateStepError(ctx context.Context, planID uuid.UUID, stepNo int, errorCode string, errorMessage string) error
	// CreateExecutionLog 创建执行日志。
	CreateExecutionLog(ctx context.Context, log *entity.AgentPlanExecutionLog) error
	// GetExecutionLogs 获取执行日志。
	GetExecutionLogs(ctx context.Context, planID uuid.UUID) ([]entity.AgentPlanExecutionLog, error)
}

// planRepository 是 PlanRepository 的 GORM 实现。
type planRepository struct {
	db *gorm.DB
}

// NewPlanRepository 创建 PlanRepository 实例。
func NewPlanRepository(db *gorm.DB) PlanRepository {
	return &planRepository{db: db}
}

// Create 创建新的 Plan。
func (r *planRepository) Create(ctx context.Context, plan *entity.AgentPlan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

// GetByID 根据 ID 获取 Plan。
func (r *planRepository) GetByID(ctx context.Context, planID uuid.UUID) (*entity.AgentPlan, error) {
	var plan entity.AgentPlan
	err := r.db.WithContext(ctx).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_no ASC")
		}).
		Where("id = ?", planID).
		First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// GetByRunID 根据 RunID 获取 Plan。
func (r *planRepository) GetByRunID(ctx context.Context, runID uuid.UUID) (*entity.AgentPlan, error) {
	var plan entity.AgentPlan
	err := r.db.WithContext(ctx).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_no ASC")
		}).
		Where("agent_run_id = ?", runID).
		Order("version DESC").
		First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// Update 更新 Plan。
func (r *planRepository) Update(ctx context.Context, plan *entity.AgentPlan) error {
	return r.db.WithContext(ctx).Save(plan).Error
}

// UpdateStatus 更新 Plan 状态。
func (r *planRepository) UpdateStatus(ctx context.Context, planID uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&entity.AgentPlan{}).
		Where("id = ?", planID).
		Update("status", status).
		Error
}

// UpdateStepStatus 更新步骤状态。
func (r *planRepository) UpdateStepStatus(ctx context.Context, planID uuid.UUID, stepNo int, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// 根据状态设置时间戳
	now := time.Now()
	switch status {
	case "running":
		updates["started_at"] = now
	case "completed", "failed", "skipped", "cancelled":
		updates["ended_at"] = now
	}

	return r.db.WithContext(ctx).
		Model(&entity.AgentPlanStep{}).
		Where("plan_id = ? AND step_no = ?", planID, stepNo).
		Updates(updates).
		Error
}

// UpdateStepResult 更新步骤执行结果。
func (r *planRepository) UpdateStepResult(ctx context.Context, planID uuid.UUID, stepNo int, inputSummary string, outputSummary string, errorCode string, errorMessage string) error {
	return r.db.WithContext(ctx).
		Model(&entity.AgentPlanStep{}).
		Where("plan_id = ? AND step_no = ?", planID, stepNo).
		Updates(map[string]interface{}{
			"input_summary":  inputSummary,
			"output_summary": outputSummary,
			"error_code":     errorCode,
			"error_message":  errorMessage,
		}).
		Error
}

// UpdateStepError 更新步骤错误信息。
func (r *planRepository) UpdateStepError(ctx context.Context, planID uuid.UUID, stepNo int, errorCode string, errorMessage string) error {
	return r.db.WithContext(ctx).
		Model(&entity.AgentPlanStep{}).
		Where("plan_id = ? AND step_no = ?", planID, stepNo).
		Updates(map[string]interface{}{
			"error_code":    errorCode,
			"error_message": errorMessage,
			"status":        "failed",
			"ended_at":      time.Now(),
		}).
		Error
}

// CreateExecutionLog 创建执行日志。
func (r *planRepository) CreateExecutionLog(ctx context.Context, log *entity.AgentPlanExecutionLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetExecutionLogs 获取执行日志。
func (r *planRepository) GetExecutionLogs(ctx context.Context, planID uuid.UUID) ([]entity.AgentPlanExecutionLog, error) {
	var logs []entity.AgentPlanExecutionLog
	err := r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}
