// Package agent 提供 Agent 运行管理的 HTTP API。
// 包括创建智能问答、查询运行状态、取消和重试等端点。
package agent

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/1090-f/Memora/internal/agent/core"
	"github.com/1090-f/Memora/internal/api/response"
	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/middleware"
	"github.com/1090-f/Memora/internal/model/dto/request"
	respdto "github.com/1090-f/Memora/internal/model/dto/response"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Controller 处理 Agent 运行相关的 HTTP 请求。
type Controller struct {
	agentService   contracts.AgentRunService     // Agent 核心执行服务（包含 Run/Cancel/Retry）
	runRepo        repository.AgentRunRepository // 运行记录的持久化查询
	toolCallRepo   repository.ToolCallRepository // 工具调用的持久化查询
	contextBuilder contracts.ContextBuilder      // 上下文构建器（组装 AgentContext）
}

// NewController 创建 Agent 运行管理的 HTTP 控制器实例。
func NewController(
	agentService contracts.AgentRunService,
	runRepo repository.AgentRunRepository,
	toolCallRepo repository.ToolCallRepository,
	contextBuilder contracts.ContextBuilder,
) *Controller {
	return &Controller{
		agentService:   agentService,
		runRepo:        runRepo,
		toolCallRepo:   toolCallRepo,
		contextBuilder: contextBuilder,
	}
}

// CreateRun 处理 POST /api/v1/agent/runs，创建智能问答任务并排队执行。
// 流程：构建上下文 → 创建排队运行 → 后台异步执行 → 立即返回 run_id。
func (ctrl *Controller) CreateRun(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	var req request.CreateAgentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	// 生成运行 ID，用于后续构建上下文和执行
	runUUID, err := uuid.NewV7()
	if err != nil {
		response.Failure(c, apperrors.ErrInternal)
		return
	}
	runID := contracts.ID(runUUID.String())

	// 1. 构建 Agent 执行上下文（含 AgentConfig、会话历史、记忆、工具白名单等）
	agentCtx, err := ctrl.contextBuilder.Build(c.Request.Context(), contracts.AgentContextRequest{
		UserID:          contracts.ID(user.ID),
		KnowledgeBaseID: contracts.ID(req.KnowledgeBaseID),
		ConversationID:  contracts.ID(req.ConversationID),
		RunID:           runID,
		Query:           req.Query,
	})
	if err != nil {
		response.Failure(c, mapAgentError(err))
		return
	}

	// 2. 构建运行请求，使用 AgentContext 中加载的配置作为基础
	runRequest := contracts.AgentRunRequest{
		RunID:   runID,
		Context: agentCtx,
		Config:  buildConfigFromContext(agentCtx),
	}

	// 3. 在后台 goroutine 中异步执行 Agent 运行
	// HTTP 请求不等待 Agent 完成，run_id 和初始状态立即返回给客户端
	go func() {
		// 创建独立上下文，避免 HTTP 请求结束后取消传播到正在执行的 Agent
		execCtx, cancel := context.WithTimeout(context.Background(), time.Duration(runRequest.Config.MaxRunSeconds)*time.Second)
		defer cancel()

		_, runErr := ctrl.agentService.Run(execCtx, runRequest)
		if runErr != nil {
			// 执行失败或取消，错误已通过 EventPublisher 发布
			return
		}
		// 执行成功，结果已落库和通过 EventPublisher 发布
	}()

	// 4. 立即返回排队成功响应
	response.Success(c, http.StatusAccepted, respdto.CreateAgentRunResponse{
		RunID:          string(runID),
		ConversationID: req.ConversationID,
		Status:         "queued",
	})
}

// GetRun 处理 GET /api/v1/agent/runs/:id，获取单次运行的详情。
func (ctrl *Controller) GetRun(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	userID, err := uuid.Parse(string(user.ID))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	run, err := ctrl.runRepo.FindByID(c.Request.Context(), userID, runID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Failure(c, apperrors.ErrNotFound)
			return
		}
		response.Failure(c, apperrors.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, toRunResponse(run))
}

