package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
)

const maxToolArgumentBytes = 64 * 1024

// 工具超时限制
const (
	defaultToolTimeout = 60 * time.Second
	maxToolTimeout     = 120 * time.Second

	// 特定工具的超时限制
	webSearchTimeout    = 30 * time.Second
	fetchURLTimeout     = 30 * time.Second
	knowledgeTimeout    = 20 * time.Second
	documentReadTimeout = 60 * time.Second
)

// ToolAvailabilityChecker 在工具真正执行前动态复核工具是否仍可用。
// 这是双层校验机制的第二层：在工具实际调用时再次校验启用状态。
//
// 第一层校验：Agent 启动前，通过 MCPToolRefresher.RefreshForUser() 从数据库查询
// 用户已启用的 MCP 工具，只把启用的工具注册到 Registry，供模型可见。
//
// 第二层校验：工具实际执行前，通过本接口再次向数据库查询工具的实时启用状态。
// 这样可以捕捉到运行过程中用户在前端动态禁用工具的情况，避免执行已被禁用的工具。
//
// 内置工具的启用状态由注册时快照固化即可；MCP 工具的启用状态可在前端动态修改，
// 因此需要在每次调用前向数据层动态复核一次。
//
// 该接口由上层（如 MCP ImportService）实现，工具模块自身不依赖数据层。
type ToolAvailabilityChecker interface {
	CheckToolAvailable(ctx context.Context, userID contracts.ID, spec contracts.ToolSpec) (bool, error)
}

// Executor 是 contracts.ToolExecutor 的唯一安全执行入口。
type Executor struct {
	registry  *Registry
	available ToolAvailabilityChecker // 可选的调用前动态可用性检查器
}

// NewExecutor 创建绑定指定注册表的执行器。
func NewExecutor(registry *Registry) *Executor { return &Executor{registry: registry} }

// SetAvailabilityChecker 注入调用前动态可用性检查器（双层校验的第二层）。
// 第一层校验：Agent 启动前通过 MCPToolRefresher 刷新工具列表，只注册已启用的工具。
// 第二层校验：工具实际调用前通过本检查器再次向数据库查询实时启用状态。
// 内置工具可省略；注入后 Executor 会对带 SourceID 的工具（MCP）在真正执行前动态复检。
func (e *Executor) SetAvailabilityChecker(checker ToolAvailabilityChecker) {
	e.available = checker
}

// Spec 返回工具的运行规格，供上层执行器实施按工具调用次数限制。
func (e *Executor) Spec(name string) (contracts.ToolSpec, bool) {
	if e == nil || e.registry == nil {
		return contracts.ToolSpec{}, false
	}
	value, ok := e.registry.find(name)
	if !ok {
		return contracts.ToolSpec{}, false
	}
	return value.Spec(), true
}

// Specs returns a stable snapshot for Plan-Execute tool selection.
func (e *Executor) Specs() []contracts.ToolSpec {
	if e == nil || e.registry == nil {
		return nil
	}
	return e.registry.Specs()
}

