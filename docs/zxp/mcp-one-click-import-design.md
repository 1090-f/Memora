# MCP 一键导入工具服务技术方案

## 1. 功能概述

### 1.1 目标

实现类似 Trae IDE 的 MCP Server 一键导入功能：用户粘贴一段 JSON 配置，系统自动完成校验、存储、连接测试和工具发现，即可使用对应的 MCP 工具。

### 1.2 核心流程

```
用户粘贴 JSON → 校验格式与安全性 → 查重 → 加密凭证 → 写入 DB → 连接测试 → 工具发现 → 返回导入结果
```

### 1.3 设计约束

- 支持 **Streamable HTTP** 与 **stdio** 两种传输（P0 阶段同时支持，`transport` 字段区分）。
- 导入**只能通过一段 JSON 内容**完成（Trae 风格 `mcpServers` 对象），不接受逐字段表单提交。
- 导入的 MCP 工具仅允许 **只读** 调用（复用现有 P0 安全策略）。
- JSON 校验失败不写入数据库，连接失败允许后续重试。
- 一个用户下同名 MCP Server 不允许重复导入。
- 凭证（API Key / Authorization Token）必须加密存储，日志禁止明文记录。
- 导入接口限制请求体大小（最大 64KB），防止滥用。

---

## 2. JSON Schema 设计

### 2.1 支持的输入格式

**只能通过一段 JSON 内容导入**，顶层固定为 `mcpServers` 对象，键为 server 名称，值为 server 配置。一次可导入一个或多个 server（Trae / Claude Desktop 兼容格式）。

**HTTP server 示例**：

```json
{
  "mcpServers": {
    "Google Maps": {
      "url": "https://mcp.example.com/mcp",
      "transport": "streamable_http",
      "headers": {
        "Authorization": "Bearer sk-xxxxx",
        "X-Custom": "value"
      },
      "description": "地图查询服务"
    }
  }
}
```

**stdio server 示例**（用户给出的格式）：

```json
{
  "mcpServers": {
    "Graphlit": {
      "command": "npx",
      "args": [
        "-y",
        "graphlit-mcp-server"
      ],
      "env": {
        "GRAPHLIT_ENVIRONMENT_ID": "",
        "GRAPHLIT_JWT_SECRET": "",
        "GRAPHLIT_ORGANIZATION_ID": ""
      }
    }
  }
}
```

**一次导入多个 server**：

```json
{
  "mcpServers": {
    "Graphlit": {
      "command": "npx",
      "args": ["-y", "graphlit-mcp-server"]
    },
    "Google Maps": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-google-maps"],
      "env": { "GOOGLE_MAPS_API_KEY": "" }
    }
  }
}
```

### 2.2 传输类型判定与字段校验规则

**传输类型判定**：优先校验 `transport` 字段，缺省时**看是否有 `command`**——有 `command` 判为 stdio，否则判为 streamable_http。

| 字段 | 类型 | 适用传输 | 必填 | 校验规则 |
|---|---|---|---|---|
| `command` | string | stdio | 是（stdio 时） | 非空，可执行命令或包名 |
| `args` | array[string] | stdio | 否 | 命令参数列表 |
| `env` | array[object] | stdio | 否 | 环境变量，Key/Value 均为字符串，最多 20 项（兼容 Trae 结构化格式） |
| `cwd` | string | stdio | 否 | 工作目录（默认不设置） |
| `url` | string | http | 是（http 时） | 合法 HTTPS URL（http 仅允许 localhost debug 模式） |
| `transport` | string | 两者 | 否 | 缺省时按有无 `command` 判定；显式时仅接受 `streamable_http` / `stdio` |
| `headers` | map | http | 否 | Key/Value 均为字符串，最多 20 个 header |
| `description` | string | 两者 | 否 | 最大 500 字符 |
| `connect_timeout_ms` | int | http | 否 | 默认 5000，范围 1000-30000 |
| `call_timeout_ms` | int | 两者 | 否 | 默认 30000，范围 5000-120000 |
| `max_response_bytes` | int | 两者 | 否 | 默认 1048576（1MB），范围 1024-10485760 |
| `enabled` | bool | 两者 | 否 | 默认 true |