// ListRuns 处理 GET /api/v1/agent/runs，按用户和知识库分页查询运行记录。
func (ctrl *Controller) ListRuns(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	kbID := c.Query("knowledge_base_id")
	if kbID == "" {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	userID, err := uuid.Parse(string(user.ID))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	kbUUID, err := uuid.Parse(kbID)
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}

	runs, total, err := ctrl.runRepo.ListByOwner(c.Request.Context(), userID, kbUUID, page, pageSize)
	if err != nil {
		response.Failure(c, apperrors.ErrInternal)
		return
	}

	items := make([]*respdto.AgentRunListItem, 0, len(runs))
	for i := range runs {
		items = append(items, toRunListItem(&runs[i]))
	}

	response.Success(c, http.StatusOK, respdto.AgentRunList{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

// CancelRun 处理 POST /api/v1/agent/runs/:id/cancel，取消指定的运行。
func (ctrl *Controller) CancelRun(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	if err := ctrl.agentService.Cancel(c.Request.Context(), contracts.ID(c.Param("id")), contracts.ID(user.ID)); err != nil {
		response.Failure(c, mapAgentError(err))
		return
	}
	response.Success(c, http.StatusOK, gin.H{"cancelled": true})
}

// RetryRun 处理 POST /api/v1/agent/runs/:id/retry，基于失败运行创建新的排队运行。
func (ctrl *Controller) RetryRun(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}

	newRunID, err := ctrl.agentService.Retry(c.Request.Context(), contracts.ID(c.Param("id")), contracts.ID(user.ID))
	if err != nil {
		response.Failure(c, mapAgentError(err))
		return
	}

	response.Success(c, http.StatusOK, respdto.RetryAgentRunResponse{
		NewRunID: string(newRunID),
		Status:   "queued",
	})
}

// 以下为内部辅助函数

// ErrNotFound 是用于判断 GORM 记录未找到的哨兵错误。
var ErrNotFound = errors.New("record not found")

// buildConfigFromContext 从 AgentContext 构建 AgentConfig。
// 将 AgentContext 中的配置参数映射为运行配置，超出 AgentContext 的字段使用默认值。
func buildConfigFromContext(ctx contracts.AgentContext) contracts.AgentConfig {
	config := contracts.DefaultAgentConfig()

	// 覆盖 ReAct 模式最大轮次（若上下文中有显式配置）
	if ctx.MaxReactRounds > 0 {
		config.MaxReactRounds = ctx.MaxReactRounds
	}

	// 覆盖 Plan-Execute 模式最大计划步数（若上下文中有显式配置）
	if ctx.MaxPlanSteps > 0 {
		config.MaxPlanSteps = ctx.MaxPlanSteps
	}

	return config
}

// mapAgentError 将 Agent 层的错误映射为统一 API 错误。
func mapAgentError(err error) error {
	if err == nil {
		return nil
	}
	var coreErr *core.CoreError
	if errors.As(err, &coreErr) {
		switch coreErr.Code {
		case contracts.ErrResourceNotFound, contracts.ErrInvalidArgument:
			return apperrors.New(coreErr.Code, coreErr)
		case contracts.ErrInvalidState, contracts.ErrRateLimited:
			return apperrors.New(coreErr.Code, coreErr)
		default:
			return apperrors.ErrInternal
		}
	}
	return apperrors.ErrInternal
}

// toRunResponse 将 AgentRun 实体转换为 API 响应 DTO。
func toRunResponse(run *entity.AgentRun) *respdto.AgentRunResponse {
	resp := &respdto.AgentRunResponse{
		ID:              run.ID.String(),
		UserID:          run.UserID.String(),
		KnowledgeBaseID: run.KnowledgeBaseID.String(),
		ConversationID:  run.ConversationID.String(),
		AgentConfigID:   run.AgentConfigID.String(),
		Query:           run.Query,
		RouterReason:    run.RouterReasonSummary,
		Status:          run.Status,
		ReplanCount:     run.ReplanCount,
		MemoryUsedCount: run.MemoryUsedCount,
		InputTokens:     run.InputTokens,
		OutputTokens:    run.OutputTokens,
		TotalTokens:     run.TotalTokens,
		DurationMs:      run.DurationMs,
		StartedAt:       run.StartedAt,
		EndedAt:         run.EndedAt,
		CreatedAt:       run.CreatedAt,
	}

	if run.RetryOfRunID != nil {
		idStr := run.RetryOfRunID.String()
		resp.RetryOfRunID = &idStr
	}
	if run.ExecutionMode != nil {
		mode := *run.ExecutionMode
		resp.ExecutionMode = &mode
	}
	if run.KnowledgeStatus != nil {
		resp.KnowledgeStatus = run.KnowledgeStatus
	}
	if run.ReviewerResult != nil {
		resp.ReviewerResult = run.ReviewerResult
	}
	if run.FinalResult != nil {
		resp.FinalResult = run.FinalResult
	}
	if run.ErrorCode != nil {
		resp.ErrorCode = run.ErrorCode
	}
	if run.ErrorMessage != nil {
		resp.ErrorMessage = run.ErrorMessage
	}

	return resp
}

// toRunListItem 将 AgentRun 实体转换为列表项 DTO。
func toRunListItem(run *entity.AgentRun) *respdto.AgentRunListItem {
	item := &respdto.AgentRunListItem{
		ID:          run.ID.String(),
		Query:       run.Query,
		Status:      run.Status,
		TotalTokens: run.TotalTokens,
		DurationMs:  run.DurationMs,
		CreatedAt:   run.CreatedAt,
	}
	if run.ExecutionMode != nil {
		mode := *run.ExecutionMode
		item.ExecutionMode = &mode
	}
	if run.ErrorCode != nil {
		item.ErrorCode = run.ErrorCode
	}
	return item
}
