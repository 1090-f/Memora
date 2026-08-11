package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
)

// PlanStateStore 定义 Plan 状态管理的接口。
type PlanStateStore interface {
	// Save 保存 Plan 到数据库。
	Save(ctx context.Context, plan contracts.Plan, runID contracts.ID, userID contracts.ID, knowledgeBaseID contracts.ID) (contracts.ID, error)
	// Get 根据 ID 获取 Plan。
	Get(ctx context.Context, planID contracts.ID) (contracts.Plan, error)
	// GetByRunID 根据 RunID 获取 Plan。
	GetByRunID(ctx context.Context, runID contracts.ID) (contracts.Plan, error)
	// UpdateStatus 更新 Plan 状态。
	UpdateStatus(ctx context.Context, planID contracts.ID, status contracts.PlanStatus) error
	// UpdateStepStatus 更新步骤状态。
	UpdateStepStatus(ctx context.Context, planID contracts.ID, stepNo int, status contracts.PlanStepStatus) error
	// RecordStepResult 记录步骤执行结果。
	RecordStepResult(ctx context.Context, planID contracts.ID, stepNo int, inputSummary string, outputSummary string, errorCode string, errorMessage string) error
	// RecordStepError 记录步骤错误。
	RecordStepError(ctx context.Context, planID contracts.ID, stepNo int, err error) error
	// RecordReview 记录审查结果。
	RecordReview(ctx context.Context, planID contracts.ID, review contracts.ReviewerResult) error
	// GetExecutionHistory 获取执行历史。
	GetExecutionHistory(ctx context.Context, planID contracts.ID) ([]ExecutionLog, error)
}

