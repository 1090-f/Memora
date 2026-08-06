package response

import (
	"encoding/json"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
)

// MCPImportResponse 是 MCP Server 导入接口的响应，
// 包含成功导入的 server 列表和失败的 server 列表。
type MCPImportResponse struct {
	Imported []ImportedServer `json:"imported"`
	Failed   []FailedServer   `json:"failed"`
	Summary  ImportSummary    `json:"summary"`
}

type ImportedServer struct {
	Server   MCPServerSummary `json:"server"`
	Warnings []string         `json:"import_warnings"`
}

type FailedServer struct {
	Name    string `json:"name"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type ImportSummary struct {
	Total    int `json:"total"`
	Imported int `json:"imported"`
	Failed   int `json:"failed"`
}

// MCPServerSummary 是 MCP Server 列表项的摘要信息。
type MCPServerSummary struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Description      *string            `json:"description"`
	Transport        string             `json:"transport"`
	URL              *string            `json:"url,omitempty"`
	HeadersMasked    *map[string]string `json:"headers_masked,omitempty"`
	Command          *string            `json:"command,omitempty"`
	Args             []string           `json:"args,omitempty"`
	EnvMasked        *map[string]string `json:"env_masked,omitempty"`
	AuthMasked       *string            `json:"auth_masked,omitempty"`
	ConnectTimeoutMs int                `json:"connect_timeout_ms"`
	CallTimeoutMs    int                `json:"call_timeout_ms"`
	MaxResponseBytes int                `json:"max_response_bytes"`
	Enabled          bool               `json:"enabled"`
	ConnectionStatus string             `json:"connection_status"`
	LastTestedAt     *time.Time         `json:"last_tested_at,omitempty"`
	LastError        *string            `json:"last_error,omitempty"`
	ToolsCount       int                `json:"tools_count"`
	Tools            []MCPToolSummary   `json:"tools,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
}

type MCPToolSummary struct {
	ID          string  `json:"id"`
	ToolName    string  `json:"tool_name"`
	Description *string `json:"description"`
	ReadOnly    bool    `json:"read_only"`
	Enabled     bool    `json:"enabled"`
}

// MCPServerDetailResponse 是获取单个 MCP Server 详情的响应。
type MCPServerDetailResponse struct {
	Server MCPServerSummary `json:"server"`
	Tools  []MCPToolDetail  `json:"tools"`
}

// MCPServerListResponse 是获取 MCP Server 列表的响应。
type MCPServerListResponse struct {
	Servers []MCPServerSummary `json:"servers"`
}

// MCPTestResult 是连接测试的结果。
type MCPTestResult struct {
	Success        bool      `json:"success"`
	Available      bool      `json:"available"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	LastTestedAt   time.Time `json:"last_tested_at"`
}

// MCPDiscoverResult 是工具发现的结果。
type MCPDiscoverResult struct {
	Tools    []MCPToolSummary `json:"tools"`
	Warnings []string         `json:"warnings"`
}

type MCPToolDetail struct {
	ID              string          `json:"id"`
	ToolName        string          `json:"tool_name"`
	Description     *string         `json:"description"`
	InputSchema     json.RawMessage `json:"input_schema"`
	SchemaHash      string          `json:"schema_hash"`
	ReadOnly        bool            `json:"read_only"`
	Enabled         bool            `json:"enabled"`
	DiscoveredAt    time.Time       `json:"discovered_at"`
	SchemaChangedAt *time.Time      `json:"schema_changed_at,omitempty"`
}

// ConvertToServerSummary 从 Entity 转换为 MCPServerSummary。
func ConvertToServerSummary(server *entity.MCPServer, tools []MCPToolSummary, warnings []string) MCPServerSummary {
	toolSummaries := make([]MCPToolSummary, 0, len(tools))
	toolSummaries = append(toolSummaries, tools...)
	maskedHeaders := make(map[string]string)
	maskedEnv := make(map[string]string)
	if server.Transport == "streamable_http" {
		if server.AuthMasked != nil {
			maskedHeaders["Authorization"] = *server.AuthMasked
		} else {
			maskedHeaders["Authorization"] = "******"
		}
	} else {
		maskedEnv["GRAPHLIT_ENVIRONMENT_ID"] = "******"
		maskedEnv["GRAPHLIT_JWT_SECRET"] = "******"
	}
	return MCPServerSummary{
		ID:               server.ID,
		Name:             server.Name,
		Description:      server.Description,
		Transport:        server.Transport,
		URL:              server.URL,
		HeadersMasked:    &maskedHeaders,
		Command:          server.Command,
		Args:             nil,
		EnvMasked:        &maskedEnv,
		AuthMasked:       server.AuthMasked,
		ConnectTimeoutMs: server.ConnectTimeoutMs,
		CallTimeoutMs:    server.CallTimeoutMs,
		MaxResponseBytes: server.MaxResponseBytes,
		Enabled:          server.Enabled,
		ConnectionStatus: server.ConnectionStatus,
		LastTestedAt:     server.LastTestedAt,
		LastError:        server.LastError,
		ToolsCount:       len(tools),
		Tools:            toolSummaries,
		CreatedAt:        server.CreatedAt,
	}
}

// ConvertToToolSummary 从 Entity 转换为 MCPToolSummary。
func ConvertToToolSummary(tool *entity.MCPTool) MCPToolSummary {
	return MCPToolSummary{
		ID:          tool.ID,
		ToolName:    tool.ToolName,
		Description: tool.Description,
		ReadOnly:    tool.ReadOnly,
		Enabled:     tool.Enabled,
	}
}

// ConvertToToolDetail 从 Entity 转换为 MCPToolDetail。
func ConvertToToolDetail(tool *entity.MCPTool) MCPToolDetail {
	return MCPToolDetail{
		ID:              tool.ID,
		ToolName:        tool.ToolName,
		Description:     tool.Description,
		InputSchema:     tool.InputSchema,
		SchemaHash:      tool.SchemaHash,
		ReadOnly:        tool.ReadOnly,
		Enabled:         tool.Enabled,
		DiscoveredAt:    tool.DiscoveredAt,
		SchemaChangedAt: tool.SchemaChangedAt,
	}
}