**结构化 `env` 兼容两种写法**（Trae 的 `array[object]` 与 Claude Desktop 的 `object`）都会在内部归一化为 `map[string]string` 后加密存储：

```json
"env": [{ "NAME": "value" }]   // Trae 风格，array of { name, value }
```

### 2.3 URL 安全校验（http 传输）

- 只允许 HTTPS（debug 模式允许 localhost HTTP）。
- 禁止 `file://`、`unix://` 等非 HTTP 协议。
- 禁止内网地址（10.x.x.x、172.16-31.x.x、192.168.x.x、127.x.x.x、`0.0.0.0`）。
- 禁止云元数据地址（169.254.169.254）。
- 对重定向目标重新校验，防止 SSRF 攻击。

### 2.4 stdio 命令安全校验

- `command` 与 `args` 中禁止 shell 元字符与命令注入（如 `;`、`&&`、`|`、`$()`、反引号、`>`、`<`、换行）。
- `env` 值禁止包含密钥明文示警与换行回显。
- 进程类 server 采用白名单命令约束（配置内维护受信任命令列表），白名单外命令拒绝导入。
- 子进程通过专用 Worker 管理，设置启动超时、内存上限、输出字节上限与强制 kill（见 5.2.5）。

---

## 3. API 设计

### 3.1 接口总览

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/mcp/servers/import` | 导入 MCP Server（JSON 内容，`mcpServers` 含多个即批量导入） |
| GET | `/api/v1/mcp/servers` | 获取已导入的 MCP Server 列表 |
| GET | `/api/v1/mcp/servers/:id` | 获取单个 MCP Server 详情及工具列表 |
| DELETE | `/api/v1/mcp/servers/:id` | 删除 MCP Server |
| POST | `/api/v1/mcp/servers/:id/test` | 测试连接 |
| POST | `/api/v1/mcp/servers/:id/discover` | 手动触发工具发现 |
| PATCH | `/api/v1/mcp/tools/:id` | 更新 MCP 工具启用/停用状态 |

> 说明：不再区分「单条导入」与「批量导入」两个接口。统一由 `POST /import` 接收整段 `mcpServers` JSON，对象内含多个 server 时逐个处理，单个失败不影响其他。

### 3.2 导入接口详细设计

**POST `/api/v1/mcp/servers/import`**

Request Header：

```
Content-Type: application/json
Content-Length: < 65536
```

Request Body（用户粘贴整段 JSON，含 `mcpServers` 顶层对象）：

```json
{
  "mcpServers": {
    "Graphlit": {
      "command": "npx",
      "args": ["-y", "graphlit-mcp-server"],
      "env": {
        "GRAPHLIT_ENVIRONMENT_ID": "env-id",
        "GRAPHLIT_JWT_SECRET": "sk-xxxxx",
        "GRAPHLIT_ORGANIZATION_ID": "org-id"
      }
    },
    "Google Maps": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-google-maps"],
      "env": { "GOOGLE_MAPS_API_KEY": "key-xxxx" }
    }
  }
}
```

Response（200 OK）—— 返回 `mcpServers` 中每个 server 的导入结果：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "imported": [
      {
        "server": {
          "id": "uuid-graphlit",
          "name": "Graphlit",
          "description": "",
          "transport": "stdio",
          "command": "npx",
          "args": ["-y", "graphlit-mcp-server"],
          "env_masked": { "GRAPHLIT_ENVIRONMENT_ID": "******", "GRAPHLIT_JWT_SECRET": "******" },
          "connect_timeout_ms": 5000,
          "call_timeout_ms": 30000,
          "enabled": true,
          "connection_status": "available",
          "tools_count": 3,
          "tools": [
            {
              "id": "tool-uuid",
              "tool_name": "graphlit_query",
              "description": "查询 Graphlit 知识库",
              "read_only": true,
              "enabled": false
            }
          ],
          "created_at": "2026-08-04T10:00:00Z"
        },
        "import_warnings": []
      }
    ],
    "failed": [],
    "summary": {
      "total": 1,
      "imported": 1,
      "failed": 0
    }
  },
  "request_id": "req-uuid"
}
```

