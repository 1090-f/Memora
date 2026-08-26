package entity

import "time"

// MCPServer 对应 mcp_servers 表，存储用户导入的 MCP Server 配置，
// 支持 streamable_http 与 stdio 两种传输方式。
type MCPServer struct {
	BaseEntity
	UserID            string     `gorm:"column:user_id;not null" json:"user_id"`
	Name              string     `gorm:"column:name;not null" json:"name"`
	Description       *string    `gorm:"column:description" json:"description"`
	Transport         string     `gorm:"column:transport;not null;default:streamable_http" json:"transport"`
	URL               *string    `gorm:"column:url" json:"url"`
	Command           *string    `gorm:"column:command" json:"command"`
	CWD               *string    `gorm:"column:cwd" json:"cwd,omitempty"`
	HeadersCiphertext []byte     `gorm:"column:headers_ciphertext" json:"-"`
	ArgsCiphertext    []byte     `gorm:"column:args_ciphertext" json:"-"`
	EnvCiphertext     []byte     `gorm:"column:env_ciphertext" json:"-"`
	AuthCiphertext    []byte     `gorm:"column:auth_ciphertext" json:"-"`
	AuthMasked        *string    `gorm:"column:auth_masked" json:"auth_masked,omitempty"`
	ConnectTimeoutMs  int        `gorm:"column:connect_timeout_ms;not null;default:30000" json:"connect_timeout_ms"`
	CallTimeoutMs     int        `gorm:"column:call_timeout_ms;not null;default:120000" json:"call_timeout_ms"`
	MaxResponseBytes  int        `gorm:"column:max_response_bytes;not null;default:1048576" json:"max_response_bytes"`
	NetworkRequired   bool       `gorm:"column:network_required;not null;default:false" json:"network_required"`
	Enabled           bool       `gorm:"column:enabled;not null;default:true" json:"enabled"`
	ConnectionStatus  string     `gorm:"column:connection_status;not null;default:unknown" json:"connection_status"`
	LastTestedAt      *time.Time `gorm:"column:last_tested_at" json:"last_tested_at,omitempty"`
	LastError         *string    `gorm:"column:last_error" json:"last_error,omitempty"`
}

func (MCPServer) TableName() string { return "mcp_servers" }