// ExecutionLog 表示执行日志。
type ExecutionLog struct {
	ID        string    `json:"id"`
	StepNo    *int      `json:"step_no,omitempty"`
	EventType string    `json:"event_type"`
	OldStatus string    `json:"old_status,omitempty"`
	NewStatus string    `json:"new_status,omitempty"`
	Message   string    `json:"message,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// planStateStore 是 PlanStateStore 的实现。
type planStateStore struct {
	planRepo repository.PlanRepository
}

// NewPlanStateStore 创建 PlanStateStore 实例。
func NewPlanStateStore(planRepo repository.PlanRepository) PlanStateStore {
	return &planStateStore{planRepo: planRepo}
}

// Save 保存 Plan 到数据库。
func (s *planStateStore) Save(ctx context.Context, plan contracts.Plan, runID contracts.ID, userID contracts.ID, knowledgeBaseID contracts.ID) (contracts.ID, error) {
	// 转换 CompletionCriteria
	completionCriteria, err := json.Marshal(plan.CompletionCriteria)
	if err != nil {
		return "", fmt.Errorf("marshal completion criteria: %w", err)
	}

	// 创建 Plan 实体
	agentPlan := entity.AgentPlan{
		ID:                 uuid.New(),
		AgentRunID:         uuid.MustParse(string(runID)),
		Version:            plan.Version,
		Goal:               plan.Goal,
		CompletionCriteria: datatypes.JSON(completionCriteria),
		Status:             string(plan.Status),
		IsCurrent:          true,
	}

	// 创建 Steps
	steps := make([]entity.AgentPlanStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		dependsOn, err := json.Marshal(step.DependsOn)
		if err != nil {
			return "", fmt.Errorf("marshal depends_on: %w", err)
		}

		agentStep := entity.AgentPlanStep{
			ID:                 uuid.New(),
			StepNo:             step.StepNo,
			Title:              step.Title,
			Description:        step.Description,
			DependsOn:          datatypes.JSON(dependsOn),
			RecommendedTool:    step.ToolHint,
			CompletionCriteria: step.CompletionCriteria,
			Status:             string(step.Status),
		}
		steps = append(steps, agentStep)
	}
	agentPlan.Steps = steps

	// 保存到数据库
	if err := s.planRepo.Create(ctx, &agentPlan); err != nil {
		return "", fmt.Errorf("create plan: %w", err)
	}

	// 记录创建日志
	s.createLog(ctx, agentPlan.ID, nil, "", "", "plan_created", fmt.Sprintf("Plan created with %d steps", len(plan.Steps)))

	logger.Info("Plan saved",
		zap.String("plan_id", agentPlan.ID.String()),
		zap.String("run_id", string(runID)),
		zap.Int("steps", len(plan.Steps)),
	)

	return contracts.ID(agentPlan.ID.String()), nil
}

// Get 根据 ID 获取 Plan。
func (s *planStateStore) Get(ctx context.Context, planID contracts.ID) (contracts.Plan, error) {
	agentPlan, err := s.planRepo.GetByID(ctx, uuid.MustParse(string(planID)))
	if err != nil {
		return contracts.Plan{}, fmt.Errorf("get plan: %w", err)
	}

	return s.convertToContract(agentPlan), nil
}

// GetByRunID 根据 RunID 获取 Plan。
func (s *planStateStore) GetByRunID(ctx context.Context, runID contracts.ID) (contracts.Plan, error) {
	agentPlan, err := s.planRepo.GetByRunID(ctx, uuid.MustParse(string(runID)))
	if err != nil {
		return contracts.Plan{}, fmt.Errorf("get plan by run id: %w", err)
	}

	return s.convertToContract(agentPlan), nil
}

// UpdateStatus 更新 Plan 状态。
func (s *planStateStore) UpdateStatus(ctx context.Context, planID contracts.ID, status contracts.PlanStatus) error {
	// 获取旧状态
	agentPlan, err := s.planRepo.GetByID(ctx, uuid.MustParse(string(planID)))
	if err != nil {
		return fmt.Errorf("get plan: %w", err)
	}
	oldStatus := agentPlan.Status

	// 更新状态
	if err := s.planRepo.UpdateStatus(ctx, uuid.MustParse(string(planID)), string(status)); err != nil {
		return fmt.Errorf("update plan status: %w", err)
	}

	// 记录日志
	s.createLog(ctx, uuid.MustParse(string(planID)), nil, oldStatus, string(status), "status_changed", fmt.Sprintf("Status changed from %s to %s", oldStatus, status))

	logger.Info("Plan status updated",
		zap.String("plan_id", string(planID)),
		zap.String("old_status", oldStatus),
		zap.String("new_status", string(status)),
	)

	return nil
}

// UpdateStepStatus 更新步骤状态。
func (s *planStateStore) UpdateStepStatus(ctx context.Context, planID contracts.ID, stepNo int, status contracts.PlanStepStatus) error {
	// 获取旧状态
	agentPlan, err := s.planRepo.GetByID(ctx, uuid.MustParse(string(planID)))
	if err != nil {
		return fmt.Errorf("get plan: %w", err)
	}

	var oldStatus string
	for _, step := range agentPlan.Steps {
		if step.StepNo == stepNo {
			oldStatus = step.Status
			break
		}
	}

	// 更新状态
	if err := s.planRepo.UpdateStepStatus(ctx, uuid.MustParse(string(planID)), stepNo, string(status)); err != nil {
		return fmt.Errorf("update step status: %w", err)
	}

	// 记录日志
	s.createLog(ctx, uuid.MustParse(string(planID)), &stepNo, oldStatus, string(status), "step_status_changed", fmt.Sprintf("Step %d status changed from %s to %s", stepNo, oldStatus, status))

	logger.Info("Step status updated",
		zap.String("plan_id", string(planID)),
		zap.Int("step_no", stepNo),
		zap.String("old_status", oldStatus),
		zap.String("new_status", string(status)),
	)

	return nil
}

// RecordStepResult 记录步骤执行结果。
func (s *planStateStore) RecordStepResult(ctx context.Context, planID contracts.ID, stepNo int, inputSummary string, outputSummary string, errorCode string, errorMessage string) error {
	if err := s.planRepo.UpdateStepResult(ctx, uuid.MustParse(string(planID)), stepNo, inputSummary, outputSummary, errorCode, errorMessage); err != nil {
		return fmt.Errorf("record step result: %w", err)
	}

	// 记录日志
	s.createLog(ctx, uuid.MustParse(string(planID)), &stepNo, "", "completed", "step_completed", fmt.Sprintf("Step %d completed", stepNo))

	logger.Info("Step result recorded",
		zap.String("plan_id", string(planID)),
		zap.Int("step_no", stepNo),
	)

	return nil
}

// RecordStepError 记录步骤错误。
func (s *planStateStore) RecordStepError(ctx context.Context, planID contracts.ID, stepNo int, err error) error {
	if updateErr := s.planRepo.UpdateStepError(ctx, uuid.MustParse(string(planID)), stepNo, "INTERNAL_ERROR", err.Error()); updateErr != nil {
		return fmt.Errorf("record step error: %w", updateErr)
	}

	// 记录日志
	s.createLog(ctx, uuid.MustParse(string(planID)), &stepNo, "", "failed", "step_failed", fmt.Sprintf("Step %d failed: %s", stepNo, err.Error()))

	logger.Error("Step error recorded",
		zap.String("plan_id", string(planID)),
		zap.Int("step_no", stepNo),
		zap.Error(err),
	)

	return nil
}

// RecordReview 记录审查结果。
func (s *planStateStore) RecordReview(ctx context.Context, planID contracts.ID, review contracts.ReviewerResult) error {
	agentPlan, err := s.planRepo.GetByID(ctx, uuid.MustParse(string(planID)))
	if err != nil {
		return fmt.Errorf("get plan: %w", err)
	}

	// 更新计划状态
	if review.Result == "completed" {
		agentPlan.Status = string(contracts.PlanCompleted)
		now := time.Now()
		agentPlan.CompletedAt = &now
	} else if review.Result == "failed" {
		agentPlan.Status = string(contracts.PlanFailed)
	}

	if err := s.planRepo.Update(ctx, agentPlan); err != nil {
		return fmt.Errorf("record review: %w", err)
	}

	// 记录日志
	s.createLog(ctx, uuid.MustParse(string(planID)), nil, "", "", "review_recorded", fmt.Sprintf("Review recorded: %s", review.Result))

	logger.Info("Review recorded",
		zap.String("plan_id", string(planID)),
		zap.String("result", review.Result),
	)

	return nil
}

// GetExecutionHistory 获取执行历史。
func (s *planStateStore) GetExecutionHistory(ctx context.Context, planID contracts.ID) ([]ExecutionLog, error) {
	logs, err := s.planRepo.GetExecutionLogs(ctx, uuid.MustParse(string(planID)))
	if err != nil {
		return nil, fmt.Errorf("get execution logs: %w", err)
	}

	result := make([]ExecutionLog, len(logs))
	for i, log := range logs {
		result[i] = ExecutionLog{
			ID:        log.ID.String(),
			StepNo:    log.StepNo,
			EventType: log.EventType,
			OldStatus: log.OldStatus,
			NewStatus: log.NewStatus,
			Message:   log.Message,
			CreatedAt: log.CreatedAt,
		}
	}

	return result, nil
}

// createLog 创建执行日志。
func (s *planStateStore) createLog(ctx context.Context, planID uuid.UUID, stepNo *int, oldStatus, newStatus, eventType, message string) {
	log := entity.AgentPlanExecutionLog{
		ID:        uuid.New(),
		PlanID:    planID,
		StepNo:    stepNo,
		EventType: eventType,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Message:   message,
	}

	if err := s.planRepo.CreateExecutionLog(ctx, &log); err != nil {
		logger.Error("Failed to create execution log",
			zap.Error(err),
			zap.String("plan_id", planID.String()),
			zap.String("event_type", eventType),
		)
	}
}

// convertToContract 将 Entity 转换为 Contract。
func (s *planStateStore) convertToContract(agentPlan *entity.AgentPlan) contracts.Plan {
	// 转换 CompletionCriteria
	var completionCriteria []string
	if err := json.Unmarshal(agentPlan.CompletionCriteria, &completionCriteria); err != nil {
		completionCriteria = []string{}
	}

	// 转换 Steps
	steps := make([]contracts.PlanStep, len(agentPlan.Steps))
	for i, step := range agentPlan.Steps {
		var dependsOn []int
		if err := json.Unmarshal(step.DependsOn, &dependsOn); err != nil {
			dependsOn = []int{}
		}

		steps[i] = contracts.PlanStep{
			ID:                 contracts.ID(step.ID.String()),
			StepNo:             step.StepNo,
			Title:              step.Title,
			Description:        step.Description,
			DependsOn:          dependsOn,
			ToolHint:           step.RecommendedTool,
			CompletionCriteria: step.CompletionCriteria,
			Status:             contracts.PlanStepStatus(step.Status),
		}
	}

	return contracts.Plan{
		ID:                 contracts.ID(agentPlan.ID.String()),
		RunID:              contracts.ID(agentPlan.AgentRunID.String()),
		Version:            agentPlan.Version,
		Goal:               agentPlan.Goal,
		CompletionCriteria: completionCriteria,
		Status:             contracts.PlanStatus(agentPlan.Status),
		Steps:              steps,
	}
}
