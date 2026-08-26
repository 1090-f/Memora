package request

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MCPImportRequest 是 MCP Server 一键导入的请求体，
// 顶层固定为 mcpServers 对象，键为 server 名称，值为 server 配置。
type MCPImportRequest struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers" binding:"required,min=1"`
}

// MCPServerConfig 描述单个 MCP Server 的导入配置，
// 兼容 Trae / Claude Desktop / Cursor 的 JSON 格式。
type MCPServerConfig struct {
	Transport        *string           `json:"transport,omitempty"`
	URL              *string           `json:"url,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	Command          *string           `json:"command,omitempty"`
	Args             []string          `json:"args,omitempty"`
	Env              EnvVars           `json:"env,omitempty"`
	CWD              *string           `json:"cwd,omitempty"`
	Description      *string           `json:"description,omitempty"`
	NetworkRequired  *bool             `json:"network_required,omitempty"`
	ConnectTimeoutMs *int              `json:"connect_timeout_ms,omitempty"`
	CallTimeoutMs    *int              `json:"call_timeout_ms,omitempty"`
	MaxResponseBytes *int              `json:"max_response_bytes,omitempty"`
	Enabled          *bool             `json:"enabled,omitempty"`
}

// EnvVars 兼容两种环境变量写法：
//   - Claude Desktop 的 map[string]string 格式
//   - Trae 的 array[{name, value}] 格式
//
// 统一归一化为 map[string]string。
type EnvVars map[string]string

func (e *EnvVars) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	// 尝试按 map[string]string 解析（Claude Desktop 格式）
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		*e = EnvVars(m)
		return nil
	}
	// 尝试按数组解析（Trae 结构化格式）
	var items []envItem
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("env must be a string map or an array of {name, value} objects: %w", err)
	}
	result := make(map[string]string, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return fmt.Errorf("env item name is required")
		}
		result[name] = item.Value
	}
	*e = EnvVars(result)
	return nil
}

type envItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// UpdateToolEnabledRequest 用于更新 MCP 工具启用/停用状态。
type UpdateToolEnabledRequest struct {
	Enabled bool `json:"enabled"`
}
