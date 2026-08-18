package adkcore

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
)

// ToolSpecLookup 根据工具名称返回其规格信息，用于判断工具类型（internal / mcp）。
// 工具未注册或无法解析时返回 (零值, false)。
type ToolSpecLookup func(name string) (contracts.ToolSpec, bool)

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
	// ToolSpecLookup 用于解析工具类型（internal / mcp），可选。
	ToolSpecLookup ToolSpecLookup
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
		return m.recordToolCall(ctx, tc, argumentsInJSON, func(ctx context.Context) (string, error) {
			return endpoint(ctx, argumentsInJSON, opts...)
		})
	}

	return wrapped, nil
}

// WrapStreamableToolCall 包装流式工具调用，同样记录调用详情到数据库。
func (m *ToolCallRecorderMiddleware) WrapStreamableToolCall(
	ctx context.Context,
	endpoint adk.StreamableToolCallEndpoint,
	tc *adk.ToolContext,
) (adk.StreamableToolCallEndpoint, error) {
	// 如果没有工具上下文或未配置仓库，直接透传。
	if tc == nil || m.ToolCallRepo == nil {
		return endpoint, nil
	}

	wrapped := func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (*schema.StreamReader[string], error) {
		return m.recordStreamableToolCall(ctx, tc, argumentsInJSON, func(ctx context.Context) (*schema.StreamReader[string], error) {
			return endpoint(ctx, argumentsInJSON, opts...)
		})
	}

	return wrapped, nil
}

// recordToolCall 记录非流式工具调用的开始与结束。
func (m *ToolCallRecorderMiddleware) recordToolCall(
	ctx context.Context,
	tc *adk.ToolContext,
	argumentsInJSON string,
	run func(ctx context.Context) (string, error),
) (string, error) {
	callID, startedAt, ok := m.createToolCall(ctx, tc, argumentsInJSON)
	if !ok {
		return run(ctx)
	}

	result, toolErr := run(ctx)
	m.finishToolCall(ctx, callID, startedAt, result, toolErr)
	return result, toolErr
}

// recordStreamableToolCall 记录流式工具调用的开始与结束。
// 流式端点返回 StreamReader，本方法会原样消费一份输出以生成结果摘要，
// 同时将另一份等价输出流返回给下游，保证数据流不丢失。
func (m *ToolCallRecorderMiddleware) recordStreamableToolCall(
	ctx context.Context,
	tc *adk.ToolContext,
	argumentsInJSON string,
	run func(ctx context.Context) (*schema.StreamReader[string], error),
) (*schema.StreamReader[string], error) {
	callID, startedAt, ok := m.createToolCall(ctx, tc, argumentsInJSON)
	if !ok {
		return run(ctx)
	}

	reader, toolErr := run(ctx)
	if toolErr != nil {
		m.finishToolCall(ctx, callID, startedAt, "", toolErr)
		return nil, toolErr
	}
	if reader == nil {
		m.finishToolCall(ctx, callID, startedAt, "", nil)
		return nil, nil
	}

	copies := reader.Copy(2)
	var parts []string
	for {
		chunk, recvErr := copies[0].Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			m.finishToolCall(ctx, callID, startedAt, concatStrings(parts), recvErr)
			return nil, recvErr
		}
		parts = append(parts, chunk)
	}
	copies[0].Close()

	m.finishToolCall(ctx, callID, startedAt, concatStrings(parts), nil)
	return copies[1], nil
}

// createToolCall 创建一条 status = running 的工具调用记录。
// 返回调用 ID 与开始时间；仓库缺失或 RunID 非法时不创建记录并返回 ok = false。
func (m *ToolCallRecorderMiddleware) createToolCall(ctx context.Context, tc *adk.ToolContext, argumentsInJSON string) (uuid.UUID, time.Time, bool) {
	if m.ToolCallRepo == nil {
		return uuid.Nil, time.Time{}, false
	}

	runID, err := uuid.Parse(string(m.RunID))
	if err != nil {
		return uuid.Nil, time.Time{}, false
	}

	startedAt := time.Now().UTC()
	callID := uuid.New()

	toolType, mcpServerID, mcpToolID := m.resolveToolIdentity(tc.Name)

	// 1. 创建初始工具调用记录（status = running）
	toolCall := &entity.ToolCall{
		ID:           callID,
		AgentRunID:   runID,
		ToolName:     tc.Name,
		ToolType:     toolType,
		MCPServerID:  mcpServerID,
		MCPToolID:    mcpToolID,
		Status:       "running",
		InputSummary: argumentsInJSON,
		StartedAt:    startedAt,
	}
	_ = m.ToolCallRepo.Create(ctx, toolCall) // 创建失败不阻塞工具调用

	return callID, startedAt, true
}

// resolveToolIdentity 根据工具规格解析工具类型、所属 MCP Server ID 与 MCP 工具 ID。
// 内置工具返回 ("internal", nil, nil)；MCP 工具返回 ("mcp", serverID, toolID)。
func (m *ToolCallRecorderMiddleware) resolveToolIdentity(name string) (string, *uuid.UUID, *uuid.UUID) {
	if m.ToolSpecLookup == nil {
		return "internal", nil, nil
	}
	spec, ok := m.ToolSpecLookup(name)
	if !ok || spec.Type != contracts.ToolTypeMCP {
		return "internal", nil, nil
	}

	var serverID, toolID *uuid.UUID
	if id, err := uuid.Parse(spec.SourceID); err == nil {
		serverID = &id
	}
	if id, err := uuid.Parse(spec.MCPToolID); err == nil {
		toolID = &id
	}
	return "mcp", serverID, toolID
}

// finishToolCall 更新工具调用记录的执行结果。
func (m *ToolCallRecorderMiddleware) finishToolCall(ctx context.Context, callID uuid.UUID, startedAt time.Time, result string, toolErr error) {
	if m.ToolCallRepo == nil {
		return
	}

	endedAt := time.Now().UTC()
	durationMs := endedAt.Sub(startedAt).Milliseconds()

	status := "succeeded"
	var errorCode, errorMessage string
	if toolErr != nil {
		status = "failed"
		errorCode = "tool_error"
		errorMessage = toolErr.Error()
	}

	_ = m.ToolCallRepo.UpdateResult(ctx, callID, status, result, errorCode, errorMessage, durationMs, false)
}

func concatStrings(parts []string) string {
	var output strings.Builder
	for _, part := range parts {
		output.WriteString(part)
	}
	return output.String()
}
