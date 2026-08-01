# Memora 后端开发规范

## 1. 新功能开发顺序

1. 对齐需求、API 和数据库文档；
2. 在 `model/entity` 定义持久化实体；
3. 在 `model/dto` 定义 Request/Response；
4. 在 `repository` 定义接口并实现数据访问；
5. 在 `service` 定义接口并实现业务用例；
6. 在 `api/v1/<resource>` 创建 Controller 与 Routes；
7. 在 `internal/app` 注入依赖，在 Router 挂载路由；
8. 新表通过 `scripts/migrations` 追加显式 Migration；
9. 同步配置、README、API 和数据库文档。
10. 跨模块类型优先复用 `internal/contracts`，变更 contracts 必须说明兼容影响。

## 2. 分层红线

- Controller 禁止写 SQL、事务或业务状态机。
- Service 禁止依赖 Gin Context，统一接收 `context.Context`。
- Repository 查询必须包含用户归属、状态和软删除过滤。
- Entity、Request DTO、Response DTO 禁止混用。
- 禁止跨层直接调用和建立无职责的公共包。
- 禁止 AutoMigrate、无超时外部调用和无上限 Goroutine。

## 3. Go 规范

- Go 版本固定为 1.25.0；所有代码必须 `gofmt`。
- 包名使用小写单词，文件名使用 `snake_case.go`。
- 接口按能力命名，不加 `I` 前缀。
- `context.Context` 是需要它的方法第一个参数。
- 错误使用 `%w` 包装，判断使用 `errors.Is/As`。
- 禁止可变全局业务状态和隐式 `init()` 注册。

## 4. HTTP 规范

- 前缀 `/api/v1`，JSON 使用 `snake_case`。
- 身份从 Auth Middleware 注入，不信任请求中的 `user_id`。
- 使用 `pkg/response` 统一返回 `code/message/data/request_id`。
- 客户端不得收到 SQL、堆栈、Secret、Prompt 或内部错误。
- Handler 完成 Binding 后必须立即处理错误并返回。

## 5. 数据规范

- PostgreSQL 时间使用 `timestamptz`，应用统一 UTC。
- UUID 使用 `gen_random_uuid()`。
- SQL 参数化，分页有上限和确定排序。
- 复杂向量、全文、锁和批量查询使用原生 SQL。
- 事务不得包裹模型、MCP、MinIO 等长时间外部 I/O。
- 已共享 Migration 禁止回写，必须追加新序号。

## 6. 安全规范

- 密码使用 Argon2id；JWT 使用官方 v5 并限制 HS256。
- 用户查询过滤 `status='active' AND deleted_at IS NULL`。
- Redis 黑名单 TTL 等于 JWT 剩余有效期。
- 日志禁止记录密码、Token、Authorization Header 和密钥。
- CORS 不允许在携带凭证时使用通配 Origin。
- `release` 模式 JWT Secret 不少于 32 字符，禁止示例值。
- 管理员初始化不得覆盖已存在用户；密码重置必须使用显式命令。

## 7. Worker 与可观测性

- Worker Source 只领取所属业务表的可执行状态，不创建额外通用任务表。
- Handler 必须响应 `context` 取消，重复执行必须由幂等键保护。
- API 请求必须携带 Request ID 和 Trace ID；关键身份、配置和授权变更写结构化审计日志。
- `/metrics` 暴露 HTTP 与 Worker 指标；`/health/workers` 通过 Redis TTL 心跳报告存活 Worker。

## 8. 交付说明

只有实际执行并成功的命令才能写入交付报告。当前不保留测试文件，因此交付时必须明确标注“未执行自动化测试和运行态验收”。
