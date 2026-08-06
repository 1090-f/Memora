package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"github.com/1090-f/Memora/pkg/utils"
	"go.uber.org/zap"
)

// Executor 负责执行一次工具调用。
// 它会做一系列前置校验（参数、权限、只读、网络等），
// 调用注册表中的工具，并对返回结果做清洗、限长与日志记录。
type Executor struct {
	registry *Registry
}

// NewExecutor 基于给定的工具注册表创建执行器。
func NewExecutor(registry *Registry) *Executor {
	return &Executor{registry: registry}
}

// Execute 执行一次工具调用并返回归一化的 ToolResult。
// 所有的校验失败都不会以 error 返回，而是通过 ToolResult 的错误码表达，
// 这样上层可以统一按返回值处理。
func (e *Executor) Execute(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	startedAt := time.Now()
	result := contracts.ToolResult{CallID: call.CallID, ToolName: call.ToolName}

	// 校验调用上下文中的必填字段。
	if err := validateToolContext(toolContext); err != nil {
		return failedResult(result, contracts.ErrInvalidArgument, err.Error()), nil
	}

	// 工具名与调用 ID 均不可为空。
	if call.ToolName == "" || call.CallID == "" {
		return failedResult(result, contracts.ErrInvalidArgument, "tool_name and call_id are required"), nil
	}

	// 从注册表查找目标工具。
	tool, ok := e.registry.Tool(call.ToolName)
	if !ok {
		return failedResult(result, contracts.ErrResourceNotFound, "tool not found"), nil
	}
	spec := tool.Spec()
	// 非 MCP 工具必须是已启用的；MCP 工具在 Run 中实时查询数据库状态。
	if spec.Type != contracts.ToolTypeMCP && !spec.Enabled {
		return failedResult(result, contracts.ErrForbidden, "tool is disabled"), nil
	}

	// 工具必须在被明确允许的名单中。
	if !isAllowedTool(toolContext.AllowedToolNames, call.ToolName) {
		return failedResult(result, contracts.ErrForbidden, "tool is not allowed"), nil
	}

	// MCP 类型的写工具被禁止，且非只读工具一律被拒绝。
	if spec.Type == contracts.ToolTypeMCP && !spec.ReadOnly {
		return failedResult(result, contracts.ErrWriteMCPToolForbidden, "write mcp tool is forbidden"), nil
	}
	if !spec.ReadOnly {
		return failedResult(result, contracts.ErrForbidden, "tool is not read-only"), nil
	}

	// 需要联网的工具必须在联网开启时才能调用。
	if spec.NetworkRequired && !toolContext.NetworkEnabled {
		return failedResult(result, contracts.ErrNetworkDisabled, "network is disabled"), nil
	}

	// 记录调用开始日志。
	log := logger.GetLogger()
	if log != nil {
		log.Info("tool call started",
			zap.String("tool_name", call.ToolName),
			zap.String("tool_call_id", string(call.CallID)),
			zap.String("agent_run_id", string(toolContext.AgentRunID)),
			zap.String("user_id", string(toolContext.UserID)),
			zap.String("knowledge_base_id", string(toolContext.KnowledgeBaseID)),
			zap.String("arguments", utils.SummarizeAndRedact(call.Arguments)),
		)
	}

	// 若工具定义了超时，则给执行上下文加上超时限制。
	execCtx := ctx
	cancel := func() {}
	if spec.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, spec.Timeout)
	}
	defer cancel()

	toolRes, err := tool.Run(execCtx, toolContext, call.Arguments)
	if err != nil {
		// 超时或取消统一映射为上游超时错误。
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return failedResult(result, contracts.ErrUpstreamTimeout, "tool execution timeout"), nil
		}
		return failedResult(result, contracts.ErrInternal, "tool execution failed"), nil
	}

	// 拷贝工具返回的各个字段到统一结果对象。
	result.Text = toolRes.Text
	result.StructuredData = toolRes.StructuredData
	result.Citations = toolRes.Citations
	result.Truncated = toolRes.Truncated
	result.Success = toolRes.Success
	result.ErrorCode = toolRes.ErrorCode
	result.ErrorMessage = toolRes.ErrorMessage

	// 对最终文本做控制字符等内容的清洗。
	result.Text = utils.SanitizeText(result.Text)

	// 若设置了结果大小上限，则压缩结果到该范围内。
	if toolContext.MaxResultBytes > 0 {
		enforceResultSizeLimit(&result, toolContext.MaxResultBytes)
	}

	// 记录调用结束日志。
	if log != nil {
		log.Info("tool call finished",
			zap.String("tool_name", call.ToolName),
			zap.String("tool_call_id", string(call.CallID)),
			zap.Duration("duration", time.Since(startedAt)),
			zap.Bool("success", result.Success),
			zap.Bool("truncated", result.Truncated),
			zap.String("error_code", string(result.ErrorCode)),
			zap.String("output", utils.SummarizeAndRedactResult(result)),
		)
	}

	return result, nil
}

// validateToolContext 检查 ToolContext 中的必填字段（用户、知识库、运行 ID），
// 缺失字段会以逗号拼接后统一返回错误。
func validateToolContext(toolContext contracts.ToolContext) error {
	var missing []string
	if toolContext.UserID == "" {
		missing = append(missing, "user_id")
	}
	if toolContext.KnowledgeBaseID == "" {
		missing = append(missing, "knowledge_base_id")
	}
	if toolContext.AgentRunID == "" {
		missing = append(missing, "agent_run_id")
	}
	if toolContext.MaxResultBytes < 0 {
		missing = append(missing, "max_result_bytes")
	}
	if len(missing) > 0 {
		return errors.New("invalid tool_context: " + strings.Join(missing, ", "))
	}
	return nil
}

// failedResult 在基础结果上写入失败状态与错误码、错误信息。
func failedResult(base contracts.ToolResult, code contracts.ErrorCode, message string) contracts.ToolResult {
	base.Success = false
	base.ErrorCode = code
	base.ErrorMessage = message
	return base
}

// isAllowedTool 判断指定工具名是否在允许名单中；名单为空则一律不允许。
func isAllowedTool(allowed []string, name string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, n := range allowed {
		if n == name {
			return true
		}
	}
	return false
}

// enforceResultSizeLimit 将结果压缩到指定字节范围内。
// 优先丢弃结构化数据与引用，然后反复截断文本直到体积达标。
func enforceResultSizeLimit(result *contracts.ToolResult, maxBytes int) {
	b, err := json.Marshal(result)
	if err == nil && len(b) <= maxBytes {
		return
	}

	// 超出大小时先清除结构化数据与引用以尽快达标。
	if len(result.StructuredData) > 0 {
		result.StructuredData = nil
		result.Truncated = true
	}
	if len(result.Citations) > 0 {
		result.Citations = nil
		result.Truncated = true
	}

	// 最多进行 4 轮文本截断。
	for i := 0; i < 4; i++ {
		b, err = json.Marshal(result)
		if err == nil && len(b) <= maxBytes {
			return
		}
		if result.Text == "" {
			return
		}
		result.Text = utils.TruncateUTF8ByBytes(result.Text, maxBytes/2)
		result.Truncated = true
	}
}
