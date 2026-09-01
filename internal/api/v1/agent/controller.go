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
	"gorm.io/gorm"
)

// Controller 处理 Agent 运行相关的 HTTP 请求。
type Controller struct {
	agentService    contracts.AgentRunService        // Agent 核心执行服务（包含 Run/Cancel/Retry）
	runRepo         repository.AgentRunRepository    // 运行记录的持久化查询
	toolCallRepo    repository.ToolCallRepository    // 工具调用的持久化查询
	agentConfigRepo repository.AgentConfigRepository // Agent 配置查询仓库
	eventSub        contracts.EventSubscriber        // Agent 事件订阅器（用于 SSE 流式推送）
	agentEventRepo  repository.AgentEventRepository  // Agent 事件持久化仓库（用于断线重连时的历史回放）
}

// NewController 创建 Agent 运行管理的 HTTP 控制器实例。
func NewController(
	agentService contracts.AgentRunService,
	runRepo repository.AgentRunRepository,
	toolCallRepo repository.ToolCallRepository,
	agentConfigRepo repository.AgentConfigRepository,
	eventSub contracts.EventSubscriber,
	agentEventRepo repository.AgentEventRepository,
) *Controller {
	return &Controller{
		agentService:    agentService,
		runRepo:         runRepo,
		toolCallRepo:    toolCallRepo,
		agentConfigRepo: agentConfigRepo,
		eventSub:        eventSub,
		agentEventRepo:  agentEventRepo,
	}
}

// CreateRun 处理 POST /api/v1/agent/runs，创建智能问答任务并排队执行。
// 流程：校验会话与模型 → 原子创建消息和排队运行 → 立即返回 run_id。
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

	// 查询 Agent 配置，获取运行记录所需的行为配置外键。
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

	run, err := ctrl.runRepo.CreateQueuedForConversation(c.Request.Context(), userID, knowledgeBaseID, conversationID, agentConfigID, req.Query, middleware.GetTraceID(c), middleware.GetRequestID(c))
	if err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			response.Failure(c, apperrors.ErrNotFound)
		} else if errors.Is(err, repository.ErrModelConfigNotFound) {
			response.Failure(c, apperrors.New(contracts.ErrInvalidState, err))
		} else {
			response.Failure(c, apperrors.ErrInternal)
		}
		return
	}

	response.Success(c, http.StatusAccepted, respdto.CreateAgentRunResponse{
		RunID:          run.ID.String(),
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Failure(c, apperrors.ErrNotFound)
			return
		}
		response.Failure(c, apperrors.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, toRunResponse(run))
}

// ListToolCalls 处理 GET /api/v1/agent/runs/:id/tool-calls，获取指定运行的完整工具调用列表。
// 用于在运行链路中查看某条工具调用的输入输出与执行元数据。
// 通过先校验运行归属，避免越权访问其他用户的工具调用记录。
func (ctrl *Controller) ListToolCalls(c *gin.Context) {
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

	// 先校验运行存在且属于当前用户，再查询工具调用记录。
	if _, err := ctrl.runRepo.FindByID(c.Request.Context(), userID, runID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Failure(c, apperrors.ErrNotFound)
			return
		}
		response.Failure(c, apperrors.ErrInternal)
		return
	}

	calls, err := ctrl.toolCallRepo.ListByRunID(c.Request.Context(), runID)
	if err != nil {
		response.Failure(c, apperrors.ErrInternal)
		return
	}

	items := make([]respdto.ToolCallResponse, 0, len(calls))
	for i := range calls {
		items = append(items, toToolCallResponse(&calls[i]))
	}

	response.Success(c, http.StatusOK, items)
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
	createdFrom, err := parseOptionalTime(c.Query("created_from"))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	createdTo, err := parseOptionalTime(c.Query("created_to"))
	if err != nil {
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
	conversationUUID := uuid.Nil
	if conversationID != "" {
		conversationUUID, err = uuid.Parse(conversationID)
		if err != nil {
			response.Failure(c, apperrors.ErrInvalidArgument)
			return
		}
	}

	runs, total, err := ctrl.runRepo.ListByOwner(c.Request.Context(), userID, kbUUID, conversationUUID, status, executionMode, createdFrom, createdTo, page, pageSize)
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

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
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
	// 1. 解析运行 ID 和起始序列号
	parsedRunID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	userID, err := uuid.Parse(string(user.ID))
	if err != nil {
		response.Failure(c, apperrors.ErrInvalidArgument)
		return
	}
	// SSE 会回放持久化事件，必须在写响应头前验证 Run 归属，避免跨用户读取。
	if _, err := ctrl.runRepo.FindByID(c.Request.Context(), userID, parsedRunID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Failure(c, apperrors.ErrNotFound)
			return
		}
		response.Failure(c, apperrors.ErrInternal)
		return
	}
	runID := contracts.ID(parsedRunID.String())
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

	// 3. 从 DB 回放历史事件（用于断线重连时恢复到最新状态）
	// 在连接 Redis 实时流之前，先将 DB 中已持久化的未读事件通过 SSE 推送。
	if ctrl.agentEventRepo != nil {
		dbEvents, err := ctrl.agentEventRepo.ListAfterSequence(c.Request.Context(), string(runID), afterSeq)
		if err != nil {
			logger.Warn("读取 Agent 历史事件失败，跳过 DB 回放",
				zap.String("run_id", string(runID)),
				zap.Error(err),
			)
		} else if len(dbEvents) > 0 {
			logger.Info("回放 Agent 历史事件",
				zap.String("run_id", string(runID)),
				zap.Int("event_count", len(dbEvents)),
			)
			for _, dbEv := range dbEvents {
				// 将 entity.AgentEvent 转换为 contracts.AgentEvent 以保持 SSE 格式一致
				var rawData json.RawMessage
				if dbEv.Data != nil {
					rawData = json.RawMessage(dbEv.Data)
				} else {
					rawData = json.RawMessage("{}")
				}
				event := contracts.AgentEvent{
					EventID:   contracts.ID(""),
					RunID:     contracts.ID(dbEv.RunID),
					EventType: contracts.EventType(dbEv.EventType),
					Sequence:  dbEv.Sequence,
					Timestamp: dbEv.Timestamp,
					Data:      rawData,
				}
				if dbEv.TraceID != nil {
					event.TraceID = *dbEv.TraceID
				}
				if dbEv.RequestID != nil {
					event.RequestID = *dbEv.RequestID
				}
				if dbEv.Stage != nil {
					event.Stage = contracts.AgentStage(*dbEv.Stage)
				}
				if dbEv.Status != nil {
					event.Status = contracts.StageStatus(*dbEv.Status)
				}
				eventData, _ := json.Marshal(event)
				fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.EventType, eventData)
				c.Writer.Flush()

				// 更新 afterSeq 为已推送的最大序号，避免后续实时流重复推送
				if event.Sequence > afterSeq {
					afterSeq = event.Sequence
				}

				// 如果历史事件中包含终端事件，则提前结束
				if event.EventType == contracts.EventRunCompleted ||
					event.EventType == contracts.EventRunFailed ||
					event.EventType == contracts.EventRunCancelled {
					return
				}
			}
		}
	}

	// 4. 订阅实时事件通道（Redis Stream）
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

	// 5. 持续消费实时事件并通过 SSE 协议推送
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

