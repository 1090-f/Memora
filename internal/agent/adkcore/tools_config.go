package adkcore

import (
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	agenttools "github.com/1090-f/Memora/internal/agent/tools"
)

// BuildToolsConfig 从当前工具注册表快照构建 ADK ToolsConfig。
// 每次调用都会读取最新注册表，适用于 MCP 工具动态刷新场景。
func BuildToolsConfig(registry *agenttools.Registry, executor *agenttools.Executor) adk.ToolsConfig {
	if registry == nil {
		return adk.ToolsConfig{}
	}

	registeredTools := registry.Tools()
	adkTools := make([]tool.BaseTool, 0, len(registeredTools))
	for _, registeredTool := range registeredTools {
		adkTools = append(adkTools, NewToolAdapter(registeredTool, executor))
	}
	if len(adkTools) == 0 {
		return adk.ToolsConfig{}
	}

	return adk.ToolsConfig{
		ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: adkTools,
		},
	}
}
