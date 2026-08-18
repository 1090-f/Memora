package repository

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlanRepository 定义计划数据访问接口。
type PlanRepository interface {
	// Create 创建计划。
	Create(ctx context.Context, plan *entity.AgentPlan) error
	// FindByID 根据 ID 查找计划。
	FindByID(ctx context.Context, planID uuid.UUID) (*entity.AgentPlan, error)
	// FindByRunID 根据运行 ID 查找计划。
	FindByRunID(ctx context.Context, runID uuid.UUID) (*entity.AgentPlan, error)
	// Update 更新计划。
	Update(ctx context.Context, plan *entity.AgentPlan) error
	// Delete 删除计划。
	Delete(ctx context.Context, planID uuid.UUID) error

	// CreateStep 创建计划步骤。
	CreateStep(ctx context.Context, step *entity.AgentPlanStep) error
	// FindStepsByPlanID 查找计划的所有步骤。
	FindStepsByPlanID(ctx context.Context, planID uuid.UUID) ([]entity.AgentPlanStep, error)
	// UpdateStep 更新步骤。
	UpdateStep(ctx context.Context, step *entity.AgentPlanStep) error
	// DeleteStepsByPlanID 删除计划的所有步骤。
	DeleteStepsByPlanID(ctx context.Context, planID uuid.UUID) error

	// CreateExecutionLog 创建执行日志。
	CreateExecutionLog(ctx context.Context, log *entity.AgentPlanExecutionLog) error
	// FindExecutionLogsByPlanID 查找计划的所有执行日志。
	FindExecutionLogsByPlanID(ctx context.Context, planID uuid.UUID) ([]entity.AgentPlanExecutionLog, error)
}

// planRepository 是 PlanRepository 接口的 GORM 实现。
type planRepository struct {
	db *gorm.DB
}

// NewPlanRepository 创建 PlanRepository 实例。
func NewPlanRepository(db *gorm.DB) PlanRepository {
	return &planRepository{db: db}
}

// Create 创建计划。
func (r *planRepository) Create(ctx context.Context, plan *entity.AgentPlan) error {
	if err := r.db.WithContext(ctx).Create(plan).Error; err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	return nil
}

// FindByID 根据 ID 查找计划。
func (r *planRepository) FindByID(ctx context.Context, planID uuid.UUID) (*entity.AgentPlan, error) {
	var plan entity.AgentPlan
	err := r.db.WithContext(ctx).
		Where("id = ?", planID).
		First(&plan).Error
	if err != nil {
		return nil, fmt.Errorf("find plan by id: %w", err)
	}
	return &plan, nil
}

// FindByRunID 根据运行 ID 查找计划。
func (r *planRepository) FindByRunID(ctx context.Context, runID uuid.UUID) (*entity.AgentPlan, error) {
	var plan entity.AgentPlan
	err := r.db.WithContext(ctx).
		Where("agent_run_id = ?", runID).
		Order("created_at DESC").
		First(&plan).Error
	if err != nil {
		return nil, fmt.Errorf("find plan by run id: %w", err)
	}
	return &plan, nil
}

// Update 更新计划。
func (r *planRepository) Update(ctx context.Context, plan *entity.AgentPlan) error {
	if err := r.db.WithContext(ctx).Save(plan).Error; err != nil {
		return fmt.Errorf("update plan: %w", err)
	}
	return nil
}

// Delete 删除计划。
func (r *planRepository) Delete(ctx context.Context, planID uuid.UUID) error {
	// 先删除步骤
	if err := r.DeleteStepsByPlanID(ctx, planID); err != nil {
		return fmt.Errorf("delete plan steps: %w", err)
	}
	// 再删除计划
	if err := r.db.WithContext(ctx).Delete(&entity.AgentPlan{}, planID).Error; err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	return nil
}

// CreateStep 创建计划步骤。
func (r *planRepository) CreateStep(ctx context.Context, step *entity.AgentPlanStep) error {
	if err := r.db.WithContext(ctx).Create(step).Error; err != nil {
		return fmt.Errorf("create plan step: %w", err)
	}
	return nil
}

// FindStepsByPlanID 查找计划的所有步骤。
func (r *planRepository) FindStepsByPlanID(ctx context.Context, planID uuid.UUID) ([]entity.AgentPlanStep, error) {
	var steps []entity.AgentPlanStep
	err := r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Order("step_number ASC").
		Find(&steps).Error
	if err != nil {
		return nil, fmt.Errorf("find plan steps: %w", err)
	}
	return steps, nil
}

// UpdateStep 更新步骤。
func (r *planRepository) UpdateStep(ctx context.Context, step *entity.AgentPlanStep) error {
	if err := r.db.WithContext(ctx).Save(step).Error; err != nil {
		return fmt.Errorf("update plan step: %w", err)
	}
	return nil
}

// DeleteStepsByPlanID 删除计划的所有步骤。
func (r *planRepository) DeleteStepsByPlanID(ctx context.Context, planID uuid.UUID) error {
	if err := r.db.WithContext(ctx).Where("plan_id = ?", planID).Delete(&entity.AgentPlanStep{}).Error; err != nil {
		return fmt.Errorf("delete plan steps: %w", err)
	}
	return nil
}

// CreateExecutionLog 创建执行日志。
func (r *planRepository) CreateExecutionLog(ctx context.Context, log *entity.AgentPlanExecutionLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("create execution log: %w", err)
	}
	return nil
}

// FindExecutionLogsByPlanID 查找计划的所有执行日志。
func (r *planRepository) FindExecutionLogsByPlanID(ctx context.Context, planID uuid.UUID) ([]entity.AgentPlanExecutionLog, error) {
	var logs []entity.AgentPlanExecutionLog
	err := r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Order("created_at ASC").
		Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("find execution logs: %w", err)
	}
	return logs, nil
}
