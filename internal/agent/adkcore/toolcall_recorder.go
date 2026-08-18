package adkcore

import (
	"context"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
)

// ToolCallRecorderMiddleware 是 ADK ChatModelAgent 的中间件，
// 负责将每次工具调用的详细信息持久化到 tool_calls 表中。
//
// 执行流程：
//  1. 工具调用开始前：创建一条 ToolCall 记录（status = running）
//  2. 工具调用完成后：更新 ToolCall 记录（status = succeeded / failed）
//
// 该中间件应放置在 SafeToolMiddleware 与 AgentMiddleware 之间，
// 这样可以捕获到原始的工具调用错误（未被 SafeToolMiddleware 吞掉之前）。
type ToolCallRecorderMiddleware struct {
	adk.BaseChatModelAgentMiddleware

	// ToolCallRepo 用于持久化工具调用记录。
	ToolCallRepo repository.ToolCallRepository
	// RunID 当前运行的 ID，用于关联工具调用记录。
	RunID contracts.ID
}

var _ adk.ChatModelAgentMiddleware = (*ToolCallRecorderMiddleware)(nil)

// WrapInvokableToolCall 包装非流式工具调用，记录调用详情到数据库。
func (m *ToolCallRecorderMiddleware) WrapInvokableToolCall(
	ctx context.Context,
	endpoint adk.InvokableToolCallEndpoint,
	tc *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	// 如果没有工具上下文或未配置仓库，直接透传。
	if tc == nil || m.ToolCallRepo == nil {
		return endpoint, nil
	}

	wrapped := func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		runID, err := uuid.Parse(string(m.RunID))
		if err != nil {
			return endpoint(ctx, argumentsInJSON, opts...)
		}

		startedAt := time.Now().UTC()
		callID := uuid.New()

		// 1. 创建初始工具调用记录（status = running）
		toolCall := &entity.ToolCall{
			ID:           callID,
			AgentRunID:   runID,
			ToolName:     tc.Name,
			ToolType:     "internal", // ReAct 模式下的工具默认为 internal
			Status:       "running",
			InputSummary: argumentsInJSON,
			StartedAt:    startedAt,
		}
		_ = m.ToolCallRepo.Create(ctx, toolCall) // 创建失败不阻塞工具调用

		// 2. 执行实际工具调用
		result, toolErr := endpoint(ctx, argumentsInJSON, opts...)
		endedAt := time.Now().UTC()
		durationMs := endedAt.Sub(startedAt).Milliseconds()

		// 3. 更新工具调用结果
		status := "succeeded"
		var errorCode, errorMessage string
		if toolErr != nil {
			status = "failed"
			errorCode = "tool_error"
			errorMessage = toolErr.Error()
		}

		_ = m.ToolCallRepo.UpdateResult(ctx, callID, status, result, errorCode, errorMessage, durationMs, false)

		return result, toolErr
	}

	return wrapped, nil
}
