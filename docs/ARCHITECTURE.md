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
internal/contracts/                跨模块 contracts v0.1 与稳定错误码
internal/apperror/                 协议无关的应用错误与错误链
internal/worker/                   任务来源、注册、执行、幂等与心跳
internal/api/v1/                   Controller 和 Routes
internal/api/httperror/            错误码到 HTTP 状态和消息的映射
internal/api/response/             统一 API Envelope
internal/middleware/               RequestID、日志、Recovery、CORS、Auth
internal/model/entity/             数据库实体
internal/model/dto/{request,response}/ HTTP DTO
internal/service/                  业务接口与实现
internal/service/preview/          预览路由、调度、Artifact 校验和渲染处理
internal/service/preview/renderer/ LibreOffice PDF 与 XLSX OpenXML 渲染器
internal/repository/               数据访问接口与实现
internal/service/rag/              Eino RAG 内核（einoadapter、pipeline、loader、transformer、indexing、retrieval、observability）
internal/service/rag/einoadapter   contracts/Entity ↔ Eino schema.Document 的单一转换边界与 metadata 常量
internal/service/rag/query         查询 NFKC、大小写与空白规范化（不负责分词）
internal/service/rag/indexing      PostgresIndexer 等 Eino Indexer 实现（向量写入 pgvector）
internal/service/rag/retrieval     ParadeDBKeywordRetriever/PgVectorRetriever 等 Eino Retriever 实现
internal/service/rag/loader        SafeWebLoader（URL 导入 SSRF 防护、重定向/大小/超时限制）
internal/service/rag/mock          仅用于联调的确定性 RetrievalService Mock
internal/worker/document/          文档处理任务 Source 与 Handler（后续任务包落位）
pkg/config/                        Viper 配置
pkg/database/                      PostgreSQL、Redis、Migration
pkg/jwt/                           JWT v5 封装
pkg/logger/                        Zap + Lumberjack
pkg/audit/                         结构化审计事件
pkg/metrics/                       Prometheus 文本指标
pkg/objectstore/                   MinIO 封装
scripts/migrations/                版本化 SQL
deploy/                            Docker Compose
```

## 3. 依赖规则

1. Controller 只依赖 Service、HTTP DTO、Middleware 和 `internal/api/response`。
2. Service 依赖 Repository 接口、Entity、`internal/apperror` 和错误码契约，不依赖 Gin 或 HTTP 状态码。
3. Repository 依赖 GORM/Redis 和 Entity，不处理 HTTP。
4. Entity 不引用 Gin、Service 或 Controller。
5. `pkg` 不包含 Memora 业务流程。
6. 所有外部依赖只在 `internal/app` 组装。
7. 稳定错误码只在 `internal/contracts/error_code.go` 定义，HTTP 映射只存在于 API 层。

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

### 6.1 RAG 与文档处理职责

- `internal/service/rag` 承载 Eino 组件、Adapter、Compiled Graph、RAG 算法与 Callback；Eino `schema.Document` 只在本层与 `einoadapter` 之间交换，不得泄漏到 HTTP DTO、Entity 与跨成员 contracts。
- `internal/service/rag/einoadapter` 是 contracts/Entity ↔ Eino 的唯一转换边界，集中定义 metadata 键常量，禁止在其他包重复转换。
- `internal/service/rag/mock` 提供 `contracts.RetrievalService` 的确定性 Mock，仅供成员三联调，生产路径不得引用。
- 数据访问（SQL、GORM、pgvector、全文检索、`SKIP LOCKED`）只允许在 `internal/repository`；自定义 Eino Indexer/Retriever 通过最小 Repository 接口访问数据，组件本身不持有 GORM 查询逻辑。
- 文档处理长任务由 `internal/worker/document` 的 Source/Handler 驱动；Eino Compose 负责任务内部的确定性数据流，PostgreSQL 任务领取、状态持久化、重试、幂等与恢复仍由 `internal/worker` 通用 Runner/Registry 负责。
- 长耗时解析、分段、Embedding 和索引必须进入 Worker，不能阻塞 HTTP 请求；数据库是业务事实来源，Redis 不保存唯一业务状态。
- 文档视觉预览与 RAG 加工解耦：`document_previews` 保存当前内容版本的派生任务状态，
  Outbox 按 `document.preview.render` 事件投递到独立 Redis Stream。预览消费者领取任务后，
  DOCX/PPTX 通过受并发和超时约束的 LibreOffice 生成 PDF，XLSX 同时生成结构化 Sheet
  Artifact 和 PDF 回退。产物写入 MinIO 的 `derived/{user}/{document}/content-{version}/`
  版本目录，并在对象成功后最后发布 manifest；API 只读取并校验已经完成的产物。
- 预览失败不回滚或阻塞文档解析、分块与索引。前端通过统一 Descriptor 轮询预览状态，
  并按服务端返回的 fallback 顺序降级到解析正文或原文件下载。
- 检索在应用启动时编译单一 Eino Graph：参数校验 → keyword/vector Retriever（hybrid 并发）→ RRF → 可降级 Reranker → 知识充分性 → Citation。生产 API 不存在绕过 Graph 的第二条检索路径。
- `CitationService` 是 Retrieval 与 DocumentReader 的统一可信引用映射；`DocumentReader` 使用绑定用户/知识库/文档/索引版本的 HMAC 游标执行有限正文读取。
- URL 导入只在 HTTP 层落 pending 任务；安全抓取节点位于文档 Worker Graph，抓取结果继续进入与 TXT/Markdown/PDF/DOCX 相同的解析、分块和索引链。