> HTTP server 的响应字段为 `url` + `headers_masked`；stdio server 为 `command` + `args` + `env_masked`。二者按 `transport` 区分配置输出。

Response（409 Conflict — 同名重复）：

```json
{
  "code": "DUPLICATE_RESOURCE",
  "message": "duplicate resource",
  "details": { "conflict_field": "name", "existing_server_id": "uuid" },
  "request_id": "req-uuid"
}
```

Response（422 — JSON 校验失败）：

```json
{
  "code": "INVALID_ARGUMENT",
  "message": "invalid argument",
  "details": { "validation_errors": ["top-level key 'mcpServers' required", "Graphlit: command not in whitelist", "Google Maps: url is required"] },
  "request_id": "req-uuid"
}
```

> 批量语义：`mcpServers` 含多个 server 时，合法的进入 `imported`（单个成功），不合法的进入 `failed`（附原因），单个失败不影响其他。全部失败时整体返回 422 或部分收发于 `failed` 列表，见 3.3。

### 3.3 批量结果说明

`POST /api/v1/mcp/servers/import` 天然支持批量：`mcpServers` 对象含几个 server 就处理几个。

Response 结构：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "imported": [ ... ],
    "failed": [
      {
        "name": "Web Reader",
        "error": "DUPLICATE_RESOURCE",
        "message": "server name already exists"
      }
    ],
    "summary": {
      "total": 3,
      "imported": 2,
      "failed": 1
    }
  },
  "request_id": "req-uuid"
}
```

批量采用逐个处理策略，单个失败不影响其他 server 导入。

---

## 4. 数据模型设计

### 4.1 复用现有表结构 + 新增 stdio 支持迁移

基础表复用 `000006_memory_mcp.up.sql` 中已定义的三张表：

- `mcp_servers` — MCP Server 配置
- `mcp_tools` — MCP 工具元数据
- `agent_mcp_tools` — Agent 工具授权关系

**但现有 `mcp_servers` 表存在限制**：`transport` 仅有 `CHECK (transport = 'streamable_http')`、`url NOT NULL`，无 stdio 字段。需新增迁移 `000007_mcp_stdio.up.sql`：

```sql
-- 000007_mcp_stdio.up.sql
-- 1. 扩展 transport 枚举，允许 stdio
ALTER TABLE mcp_servers
    DROP CONSTRAINT IF EXISTS mcp_servers_transport_check;

ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_transport_check
    CHECK (transport IN ('streamable_http', 'stdio'));

-- 2. url 允许为空（stdio server 无 url）
ALTER TABLE mcp_servers
    ALTER COLUMN url DROP NOT NULL;

-- 3. 新增 stdio 字段（command/args/env 整体加密存储）
ALTER TABLE mcp_servers
    ADD COLUMN command varchar(255),
    ADD COLUMN args_ciphertext bytea,
    ADD COLUMN env_ciphertext bytea;
```

> stdio 的 `args` 与 `env` 与 headers 一样整体 AES-256-GCM 加密后存 `args_ciphertext` / `env_ciphertext`，防止密钥泄露到数据库明文。

### 4.2 Entity 映射

**`internal/model/entity/mcp_server.go`**：

```go
package entity

import "time"

type MCPServer struct {
	BaseEntity
	UserID            string     `gorm:"column:user_id;not null" json:"user_id"`
	Name              string     `gorm:"column:name;not null" json:"name"`
	Description       *string    `gorm:"column:description" json:"description"`
	Transport         string     `gorm:"column:transport;not null;default:streamable_http" json:"transport"`
	URL               *string    `gorm:"column:url" json:"url"`
	Command           *string    `gorm:"column:command" json:"command"`
	HeadersCiphertext []byte     `gorm:"column:headers_ciphertext" json:"-"`
	ArgsCiphertext    []byte     `gorm:"column:args_ciphertext" json:"-"`
	EnvCiphertext     []byte     `gorm:"column:env_ciphertext" json:"-"`
	AuthCiphertext    []byte     `gorm:"column:auth_ciphertext" json:"-"`
	AuthMasked        *string    `gorm:"column:auth_masked" json:"auth_masked,omitempty"`
	ConnectTimeoutMs  int        `gorm:"column:connect_timeout_ms;not null;default:5000" json:"connect_timeout_ms"`
	CallTimeoutMs     int        `gorm:"column:call_timeout_ms;not null;default:30000" json:"call_timeout_ms"`
	MaxResponseBytes  int        `gorm:"column:max_response_bytes;not null;default:1048576" json:"max_response_bytes"`
	Enabled           bool       `gorm:"column:enabled;not null;default:true" json:"enabled"`
	ConnectionStatus  string     `gorm:"column:connection_status;not null;default:unknown" json:"connection_status"`
	LastTestedAt      *time.Time `gorm:"column:last_tested_at" json:"last_tested_at,omitempty"`
	LastError         *string    `gorm:"column:last_error" json:"last_error,omitempty"`
}

