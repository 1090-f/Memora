# Memora Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立可启动、可测试的 Memora API/Worker 基础框架，并交付认证、公共协议、依赖健康检查与显式数据库迁移。

**Architecture:** 使用一个 Go Module 构建 `memora-api`、`memora-worker` 和 `memora-migrate`。业务模块按 domain/application/ports/adapters 分层；PostgreSQL 是业务真相源，Redis 仅用于令牌失效和短期信号，MinIO 通过 Port 隔离。

**Tech Stack:** Go 1.26.5、Gin 1.12.0、GORM 1.31.1、PostgreSQL 17 + pgvector、Redis 7、MinIO、golang-migrate、JWT v5、Testify。

## Global Constraints

- Go Module 固定为 `github.com/1090-f/Memora`。
- HTTP 前缀固定为 `/api/v1`，JSON 使用 `snake_case`。
- API 响应固定包含 `code`、`message`、`data` 或 `details`、`request_id`。
- 主键使用 PostgreSQL `uuid DEFAULT gen_random_uuid()`；时间使用 `timestamptz`。
- P0 数据库最终严格保持当前数据库文档定义的 20 张核心表；不得增加 Session、Job、AgentEvent、Citation 或独立索引版本表。
- 普通 CRUD 使用 GORM；全文检索、pgvector、RRF 和并发锁使用参数化原生 SQL。
- 身份字段由后端注入，资源查询必须同时校验 `user_id`，知识库资源还要校验 `knowledge_base_id`。
- 生产环境禁止 GORM AutoMigrate，只执行版本化 SQL Migration。
- 测试遵循 TDD；每个任务完成后运行目标测试和 `go test ./...`，然后独立提交。
- 当前三份中文需求/API/数据库文档是实现事实来源；已提交架构文档仅提供不冲突的模块化指导。

## Delivery Roadmap

本计划只实现可独立验收的 Foundation。后续按顺序分别生成并执行以下计划：

1. Knowledge：知识库、目录、20 表剩余基础迁移、文档、导入与 MinIO。
2. RAG：解析、分段、Embedding、关键词/向量检索、RRF、Reranker、版本切换。
3. Context：Conversation、Message、Memory、ContextBuilder、Router。
4. Agent Core：AgentRun、预算、取消、ToolRegistry、ToolExecutor、SSE。
5. ReAct 与 Plan：两种执行引擎、Replan、Reviewer、Citation JSONB。
6. MCP：Server、发现、授权、只读调用、SSRF 和凭证加密。
7. E2E：两条主链、降级、性能、安全和发布验收。

---

### Task 1: Go Module、构建入口与版本信息

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `Makefile`
- Create: `cmd/memora-api/main.go`
- Create: `cmd/memora-worker/main.go`
- Create: `cmd/memora-migrate/main.go`
- Create: `internal/buildinfo/version.go`
- Test: `internal/buildinfo/version_test.go`

**Interfaces:**
- Produces: `buildinfo.Info() buildinfo.BuildInfo`，三个后续组合根均可调用。

- [ ] **Step 1: 写版本信息失败测试**

```go
func TestInfoHasServiceName(t *testing.T) {
    got := buildinfo.Info()
    require.Equal(t, "memora", got.Service)
    require.NotEmpty(t, got.Version)
}
```

- [ ] **Step 2: 初始化 Module 并验证测试失败**

Run: `go mod init github.com/1090-f/Memora`，添加 Testify 后运行 `go test ./internal/buildinfo -v`。  
Expected: FAIL，`buildinfo.Info` 尚未定义。

- [ ] **Step 3: 实现最小版本结构并创建三个可执行入口**

```go
type BuildInfo struct { Service, Version, Commit, BuiltAt string }

var Version = "dev"
var Commit = "unknown"
var BuiltAt = "unknown"

func Info() BuildInfo {
    return BuildInfo{Service: "memora", Version: Version, Commit: Commit, BuiltAt: BuiltAt}
}
```