// Execute 统一执行前置授权、参数大小/合法性、超时和结果归一化检查。
func (e *Executor) Execute(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (contracts.ToolResult, error) {
	if e == nil || e.registry == nil {
		return failure(call, contracts.ErrInvalidState, "tool registry is unavailable")
	}
	value, ok := e.registry.find(call.ToolName)
	if !ok {
		return failure(call, contracts.ErrResourceNotFound, "tool is not registered")
	}
	spec := value.Spec()
	if !spec.Enabled {
		return failure(call, contracts.ErrInvalidState, "tool is disabled")
	}
	if !spec.ReadOnly {
		return failure(call, contracts.ErrWriteMCPToolForbidden, "write tool is forbidden")
	}
	// 仅在显式设置了白名单时才进行过滤。当 AllowedToolNames 为空时，表示无限制，所有已启用工具均可调用。
	if len(toolContext.AllowedToolNames) > 0 && !contains(toolContext.AllowedToolNames, call.ToolName) {
		return failure(call, contracts.ErrForbidden, "tool is not allowed")
	}
	if spec.NetworkRequired && !toolContext.NetworkEnabled {
		return failure(call, contracts.ErrNetworkDisabled, "network tool is disabled")
	}

	// ====== 双层校验机制的第二层：工具实际调用前的实时启用状态检查 ======
	// 第一层（Agent 启动前）：通过 MCPToolRefresher.RefreshForUser() 查询数据库，
	//   只把已启用的 MCP 工具注册到 Registry，模型只能看到已启用的工具。
	// 第二层（工具调用前）：再次向数据库查询工具的实时启用状态，捕捉运行过程中
	//   用户在前端动态禁用工具的情况。对于 MCP 工具，注册时的快照已不可信，
	//   必须在每次调用前向数据层动态复核 Server 与 Tool 的启用状态。
	// 内置工具的 SourceID 为空，跳过动态检查，仅依赖注册时的静态快照。
	if e.available != nil && spec.SourceID != "" {
		available, err := e.available.CheckToolAvailable(ctx, toolContext.UserID, spec)
		if err != nil {
			return failure(call, contracts.ErrInternal, "failed to check tool availability")
		}
		if !available {
			return failure(call, contracts.ErrMCPToolDisabled, "tool is no longer available")
		}
	}
	if len(call.Arguments) > maxToolArgumentBytes {
		return failure(call, contracts.ErrPayloadTooLarge, "tool arguments are too large")
	}
	if !json.Valid(call.Arguments) {
		return failure(call, contracts.ErrInvalidArgument, "tool arguments must be valid JSON")
	}

	callContext := ctx
	// 设置工具调用超时：优先使用工具配置的超时，否则使用默认超时
	toolTimeout := spec.Timeout
	if toolTimeout <= 0 {
		toolTimeout = defaultToolTimeout
	}

	// 根据工具能力应用最大超时限制
	if hasCapability(spec.Capabilities, contracts.CapabilityWebFetch) {
		if toolTimeout > fetchURLTimeout {
			toolTimeout = fetchURLTimeout
		}
	} else if hasCapability(spec.Capabilities, contracts.CapabilityWebSearch) {
		if toolTimeout > webSearchTimeout {
			toolTimeout = webSearchTimeout
		}
	} else if hasCapability(spec.Capabilities, contracts.CapabilityKnowledge) {
		if toolTimeout > knowledgeTimeout {
			toolTimeout = knowledgeTimeout
		}
	} else if hasCapability(spec.Capabilities, contracts.CapabilityDocumentRead) {
		if toolTimeout > documentReadTimeout {
			toolTimeout = documentReadTimeout
		}
	} else {
		// 其他工具使用全局最大超时
		if toolTimeout > maxToolTimeout {
			toolTimeout = maxToolTimeout
		}
	}

	var cancel context.CancelFunc
	callContext, cancel = context.WithTimeout(ctx, toolTimeout)
	defer cancel()

	result, err := value.Execute(callContext, toolContext, call)
	if err != nil {
		if result.CallID == "" {
			result.CallID = call.CallID
		}
		if result.ToolName == "" {
			result.ToolName = call.ToolName
		}
		if result.ErrorCode == "" {
			result.ErrorCode = contracts.ErrInternal
		}
		result.Success = false
		return result, err
	}
	return truncateResult(result, toolContext.MaxResultBytes), nil
}

// InvokeEino 将标准 ToolResult 序列化为模型工具调用需要的 JSON 字符串。
func (e *Executor) InvokeEino(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (string, error) {
	result, err := e.Execute(ctx, toolContext, call)
	data, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return "", fmt.Errorf("marshal tool result: %w", marshalErr)
	}
	if err != nil {
		return string(data), err
	}
	return string(data), nil
}

// InvokeContext 是将服务端 ToolContext 注入 Eino 上下文的便捷入口。
func (e *Executor) InvokeContext(ctx context.Context, toolContext contracts.ToolContext, call contracts.ToolCall) (string, error) {
	return e.InvokeEino(withToolContext(ctx, toolContext), toolContext, call)
}

func failure(call contracts.ToolCall, code contracts.ErrorCode, message string) (contracts.ToolResult, error) {
	result := contracts.ToolResult{CallID: call.CallID, ToolName: call.ToolName, Success: false, ErrorCode: code, ErrorMessage: message}
	return result, fmt.Errorf("%s: %s", code, message)
}

// truncateResult 同时限制 Text 和 StructuredData，不能只限制模型看到的文本字段。
func truncateResult(result contracts.ToolResult, maxBytes int) contracts.ToolResult {
	if maxBytes <= 0 {
		return result
	}
	if len(result.Text) > maxBytes {
		result.Text = result.Text[:maxBytes]
		result.Truncated = true
	}
	if len(result.StructuredData) > maxBytes {
		result.StructuredData = append(json.RawMessage(nil), result.StructuredData[:maxBytes]...)
		result.Truncated = true
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// hasCapability 检查工具是否具有指定能力
func hasCapability(capabilities []string, required string) bool {
	for _, cap := range capabilities {
		if cap == required {
			return true
		}
	}
	return false
}

var _ contracts.ToolExecutor = (*Executor)(nil)
