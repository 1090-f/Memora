package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/pkg/logger"
	"go.uber.org/zap"
)

type Executor struct {
	registry *Registry
}

func NewExecutor(registry *Registry) *Executor {
	return &Executor{registry: registry}
}

func (e *Executor) Execute(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	startedAt := time.Now()
	result := contracts.ToolResult{CallID: call.CallID, ToolName: call.ToolName}

	if err := validateToolContext(toolContext); err != nil {
		return failedResult(result, contracts.ErrInvalidArgument, err.Error()), nil
	}

	if call.ToolName == "" || call.CallID == "" {
		return failedResult(result, contracts.ErrInvalidArgument, "tool_name and call_id are required"), nil
	}

	tool, ok := e.registry.Tool(call.ToolName)
	if !ok {
		return failedResult(result, contracts.ErrResourceNotFound, "tool not found"), nil
	}
	spec := tool.Spec()
	if !spec.Enabled {
		return failedResult(result, contracts.ErrForbidden, "tool is disabled"), nil
	}

	if !isAllowedTool(toolContext.AllowedToolNames, call.ToolName) {
		return failedResult(result, contracts.ErrForbidden, "tool is not allowed"), nil
	}

	if spec.Type == contracts.ToolTypeMCP && !spec.ReadOnly {
		return failedResult(result, contracts.ErrWriteMCPToolForbidden, "write mcp tool is forbidden"), nil
	}
	if !spec.ReadOnly {
		return failedResult(result, contracts.ErrForbidden, "tool is not read-only"), nil
	}

	if spec.NetworkRequired && !toolContext.NetworkEnabled {
		return failedResult(result, contracts.ErrNetworkDisabled, "network is disabled"), nil
	}

	log := logger.GetLogger()
	if log != nil {
		log.Info("tool call started",
			zap.String("tool_name", call.ToolName),
			zap.String("tool_call_id", string(call.CallID)),
			zap.String("agent_run_id", string(toolContext.AgentRunID)),
			zap.String("user_id", string(toolContext.UserID)),
			zap.String("knowledge_base_id", string(toolContext.KnowledgeBaseID)),
			zap.String("arguments", summarizeAndRedact(call.Arguments)),
		)
	}

	execCtx := ctx
	cancel := func() {}
	if spec.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, spec.Timeout)
	}
	defer cancel()

	toolRes, err := tool.Run(execCtx, toolContext, call.Arguments)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return failedResult(result, contracts.ErrUpstreamTimeout, "tool execution timeout"), nil
		}
		return failedResult(result, contracts.ErrInternal, "tool execution failed"), nil
	}

	result.Text = toolRes.Text
	result.StructuredData = toolRes.StructuredData
	result.Citations = toolRes.Citations
	result.Truncated = toolRes.Truncated
	result.Success = toolRes.Success
	result.ErrorCode = toolRes.ErrorCode
	result.ErrorMessage = toolRes.ErrorMessage

	result.Text = sanitizeText(result.Text)

	if toolContext.MaxResultBytes > 0 {
		enforceResultSizeLimit(&result, toolContext.MaxResultBytes)
	}

	if log != nil {
		log.Info("tool call finished",
			zap.String("tool_name", call.ToolName),
			zap.String("tool_call_id", string(call.CallID)),
			zap.Duration("duration", time.Since(startedAt)),
			zap.Bool("success", result.Success),
			zap.Bool("truncated", result.Truncated),
			zap.String("error_code", string(result.ErrorCode)),
			zap.String("output", summarizeAndRedactResult(result)),
		)
	}

	return result, nil
}

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

func failedResult(base contracts.ToolResult, code contracts.ErrorCode, message string) contracts.ToolResult {
	base.Success = false
	base.ErrorCode = code
	base.ErrorMessage = message
	return base
}

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

func enforceResultSizeLimit(result *contracts.ToolResult, maxBytes int) {
	b, err := json.Marshal(result)
	if err == nil && len(b) <= maxBytes {
		return
	}

	if len(result.StructuredData) > 0 {
		result.StructuredData = nil
		result.Truncated = true
	}
	if len(result.Citations) > 0 {
		result.Citations = nil
		result.Truncated = true
	}

	for i := 0; i < 4; i++ {
		b, err = json.Marshal(result)
		if err == nil && len(b) <= maxBytes {
			return
		}
		if result.Text == "" {
			return
		}
		result.Text = truncateUTF8ByBytes(result.Text, maxBytes/2)
		result.Truncated = true
	}
}