三个 `main.go` 在本任务只输出 `buildinfo.Info()` 后正常退出，不引用尚未创建的组合根；Task 4、5、8 再分别接入 migrate、API 和 Worker。Makefile 提供 `test`、`lint`、`build` 三个目标。

- [ ] **Step 4: 验证构建**

Run: `go test ./...`  
Run: `go build ./cmd/memora-api ./cmd/memora-worker ./cmd/memora-migrate`  
Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add go.mod go.sum .gitignore Makefile cmd internal/buildinfo
git commit -m "build: initialize Memora Go services"
```

### Task 2: 强类型配置与启动校验

**Files:**
- Create: `internal/platform/config/config.go`
- Create: `internal/platform/config/load.go`
- Test: `internal/platform/config/load_test.go`
- Create: `.env.example`

**Interfaces:**
- Produces: `config.Load() (config.Config, error)`。
- Produces: `Config.Validate() error`。

- [ ] **Step 1: 写缺少密钥和数据库地址的失败测试**

```go
func TestLoadRejectsMissingRequiredValues(t *testing.T) {
    t.Setenv("MEMORA_DATABASE_URL", "")
    t.Setenv("MEMORA_JWT_SECRET", "")
    _, err := config.Load()
    require.ErrorContains(t, err, "MEMORA_DATABASE_URL")
}
```

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/platform/config -v`  
Expected: FAIL，配置包不存在。

- [ ] **Step 3: 实现配置结构**

```go
type Config struct {
    Environment string
    HTTP HTTPConfig
    Database DatabaseConfig
    Redis RedisConfig
    MinIO MinIOConfig
    Auth AuthConfig
}

type HTTPConfig struct { Address string; ReadTimeout, WriteTimeout, ShutdownTimeout time.Duration }
type DatabaseConfig struct { URL string; MaxOpen, MaxIdle int }
type RedisConfig struct { Address, Password string; DB int }
type MinIOConfig struct { Endpoint, AccessKey, SecretKey, Bucket string; UseSSL bool }
type AuthConfig struct { JWTSecret string; AccessTTL time.Duration }
```

使用 `os.LookupEnv` 和 `time.ParseDuration`，默认 HTTP 地址 `:8080`、AccessTTL `2h`。错误必须列出具体环境变量名；日志不得打印 Secret 值。

- [ ] **Step 4: 验证合法配置和错误配置**

Run: `go test ./internal/platform/config -v`  
Expected: PASS，覆盖默认值、无效 duration、缺少 URL、缺少 JWT Secret。

- [ ] **Step 5: 提交**

```bash
git add internal/platform/config .env.example
git commit -m "feat: add validated runtime configuration"
```

### Task 3: 公共 API 响应、错误码与请求 ID

**Files:**
- Create: `internal/contracts/common.go`
- Create: `internal/contracts/errors.go`
- Create: `internal/platform/httpx/response.go`
- Create: `internal/platform/httpx/request_id.go`
- Test: `internal/platform/httpx/response_test.go`

**Interfaces:**
- Produces: `contracts.AppError`、`contracts.ErrorCode`。
- Produces: `httpx.Success(c, status, data)`、`httpx.Failure(c, appError)`。
- Produces: `httpx.RequestID() gin.HandlerFunc`、`httpx.RequestIDFrom(ctx) string`。

- [ ] **Step 1: 写成功和错误响应测试**

```go
func TestSuccessEnvelope(t *testing.T) {
    c, recorder := newTestContext("req-123")
    httpx.Success(c, http.StatusOK, gin.H{"value": 1})
    require.JSONEq(t, `{"code":"OK","message":"success","data":{"value":1},"request_id":"req-123"}`, recorder.Body.String())
}
```

再增加错误测试，确认 `AppError{Code: ResourceNotFound}` 映射为 404，响应不包含内部 `Cause`。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/platform/httpx -v`  
Expected: FAIL，响应函数尚未定义。

- [ ] **Step 3: 实现协议**

```go
type AppError struct {
    Code ErrorCode
    Message string
    Details any
    Cause error
}