func (MCPServer) TableName() string { return "mcp_servers" }
```

**`internal/model/entity/mcp_tool.go`**：

```go
package entity

import "time"

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
```

---

## 5. 服务架构设计

### 5.1 导入流程

```
用户提交 JSON（mcpServers 对象）
    │
    ▼
MCPImportService.Import(ctx, userID, jsonConfig)
    │
    ├─── 1. JSON 解析 + Schema 校验
    │       ├── 顶层必须是 mcpServers 对象
    │       ├── 每个 server 判定传输类型（transport / command 有无）
    │       ├── 字段类型、必填、长度、范围校验
    │       ├── http：URL 格式与安全性校验（SSRF 防护）
    │       └── stdio：command 白名单与注入字符校验
    │
    ├─── 2. 逐个 server 同名查重
    │       └── DB: SELECT ... WHERE user_id = ? AND lower(name) = lower(?)
    │
    ├─── 3. 凭证加密
    │       └── AES-256-GCM 加密 headers / args / env 中的敏感值
    │
    ├─── 4. 写入 mcp_servers 表
    │       └── INSERT INTO mcp_servers（transport=streamable_http|stdio）
    │
    ├─── 5. 连接测试 + 工具发现（可选，并行执行）
    │       ├── http：MCP Client → POST /initialize + /tools/list
    │       └── stdio：拉起子进程 → initialize + /tools/list → 优雅退出
    │
    ├─── 6. 写入 mcp_tools 表
    │       └── 批量 INSERT INTO mcp_tools
    │
    └─── 7. 返回导入结果
            └── 每个 server 的导入结果 + tools 列表 + warnings
```

> `mcpServers` 含多个 server 时循环执行 2–6，单个失败记入 `failed` 列表，不影响其他 server。

### 5.2 核心模块

#### 5.2.1 Service 层

**`internal/service/mcp_import_service.go`**（接口）：

```go
type MCPImportService interface {
    Import(ctx context.Context, userID string, req *request.MCPImportRequest) (*response.MCPImportResponse, error)
    BatchImport(ctx context.Context, userID string, req *request.MCPBatchImportRequest) (*response.MCPBatchImportResponse, error)
    List(ctx context.Context, userID string) ([]response.MCPServerSummary, error)
    GetDetail(ctx context.Context, userID string, serverID string) (*response.MCPServerDetailResponse, error)
    Delete(ctx context.Context, userID string, serverID string) error
    TestConnection(ctx context.Context, userID string, serverID string) (*response.MCPTestResult, error)
    DiscoverTools(ctx context.Context, userID string, serverID string) (*response.MCPDiscoverResult, error)
    UpdateToolStatus(ctx context.Context, userID string, toolID string, enabled bool) error
}
```

#### 5.2.2 Repository 层

**`internal/repository/mcp_repository.go`**（接口）：

```go
type MCPServerRepository interface {
    FindActiveByName(ctx context.Context, userID, name string) (*entity.MCPServer, error)
    FindActiveByID(ctx context.Context, userID, serverID string) (*entity.MCPServer, error)
    Create(ctx context.Context, server *entity.MCPServer) error
    UpdateStatus(ctx context.Context, serverID, status string, lastErr *string) error
    Delete(ctx context.Context, userID, serverID string) error
    ListByUser(ctx context.Context, userID string) ([]entity.MCPServer, error)
}

