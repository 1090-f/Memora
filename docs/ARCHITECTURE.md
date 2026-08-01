# Memora 后端框架说明

## 1. 架构形态

Memora 使用 MVC 三层模块化单体：Controller 处理 HTTP，Service 处理业务，Repository 处理数据访问。通过构造函数注入依赖，由 App Launcher 集中初始化。

```text
cmd/server → internal/app → internal/api → internal/service → internal/repository
                                              ↓                    ↓
                                         model/dto            model/entity
```

部署包含三个进程：

- `memora-server`：REST API 和健康检查；
- `memora-worker`：后台任务与生命周期；
- `memora-migrate`：显式 SQL Migration 和管理员初始化。

## 2. 目录

```text
cmd/{server,worker,migrate}/       进程入口
configs/                           YAML 配置示例
internal/app/                      App Launcher 与生命周期
internal/contracts/                跨模块 contracts v0.1
internal/worker/                   任务来源、注册、执行、幂等与心跳
internal/api/v1/                   Controller 和 Routes
internal/middleware/               RequestID、日志、Recovery、CORS、Auth
internal/model/entity/             数据库实体
internal/model/dto/{request,response}/ HTTP DTO
internal/service/                  业务接口与实现
internal/repository/               数据访问接口与实现
pkg/config/                        Viper 配置
pkg/database/                      PostgreSQL、Redis、Migration
pkg/errors/                        稳定错误码
pkg/jwt/                           JWT v5 封装
pkg/logger/                        Zap + Lumberjack
pkg/audit/                         结构化审计事件
pkg/metrics/                       Prometheus 文本指标
pkg/objectstore/                   MinIO 封装
pkg/response/                      统一 API Envelope
scripts/migrations/                版本化 SQL
deploy/                            Docker Compose
```

## 3. 依赖规则

1. Controller 只依赖 Service、HTTP DTO、Middleware 和 Response。
2. Service 依赖 Repository 接口与 Entity，不依赖 Gin。
3. Repository 依赖 GORM/Redis 和 Entity，不处理 HTTP。
4. Entity 不引用 Gin、Service 或 Controller。
5. `pkg` 不包含 Memora 业务流程。
6. 所有外部依赖只在 `internal/app` 组装。

## 4. 请求链

```text
RequestID → AccessLog → Recovery → CORS → Auth → Controller
→ Service → Repository → PostgreSQL/Redis → Response Envelope
```

业务 API 前缀为 `/api/v1`。健康检查固定为 `/health/live` 与 `/health/ready`。

## 5. 数据与基础设施

- PostgreSQL 是业务事实来源，pgvector 与全文检索后续在 Repository 使用原生 SQL。
- Redis 保存撤销令牌、通知和控制状态，不作为核心数据唯一存储。
- MinIO 保存文件对象，业务层通过封装访问。
- 数据库只使用 `scripts/migrations` 中的显式 SQL，禁止 AutoMigrate。
- P0 Schema 固定为需求文档定义的 20 张表，Worker 复用各业务状态表领取任务。
- UUID 由 PostgreSQL `gen_random_uuid()` 生成。

## 6. 扩展模块

新增资源时使用 `internal/api/v1/<resource>` Controller，并在 `internal/service`、`internal/repository`、`internal/model` 增加对应实现。长耗时文档、索引、Agent 与 Memory 工作必须注册到 Worker，不得阻塞 HTTP 请求。

跨模块调用必须依赖 `internal/contracts`，不得复制定义 Retrieval、AgentContext、ToolResult、Citation 或 AgentEvent。Worker 模块通过 `RegisterJob` 注册业务状态表对应的 Source 与 Handler；任务必须支持超时、幂等、重试和取消。