type Envelope struct {
    Code string `json:"code"`
    Message string `json:"message"`
    Data any `json:"data,omitempty"`
    Details any `json:"details,omitempty"`
    RequestID string `json:"request_id"`
}
```

请求 ID 优先接受合法 `X-Request-ID`，否则生成 UUID；写入 Gin Context 和响应 Header。实现 API 文档列出的 HTTP/错误码映射。

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/platform/httpx ./internal/contracts -v`  
Run: `go test ./...`  
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/contracts internal/platform/httpx
git commit -m "feat: define API envelopes and application errors"
```

### Task 4: PostgreSQL 连接、扩展与显式 Migration

**Files:**
- Create: `internal/platform/database/open.go`
- Create: `internal/platform/database/health.go`
- Create: `internal/platform/database/migrate.go`
- Test: `internal/platform/database/database_integration_test.go`
- Create: `migrations/000001_extensions.up.sql`
- Create: `migrations/000001_extensions.down.sql`
- Create: `migrations/000002_users.up.sql`
- Create: `migrations/000002_users.down.sql`
- Create: `deploy/docker-compose.dependencies.yml`

**Interfaces:**
- Produces: `database.Open(ctx, config.DatabaseConfig) (*gorm.DB, error)`。
- Produces: `database.Check(ctx, *gorm.DB) error`。
- Produces: `database.Migrate(ctx, databaseURL, direction) error`。

- [ ] **Step 1: 写带构建标签的集成测试**

```go
func TestMigrateCreatesExtensionsAndUsers(t *testing.T) {
    dbURL := requireDatabaseURL(t)
    require.NoError(t, database.Migrate(context.Background(), dbURL, database.Up))
    requireTable(t, dbURL, "users")
    requireExtension(t, dbURL, "vector")
    requireExtension(t, dbURL, "pgcrypto")
}
```

- [ ] **Step 2: 验证失败**

Run: `docker compose -f deploy/docker-compose.dependencies.yml up -d postgres`  
Run: `go test -tags=integration ./internal/platform/database -v`  
Expected: FAIL，Migration 尚不存在。

- [ ] **Step 3: 实现迁移与连接**

`000001` 只创建 `pgcrypto` 和 `vector`。`000002` 严格按 20 表数据库文档创建 `users`，主键为 `uuid DEFAULT gen_random_uuid()`，username/email 唯一，时间为 `timestamptz`。连接池应用 MaxOpen/MaxIdle，所有 Ping 使用超时 Context。

- [ ] **Step 4: 验证正反向迁移**

Run: `go test -tags=integration ./internal/platform/database -v`  
Run: `go test ./...`  
Expected: PASS；向下迁移删除 users，但不自动删除共享扩展。

- [ ] **Step 5: 提交**

```bash
git add internal/platform/database migrations cmd/memora-migrate deploy/docker-compose.dependencies.yml
git commit -m "feat: add PostgreSQL migrations and health checks"
```

### Task 5: Gin Router、恢复中间件与健康接口

**Files:**
- Create: `internal/app/api/app.go`
- Create: `internal/app/api/router.go`
- Create: `internal/app/api/server.go`
- Create: `internal/platform/httpx/recovery.go`
- Create: `internal/modules/system/adapters/http/health_handler.go`
- Test: `internal/app/api/router_test.go`

**Interfaces:**
- Produces: `api.New(deps api.Dependencies) (*api.App, error)`。
- Produces: `App.Run(ctx) error`。
- Produces: `GET /health/live` 与 `GET /health/ready`。

- [ ] **Step 1: 写路由测试**

```go
func TestLiveHealthUsesStandardEnvelope(t *testing.T) {
    router := newTestRouter(allDependenciesHealthy())
    response := performRequest(router, http.MethodGet, "/health/live")
    require.Equal(t, http.StatusOK, response.Code)
    require.Contains(t, response.Body.String(), `"code":"OK"`)
}
```

增加 readiness 失败返回 503、panic 恢复返回 INTERNAL_ERROR 且携带 request_id 的测试。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/app/api -v`  
Expected: FAIL，Router 尚未定义。