// buildConfigFromContext 从 AgentContext 构建 AgentConfig。
// 将 AgentContext 中的配置参数映射为运行配置，超出 AgentContext 的字段使用默认值。
func buildConfigFromContext(ctx contracts.AgentContext) contracts.AgentConfig {
	config := contracts.DefaultAgentConfig()

	// 覆盖 ReAct 模式最大轮次（若上下文中有显式配置）
	if ctx.MaxReactRounds > 0 {
		config.MaxReactRounds = ctx.MaxReactRounds
	}

	// Plan-Execute 模式配置
	if ctx.MaxPlanSteps > 0 {
		config.MaxPlanSteps = ctx.MaxPlanSteps
	}
	if ctx.MaxReplans >= 0 {
		config.MaxReplans = ctx.MaxReplans
	}
	if ctx.ReviewerRuns > 0 {
		config.ReviewerRuns = ctx.ReviewerRuns
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
		ID:                 run.ID.String(),
		UserID:             run.UserID.String(),
		KnowledgeBaseID:    run.KnowledgeBaseID.String(),
		ConversationID:     run.ConversationID.String(),
		AgentConfigID:      run.AgentConfigID.String(),
		ChatModelID:        run.ChatModelID.String(),
		Query:              run.Query,
		TraceID:            run.TraceID,
		RequestID:          run.RequestID,
		ExecutionTrace:     json.RawMessage(run.ExecutionTrace),
		RouterConfidence:   run.RouterConfidence,
		RouterFallbackUsed: run.RouterFallbackUsed,
		RouterReason:       run.RouterReasonSummary,
		Status:             run.Status,
		MemoryUsedCount:    run.MemoryUsedCount,
		InputTokens:        run.InputTokens,
		OutputTokens:       run.OutputTokens,
		TotalTokens:        run.TotalTokens,
		DurationMs:         run.DurationMs,
		StartedAt:          run.StartedAt,
		EndedAt:            run.EndedAt,
		CreatedAt:          run.CreatedAt,
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

// toToolCallResponse 将 ToolCall 实体转换为 API 响应 DTO。
// JSON 列（arguments_redacted、result_meta）直接以原始字节透传，nil 时缺省。
func toToolCallResponse(call *entity.ToolCall) respdto.ToolCallResponse {
	resp := respdto.ToolCallResponse{
		ID:            call.ID.String(),
		ToolName:      call.ToolName,
		ToolType:      call.ToolType,
		Status:        call.Status,
		InputSummary:  call.InputSummary,
		OutputSummary: call.OutputSummary,
		IsTruncated:   call.IsTruncated,
		StartedAt:     call.StartedAt,
	}
	if len(call.ArgumentsRedacted) > 0 {
		resp.ArgumentsRedacted = json.RawMessage(call.ArgumentsRedacted)
	}
	if len(call.ResultMeta) > 0 {
		resp.ResultMeta = json.RawMessage(call.ResultMeta)
	}
	if call.ReactRoundNo != nil {
		resp.ReactRoundNo = call.ReactRoundNo
	}
	if call.ResponseBytes != nil {
		resp.ResponseBytes = call.ResponseBytes
	}
	if call.ErrorCode != nil {
		resp.ErrorCode = call.ErrorCode
	}
	if call.ErrorMessage != nil {
		resp.ErrorMessage = call.ErrorMessage
	}
	if call.DurationMs != nil {
		resp.DurationMs = call.DurationMs
	}
	if call.EndedAt != nil {
		resp.EndedAt = call.EndedAt
	}
	return resp
}