type MCPToolRepository interface {
    BatchCreate(ctx context.Context, tools []entity.MCPTool) error
    FindByServer(ctx context.Context, serverID string) ([]entity.MCPTool, error)
    UpdateEnabled(ctx context.Context, toolID string, enabled bool) error
    UpdateSchema(ctx context.Context, toolID, schemaHash string, schema []byte) error
    DeleteByServer(ctx context.Context, serverID string) error
}
```

#### 5.2.3 MCP 客户端

**`internal/mcp/client.go`**：

```go
type MCPClient interface {
    Initialize(ctx context.Context, target MCPServerTarget, timeout time.Duration) error
    ListTools(ctx context.Context, target MCPServerTarget, timeout time.Duration) ([]MCPServerTool, error)
    CallTool(ctx context.Context, target MCPServerTarget, toolName string, arguments json.RawMessage, timeout time.Duration) (json.RawMessage, error)
}

// MCPServerTarget 统一描述两种传输的目标：
// - HTTP：URL + Headers
// - stdio：Command + Args + Env
type MCPServerTarget struct {
    Transport string            `json:"transport"` // streamable_http | stdio
    URL       string            `json:"url,omitempty"`
    Headers   map[string]string `json:"headers,omitempty"`
    Command   string            `json:"command,omitempty"`
    Args      []string          `json:"args,omitempty"`
    Env       map[string]string `json:"env,omitempty"`
}

type MCPServerTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}
```

#### 5.2.4 stdio 进程管理（[stdio.go](../internal/mcp/stdio.go)）

stdio server 通过专用 Worker 进程户管理，禁止在主进程直接 `exec.Command` 长期运行：

```go
type StdioProcess struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser

    cfg struct {
        StartTimeout   time.Duration // 默认 5s
        MaxOutputBytes int64         // 默认 1MB，超过即截断
        MaxMemoryKB    int64         // Linux cgroup 内存上限
        KillTimeout    time.Duration // kill 后强杀等待，默认 2s
    }
}