- [ ] **Step 3: 实现 Router 和 Server**

使用 `gin.New()`，按顺序注册 RequestID、结构化访问日志、Recovery。`/health/live` 不检查依赖；`/health/ready` 并发检查 PostgreSQL、Redis、MinIO，并为每项设置短超时。Server 使用显式 ReadHeaderTimeout、ReadTimeout、WriteTimeout 和优雅关闭。

- [ ] **Step 4: 验证**

Run: `go test ./internal/app/api ./internal/modules/system/... -v`  
Run: `go test ./...`  
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/app/api internal/modules/system internal/platform/httpx cmd/memora-api
git commit -m "feat: add HTTP server and health endpoints"
```

### Task 6: Redis、MinIO Ports 与基础适配器

**Files:**
- Create: `internal/platform/cache/client.go`
- Create: `internal/platform/objectstore/client.go`
- Create: `internal/platform/objectstore/port.go`
- Test: `internal/platform/cache/client_test.go`
- Test: `internal/platform/objectstore/client_test.go`

**Interfaces:**
- Produces: `cache.Open(config.RedisConfig) (*redis.Client, error)`。
- Produces: `objectstore.Store`，方法为 `Health(ctx) error`、`Put(ctx, key, reader, size, contentType) error`、`Get(ctx, key) (io.ReadCloser, error)`、`Delete(ctx, key) error`。

- [ ] **Step 1: 写配置和错误映射测试**

```go
func TestStoreRejectsEmptyObjectKey(t *testing.T) {
    store := objectstore.New(fakeMinIOClient{})
    err := store.Put(context.Background(), "", strings.NewReader("x"), 1, "text/plain")
    require.ErrorIs(t, err, objectstore.ErrInvalidKey)
}
```

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/platform/cache ./internal/platform/objectstore -v`  
Expected: FAIL，适配器尚未定义。

- [ ] **Step 3: 实现最小客户端**

Redis 客户端启动时 Ping；MinIO 启动时确认 Bucket 存在，不允许自动公开 Bucket。对象 Key 只能是应用生成的相对路径，拒绝空值、反斜杠和 `..` 路径段。

- [ ] **Step 4: 验证**

Run: `go test ./internal/platform/cache ./internal/platform/objectstore -v`  
Run: `go test ./...`  
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/platform/cache internal/platform/objectstore
git commit -m "feat: add Redis and object storage adapters"
```

### Task 7: 用户认证垂直切片

**Files:**
- Create: `internal/modules/identity/domain/user.go`
- Create: `internal/modules/identity/ports/user_repository.go`
- Create: `internal/modules/identity/application/auth_service.go`
- Create: `internal/modules/identity/adapters/persistence/user_repository.go`
- Create: `internal/modules/identity/adapters/http/auth_handler.go`
- Create: `internal/modules/identity/adapters/http/auth_middleware.go`
- Test: `internal/modules/identity/application/auth_service_test.go`
- Test: `internal/modules/identity/adapters/http/auth_handler_test.go`

**Interfaces:**
- Produces: `AuthService.Login(ctx, account, password) (LoginResult, error)`。
- Produces: `AuthService.Logout(ctx, tokenID, expiresAt) error`。
- Produces: `AuthRequired() gin.HandlerFunc`，向标准 Context 注入 UserID。

- [ ] **Step 1: 写登录失败和成功测试**

```go
func TestLoginRejectsWrongPassword(t *testing.T) {
    service := newAuthService(userWithPassword("admin@example.com", "correct"))
    _, err := service.Login(context.Background(), "admin@example.com", "wrong")
    require.ErrorIs(t, err, identity.ErrInvalidCredentials)
}
```

增加成功返回 `token_type=Bearer`、`expires_in=7200`，以及退出后 Token 被 Redis 黑名单拒绝的测试。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/modules/identity/... -v`  
Expected: FAIL，AuthService 尚未定义。

