// Package agent 提供 Agent 运行管理的 HTTP API。
// 包括创建智能问答、查询运行状态、取消和重试等端点。
package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Controller 处理 Agent 运行相关的 HTTP 请求。
type Controller struct {
	agentService    contracts.AgentRunService        // Agent 核心执行服务（包含 Run/Cancel/Retry）
	runRepo         repository.AgentRunRepository    // 运行记录的持久化查询
	toolCallRepo    repository.ToolCallRepository    // 工具调用的持久化查询
	messageRepo     repository.MessageRepository     // 用户消息持久化仓库
	agentConfigRepo repository.AgentConfigRepository // Agent 配置查询仓库
	contextBuilder  contracts.ContextBuilder         // 上下文构建器（组装 AgentContext）
	eventSub        contracts.EventSubscriber        // Agent 事件订阅器（用于 SSE 流式推送）
}

// NewController 创建 Agent 运行管理的 HTTP 控制器实例。
func NewController(
	agentService contracts.AgentRunService,
	runRepo repository.AgentRunRepository,
	toolCallRepo repository.ToolCallRepository,
	messageRepo repository.MessageRepository,
	agentConfigRepo repository.AgentConfigRepository,
	contextBuilder contracts.ContextBuilder,
	eventSub contracts.EventSubscriber,
) *Controller {
	return &Controller{
		agentService:    agentService,
		runRepo:         runRepo,
		toolCallRepo:    toolCallRepo,
		messageRepo:     messageRepo,
		agentConfigRepo: agentConfigRepo,
		contextBuilder:  contextBuilder,
		eventSub:        eventSub,
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
	logger.Info("Agent 请求开始构建上下文",
		zap.String("conversation_id", req.ConversationID),
		zap.String("knowledge_base_id", req.KnowledgeBaseID),
	)
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
	// 上下文构建用于提前校验会话、知识库和 Agent 配置；Worker 会在领取后重新构建最新上下文。
	logger.Info("Agent 请求上下文构建完成",
		zap.String("conversation_id", req.ConversationID),
		zap.Int("history_message_count", len(agentCtx.Conversation.Messages)),
	)
	_ = agentCtx

	// 2. 查询 Agent 配置，获取运行记录所需的外键 ID。
	userID, err := uuid.Parse(string(user.ID))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	knowledgeBaseID, err := uuid.Parse(string(req.KnowledgeBaseID))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	conversationID, err := uuid.Parse(string(req.ConversationID))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	agentConfig, err := ctrl.agentConfigRepo.FindByKnowledgeBase(c.Request.Context(), userID.String(), knowledgeBaseID.String())
	if err != nil {
		response.Failure(c, mapAgentError(err))
		return
	}
	agentConfigID, err := uuid.Parse(agentConfig.ID)
	if err != nil {
		response.Failure(c, apperrors.ErrInternal)
		return
	}

	// 3. 持久化用户消息，作为 agent_runs.user_message_id 的关联记录。
	message := &entity.Message{
		ID:              runUUID.String(),
		ConversationID:  conversationID.String(),
		UserID:          userID.String(),
		KnowledgeBaseID: knowledgeBaseID.String(),
		Role:            "user",
		Content:         req.Query,
		Status:          "completed",
		CreatedAt:       time.Now().UTC(),
	}
	if err := ctrl.messageRepo.Create(c.Request.Context(), message); err != nil {
		response.Failure(c, apperrors.ErrInternal)
		return
	}

	// 4. 创建 queued 状态的 Agent 运行记录，供 Worker 原子领取。
	run := &entity.AgentRun{
		ID:              runUUID,
		UserID:          userID,
		KnowledgeBaseID: knowledgeBaseID,
		ConversationID:  conversationID,
		UserMessageID:   runUUID,
		AgentConfigID:   agentConfigID,
		Query:           req.Query,
		Status:          "queued",
	}
	if err := ctrl.runRepo.CreateQueued(c.Request.Context(), run); err != nil {
		response.Failure(c, apperrors.ErrInternal)
		return
	}

	// 5. 上下文已构建并完成校验，运行记录进入队列后由 Worker 重新构建上下文执行。
	// Worker 重新加载上下文可以避免 HTTP 请求结束后复用已取消的请求上下文。

	// 6. 运行记录已进入 queued 队列，由 Agent Worker 负责领取和执行。
	// HTTP 请求不等待 Agent 完成，run_id 和初始状态立即返回给客户端。

	// 7. 立即返回排队成功响应
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

// ListRuns 处理 GET /api/v1/agent/runs，按用户、知识库和会话分页查询运行记录。
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
	conversationID := c.Query("conversation_id")
	status := c.Query("status")
	executionMode := c.Query("execution_mode")

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
	conversationUUID := uuid.Nil
	if conversationID != "" {
		conversationUUID, err = uuid.Parse(conversationID)
		if err != nil {
			response.Failure(c, apperrors.ErrInvalidArgument)
			return
		}
	}

	runs, total, err := ctrl.runRepo.ListByOwner(c.Request.Context(), userID, kbUUID, conversationUUID, status, executionMode, page, pageSize)
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

// RetryRun 处理 POST /api/v1/agent/runs/:id/retry，基于已有运行创建新的排队运行。
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

// SubscribeEvents 处理 GET /api/v1/agent/runs/:id/events，通过 Server-Sent Events 协议流式推送 Agent 运行事件。
// 客户端可通过 after_sequence 查询参数指定已收到的最新序列号，用于断线重连时跳过已处理的事件。
// 每次推送一个 JSON 格式的 AgentEvent 行，遵循 SSE 协议（Content-Type: text/event-stream）。
func (ctrl *Controller) SubscribeEvents(c *gin.Context) {
	user, ok := middleware.GetUser(c)
	if !ok {
		response.Failure(c, apperrors.ErrUnauthorized)
		return
	}
	_ = user // 当前先校验用户认证，后续可扩展按用户过滤事件

	// 1. 解析运行 ID 和起始序列号
	runID := contracts.ID(c.Param("id"))
	afterSeq, _ := strconv.ParseInt(c.DefaultQuery("after_sequence", "0"), 10, 64)

	// 2. 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲，确保流式推送的实时性

	// 禁用 WriteTimeout：SSE 是长期流式连接，HTTP Server 的全局 WriteTimeout
	// 会强制中断流式推送。使用 http.ResponseController 将写入截止时间置零。
	if rc := http.NewResponseController(c.Writer); rc != nil {
		rc.SetWriteDeadline(time.Time{}) // zero value = no deadline
	}

	// 3. 订阅事件通道
	eventCh, err := ctrl.eventSub.Subscribe(c.Request.Context(), runID, afterSeq)
	if err != nil {
		// SSE 场景下无法返回 JSON（响应头已设为 text/event-stream），直接写入错误事件行
		errData, _ := json.Marshal(gin.H{"error": err.Error()})
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", errData)
		c.Writer.Flush()
		return
	}

	// 创建定时 flush ticker，用于定期发送心跳和刷新缓冲区
	flushTicker := time.NewTicker(10 * time.Second)
	defer flushTicker.Stop()

	// 4. 持续消费事件并通过 SSE 协议推送
	for {
		select {
		case <-c.Request.Context().Done():
			// 客户端断开连接或请求超时，退出循环
			return
		case <-flushTicker.C:
			// 定时发送 SSE 注释行（以 : 开头）作为心跳，防止代理/负载均衡器因长时间无数据而断开连接
			io.WriteString(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		case event, ok := <-eventCh:
			if !ok {
				// 事件通道已关闭（运行结束或异常），发送完成事件后退出
				io.WriteString(c.Writer, "event: complete\ndata: {}\n\n")
				c.Writer.Flush()
				return
			}

			// 将事件序列化为 JSON
			eventData, err := json.Marshal(event)
			if err != nil {
				continue
			}

			// SSE 格式：每个事件由 event 行（事件类型）和 data 行（JSON 载荷）组成，以空行分隔
			// 示例：
			//   event: agent.run.started
			//   data: {"event_id":"...","sequence":1,...}
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.EventType, eventData)
			c.Writer.Flush()

			// 终端事件处理：Agent 运行已结束（完成/失败/取消），无需继续等待。
			// 立即退出循环避免 handler 长时间空转占用连接。
			if event.EventType == contracts.EventRunCompleted ||
				event.EventType == contracts.EventRunFailed ||
				event.EventType == contracts.EventRunCancelled {
				return
			}
		}
	}
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
		ID:             run.ID.String(),
		ConversationID: run.ConversationID.String(),
		Query:          run.Query,
		Status:         run.Status,
		TotalTokens:    run.TotalTokens,
		DurationMs:     run.DurationMs,
		CreatedAt:      run.CreatedAt,
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