func Start(target MCPServerTarget, cfg StdioProcessConfig) (*StdioProcess, error)
func (p *StdioProcess) Request(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
func (p *StdioProcess) Close() error  // 先发 exit 通知，超时后强杀 SIGKILL
```

- 每次工具调用前按需拉起子进程，调用结束 `Close()` 回收，避免进程泄漏。
- 输出用 size-limited buffer 读取，防止子进程无限输出耗尽内存。
- 命令白名单（配置内维护，如 `npx`、`python`、`uvx`），白名单外拒绝。
- 进程以非交互、无 shell 方式启动（`exec.Cmd` 直接传 argv，不拼 shell 字符串）。

#### 5.2.6 加密与安全

**`internal/mcp/crypto.go`**：

```go
func Encrypt(data []byte, key []byte) ([]byte, error)          // 任一字段整体加密
func Decrypt(ciphertext []byte, key []byte) ([]byte, error)
func EncryptStringMap(m map[string]string, key []byte) ([]byte, error) // headers / env
func DecryptStringMap(ciphertext []byte, key []byte) (map[string]string, error)
func MaskEnv(env map[string]string) map[string]string           // 仅返回 Key，值隐藏
func MaskHeaders(headers map[string]string) map[string]string
```

**`internal/mcp/security.go`**：

```go
func ValidateURL(rawURL string, allowHTTP bool) error
func IsPrivateIP(host string) bool
```

---

## 6. 开发任务分解

按照 [DEVELOPMENT.md](../DEVELOPMENT.md) 规范，分层开发，每个 Task 为独立可验收单元：

### Task 0: 数据库迁移（stdio 支持）

| 项目 | 内容 |
|---|---|
| 文件变更 | 创建 `scripts/migrations/000007_mcp_stdio.up.sql`，创建 `scripts/migrations/000007_mcp_stdio.down.sql` |
| 依赖 | 无 |
| 产出 | `mcp_servers` 表支持 `transport IN ('streamable_http','stdio')`、`url` 可空、新增 `command`/`args_ciphertext`/`env_ciphertext` 列 |
| 验收 | up/down 迁移可逆；旧数据兼容（http server 不受影响） |

### Task 1: Entity 定义

| 项目 | 内容 |
|---|---|
| 文件变更 | 创建 `internal/model/entity/mcp_server.go`，创建 `internal/model/entity/mcp_tool.go` |
| 依赖 | Task 0 |
| 产出 | `MCPServer`、`MCPTool` 结构体，含 `Command`/`ArgsCiphertext`/`EnvCiphertext` 字段 |
| 验收 | 编译通过，字段与迁移后数据库表一一对应 |

### Task 2: DTO 定义

| 项目 | 内容 |
|---|---|
| 文件变更 | 创建 `internal/model/dto/request/mcp.go`，创建 `internal/model/dto/response/mcp.go` |
| 依赖 | Task 1 |
| 产出 | `MCPImportRequest`（顶层 `mcpServers` map）、`MCPServerConfig`（transport/url/headers/command/args/env）、`MCPImportResponse`、`MCPServerSummary`、`MCPServerDetailResponse`、`MCPTestResult`、`MCPDiscoverResult` |
| 验收 | 编译通过，JSON tag 与 API 设计一致（含 `command`/`args`/`env`） |

### Task 3: Repository 接口与实现

| 项目 | 内容 |
|---|---|
| 文件变更 | 创建 `internal/repository/mcp_repository.go`（接口），创建 `internal/repository/mcp_server_repository.go`（实现），创建 `internal/repository/mcp_tool_repository.go`（实现） |
| 依赖 | Task 1, Task 2 |
| 产出 | MCPServerRepository、MCPToolRepository 完整实现 |
| 验收 | 单元测试覆盖核心查询路径 |

### Task 4: MCP 客户端与安全模块

| 项目 | 内容 |
|---|---|
| 文件变更 | 创建 `internal/mcp/client.go`，创建 `internal/mcp/crypto.go`，创建 `internal/mcp/security.go`，创建 `internal/mcp/stdio.go` |
| 依赖 | Task 2 |
| 产出 | MCPClient 接口实现（`MCPServerTarget` 统一 HTTP/stdio）、URL 安全校验、headers/args/env 加解密、stdio 子进程管理 |
| 验收 | URL 安全校验覆盖全部禁止场景；加解密可逆；SSRF 防护测试通过；stdio 白名单与注入校验测试通过 |

### Task 5: Service 层

| 项目 | 内容 |
|---|---|
| 文件变更 | 创建 `internal/service/mcp_import_service.go`（接口），创建 `internal/service/mcp_import_service_impl.go`（实现） |
| 依赖 | Task 3, Task 4 |
| 产出 | MCPImportService 完整业务逻辑（解析 `mcpServers` → 判定传输 → 校验 → 查重 → 加密 → 入库 → 测试/发现） |
| 验收 | 单元测试覆盖导入、查重、加密、工具发现路径，含 http/stdio 两种传输 |

### Task 6: Controller 与 Routes

| 项目 | 内容 |
|---|---|
| 文件变更 | 创建 `internal/api/v1/mcp/controller.go`，创建 `internal/api/v1/mcp/routes.go` |
| 依赖 | Task 5 |
| 产出 | HTTP Handler + 路由注册（`POST /servers/import` 统一单/批量） |
| 验收 | 编译通过，路由与 API 设计一致 |

### Task 7: Router 挂载与依赖注入

| 项目 | 内容 |
|---|---|
| 文件变更 | 修改 `internal/api/router.go`，修改 `internal/app/server.go` |
| 依赖 | Task 6 |
| 产出 | MCP 模块接入系统 |
| 验收 | API 服务启动后 `/api/v1/mcp/servers/import` 可访问 |

### Task 8: 错误码补充

| 项目 | 内容 |
|---|---|
| 文件变更 | 修改 `internal/contracts/errors.go`，修改 `pkg/errors/code.go` |
| 依赖 | 无 |
| 产出 | `ErrMCPImportFailed`、`ErrMCPConnectionFailed`、`ErrMCPDiscoveryFailed` 等错误码 |
| 验收 | 编译通过，错误码与 HTTP 状态码映射正确 |

---

## 7. 安全设计

### 7.1 凭证管理

| 环节 | 策略 |
|---|---|
| 传输 | 接口强制 HTTPS |
| 存储 | headers 中的 `Authorization`、`Api-Key` 及 stdio 的 `env` 中敏感值使用 AES-256-GCM 加密后存入 `headers_ciphertext` / `env_ciphertext`；`args` 整体加密存入 `args_ciphertext` |
| 展示 | API 响应中只返回脱敏值（如 `Bearer sk-xxxx`、env 仅返回 Key） |
| 日志 | 禁止记录 headers/env 明文，仅记录脱敏摘要 |

### 7.2 SSRF 防护（http 传输）

- URL 白名单：仅允许 HTTPS 公网地址。
- 内网地址黑名单：10.0.0.0/8、172.16.0.0/12、192.168.0.0/16、127.0.0.0/8、169.254.0.0/16。
- 重定向校验：对每次 302/301 跳转目标重新执行地址检查。
- 连接超时：默认 5s，最大 30s，防止长连接攻击。

### 7.3 stdio 进程安全（stdio 传输）

- 命令白名单：`command` 必须在受信任命令列表内（配置维护，如 `npx`、`python`、`uvx`）。
- 注入防护：`command`/`args` 禁止 shell 元字符与换行；不使用 shell 拼接。
- 资源限制：启动超时（5s）、输出上限（1MB）、内存上限、强制 kill。
- 进程隔离：以专用 Worker 拉起，调用结束即回收，禁止常驻。

### 7.4 调用安全

- 只允许启用 `read_only = true` 的工具（复用现有策略）。
- 非只读工具自动发现后 `enabled = false`，需用户手动启用（P0 不允许写工具启用）。
- 单次工具调用限制：超时（`call_timeout_ms`）、返回大小（`max_response_bytes`）。

---

## 8. 与 Trae 的兼容性说明

Trae 等 IDE 的 MCP Server JSON 格式：

```json
{
  "mcpServers": {
    "server-name": {
      "url": "https://...",
      "headers": { "Authorization": "Bearer xxx" }
    }
  }
}
```

**兼容策略：**

1. **导入入口唯一**：只接受顶层为 `mcpServers` 对象的 JSON 内容，直接兼容 Trae / Claude Desktop / Cursor 导出的配置片段。
2. **transport 自动判定**：server 值含 `url`/`headers` 判为 Streamable HTTP；含 `command`/`args`/`env` 判为 stdio；两者兼备时以显式 `transport` 为准。
3. **stdio 原生支持**：`command` + `args` + `env`（含 `mcpServers` 键值对格式）完整解析、加密存储并支持连接测试与工具发现。
4. **不支持的格式**：无 `mcpServers` 顶层（如 `{ "servers": [...] }`、裸 server 对象）返回明确的 422 错误并提示期望格式。

## 9. 验收标准

1. 导入：粘贴 `mcpServers` 对象（单个或多个）→ 每个 server 返回结果 + 工具列表 → 数据库写入正确。
2. HTTP server：`url` + `headers` → 连接测试成功 → `mcp_tools` 数据正确。
3. stdio server：`command` + `args` + `env`（Graphlit / Google Maps 示例）→ 拉起子进程发现工具 → 结果正确。
4. 混合导入：同一 JSON 中 http 与 stdio 并存 → 各自正确导入。
5. 重复导入：同名 server → 返回 409 错误。
6. 格式校验：非 `mcpServers` 顶层 / 无效 JSON → 返回 422 错误。
7. URL 校验：内网地址 → 422；stdio 命令不在白名单 → 422。
8. 连接测试：`POST /test` 返回连接状态和响应时间。
9. 工具启停：`PATCH /tools/:id` 正确更新 `enabled` 状态。
10. 删除：删除 server → 关联 tools 一并删除 → stdio 进程回收。
11. 安全：headers/env 加密存储 → API 不返回明文 → 日志不记录密钥。
12. 所有现有测试通过，无回归。

---

## 10. 后续扩展（P1 预留）

- stdio 进程常驻池与自动重启
- 工具调用人工确认和暂停恢复
- MCP Server 从 URL 批量抓取（自动发现 JSON 端点）
- MCP 工具版本监控和自动 Schema 更新通知
- 工具调用历史统计和性能分析