- [ ] **Step 3: 实现认证**

密码使用 Argon2id 参数化哈希；JWT 至少包含 `sub`、`jti`、`iat`、`exp`。Repository 按 username 或 email 查询未删除且 active 的用户。退出将 JTI 写入 Redis，TTL 等于 Token 剩余有效期；中间件同时验证签名、期限、用户状态和黑名单。

实现并注册：`POST /api/v1/auth/login`、`POST /api/v1/auth/logout`、`GET /api/v1/users/me`。响应字段严格匹配当前 API 文档。

- [ ] **Step 4: 验证认证边界**

Run: `go test ./internal/modules/identity/... -v`  
Run: `go test ./...`  
Expected: PASS，错误账号与错误密码均返回相同 UNAUTHORIZED，不泄露账号是否存在。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/identity internal/app/api
git commit -m "feat: add JWT user authentication"
```

### Task 8: Worker 生命周期、Compose 与 CI 门禁

**Files:**
- Create: `internal/app/worker/app.go`
- Create: `internal/app/worker/runner.go`
- Test: `internal/app/worker/app_test.go`
- Create: `deploy/docker-compose.yml`
- Create: `deploy/postgres/init.sql`
- Create: `Dockerfile`
- Create: `.github/workflows/ci.yml`
- Modify: `README.md`

**Interfaces:**
- Produces: `worker.New(deps worker.Dependencies) *worker.App`。
- Produces: `App.Run(ctx) error`，收到取消后等待运行任务结束并关闭依赖。

- [ ] **Step 1: 写 Worker 取消测试**

```go
func TestWorkerStopsOnContextCancellation(t *testing.T) {
    runner := newBlockingRunner()
    app := worker.New(worker.Dependencies{Runners: []worker.Runner{runner}})
    ctx, cancel := context.WithCancel(context.Background())
    done := runAsync(func() error { return app.Run(ctx) })
    cancel()
    require.NoError(t, <-done)
    require.True(t, runner.Stopped())
}
```

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/app/worker -v`  
Expected: FAIL，Worker App 尚未定义。

- [ ] **Step 3: 实现生命周期与本地依赖**

Worker 当前只注册空 Runner 集合和健康生命周期，不创建未定义的持久化 Job 表。Compose 在 Task 4 的依赖配置基础上增加 API、Worker 和 MinIO 初始化服务；健康检查通过后应用容器才启动。Dockerfile 使用多阶段构建和非 root 运行用户。

- [ ] **Step 4: 配置 CI**

CI 使用 Go 1.26.5，执行 `gofmt` 检查、`go vet ./...`、`go test -race ./...`、`go build ./cmd/...`。README 写明环境变量、Migration、启动和测试命令，并明确当前只完成 Foundation。

- [ ] **Step 5: 完整验证**

Run: `go test -race ./...`  
Run: `go build ./cmd/...`  
Run: `docker compose --env-file .env.example -f deploy/docker-compose.yml config`  
Expected: 全部 PASS；Compose 配置无未解析变量。

- [ ] **Step 6: 提交**

```bash
git add internal/app/worker cmd/memora-worker deploy Dockerfile .github README.md
git commit -m "build: add worker lifecycle and local deployment"
```

## Foundation Acceptance

完成本计划后必须能够演示：显式 Migration 创建扩展和 users 表；API/Worker 正常启动和优雅停止；live/ready 健康检查；统一 request_id 响应；初始化用户能够登录、访问 `/users/me`、退出后令牌立即失效；Docker Compose 配置可解析；全部单元测试、Race Detector 和构建通过。不得出现数据库文档之外的新持久化表。
