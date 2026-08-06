package entity

import "time"

// MCPTool 对应 mcp_tools 表，存储从 MCP Server 发现的工具元数据。
type MCPTool struct {
	ID              string     `gorm:"type:uuid;primaryKey" json:"id"`
	ServerID        string     `gorm:"column:server_id;not null" json:"server_id"`
	ToolName        string     `gorm:"column:tool_name;not null" json:"tool_name"`
	Description     *string    `gorm:"column:description" json:"description"`
	InputSchema     []byte     `gorm:"column:input_schema;not null" json:"input_schema"`
	SchemaHash      string     `gorm:"column:schema_hash;not null" json:"schema_hash"`
	ReadOnly        bool       `gorm:"column:read_only;not null" json:"read_only"`
	Enabled         bool       `gorm:"column:enabled;not null;default:false" json:"enabled"`
	DiscoveredAt    time.Time  `gorm:"column:discovered_at;not null" json:"discovered_at"`
	LastCheckedAt   time.Time  `gorm:"column:last_checked_at;not null" json:"last_checked_at"`
	SchemaChangedAt *time.Time `gorm:"column:schema_changed_at" json:"schema_changed_at,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (MCPTool) TableName() string { return "mcp_tools" }
