# Memora 成员一 RAG 与知识处理模块执行计划（DeepSeek 版）

> 编制日期：2026-08-05  
> 架构基线：2026-08-05 当前仓库（协议无关应用错误 + API 层 HTTP 映射）  
> 适用仓库：Memora  
> 负责人范围：成员一——完整 RAG 与知识处理  
> 技术策略：Eino 优先；官方组件满足需求时直接复用，不满足时按 Eino 接口实现 Memora 适配器或自定义组件。  
> 执行方式：将本文交给 DeepSeek，按任务包逐个实施；每次只执行一个任务包，验收通过后再进入下一包。

## 1. 给 DeepSeek 的总指令

你正在 Memora 仓库中实现“成员一：完整 RAG 与知识处理”模块。你的目标不是重新设计整个系统，而是在现有 Go/Gin/GORM/PostgreSQL/pgvector/Redis/MinIO 基础上，以 CloudWeGo Eino 的组件抽象和 Compose 编排作为 RAG 内核，按本文顺序完成一条可运行、可联调、可验收的 RAG 垂直链路。

开始任何修改前，必须完整阅读：

1. `后端代码开发规范.md`，它是当前后端实现和交付的完整约束；
2. `AI智能知识库与知识服务Agent系统需求规格说明书_轻量化数据版.md`，重点阅读 4.3～4.8、4.13、10.2、10.6、11.1～11.7、12.1～12.9；
3. `AI智能知识库_API接口文档_P0_轻量化版.md`，重点阅读知识库、目录、文档导入、搜索接口；
4. `AI智能知识库_数据库设计文档_P0_20表版.md`，重点阅读 5.2～5.9；
5. `docs/ARCHITECTURE.md`；
6. `docs/DEVELOPMENT.md`；
7. `internal/contracts/*.go`，尤其是 `error_code.go`；
8. `internal/apperror/error.go`、`internal/api/httperror/mapper.go`、`internal/api/response/response.go`；
9. `internal/worker/*.go`；
10. `scripts/migrations/000003_knowledge_configuration.up.sql` 和 `000004_documents_retrieval.up.sql`。

若简版 `docs/DEVELOPMENT.md` 与根目录 `后端代码开发规范.md` 在执行顺序或边界上不一致，以需求/API/数据库契约和根目录完整规范为准，并同步修正文档差异，不得静默选择对自己方便的规则。

同时阅读并以当前锁定版本源码为最终依据核对 Eino 官方资料：

1. Eino Components：`https://www.cloudwego.io/zh/docs/eino/core_modules/components/`；
2. Document Loader：`https://www.cloudwego.io/docs/eino/core_modules/components/document_loader_guide/`；
3. Document Transformer：`https://www.cloudwego.io/docs/eino/core_modules/components/document_transformer_guide/`；
4. Embedding：`https://www.cloudwego.io/docs/eino/core_modules/components/embedding_guide/`；
5. Indexer：`https://www.cloudwego.io/docs/eino/core_modules/components/indexer_guide/`；
6. Retriever：`https://www.cloudwego.io/zh/docs/eino/core_modules/components/retriever_guide/`；
7. Chain/Graph/Workflow 编排：`https://www.cloudwego.io/zh/docs/eino/core_modules/chain_and_graph_orchestration/`；
8. Eino 与扩展组件源码：`https://github.com/cloudwego/eino`、`https://github.com/cloudwego/eino-ext`。

执行通则：

- 先检查工作树，保留用户已有修改，不覆盖、不回滚无关文件。
- 严格遵守 Controller → Service → Repository 分层。
- Controller 不写 SQL 和状态机；Service 不依赖 Gin；Repository 必须做用户、知识库、状态和软删除过滤。
- Repository 必须在同一条查询中组合资源 ID、当前 UserID、KnowledgeBaseID、状态和软删除条件；禁止先按 ID 查询再由 Service 补做归属验证。
- Controller 统一使用 `internal/api/response`；Service 只返回 `internal/apperror.AppError` 或可映射的内部错误，不携带 HTTP 状态码。
- 稳定错误码只定义在 `internal/contracts/error_code.go`；HTTP 状态和公开消息只定义在 `internal/api/httperror`。禁止在 RAG 模块另建错误码枚举或复制错误消息表。
- 跨模块对象只能复用 `internal/contracts`，禁止复制 `RetrievalResult`、`Citation`、`DocumentReadResult` 等定义。
- RAG 内部优先使用 Eino 标准组件接口：`document.Loader`、`document.Transformer`、`embedding.Embedder`、`indexer.Indexer`、`retriever.Retriever` 和 `compose`。
- 官方 Eino/eino-ext 组件只有在满足本项目安全、数据归属、版本和存储约束时才能直接使用；不满足时，实现对应 Eino 接口的自定义组件，而不是绕开 Eino 另建一套不兼容抽象。
- Eino 的 `schema.Document` 只作为 RAG 内部交换对象；HTTP DTO、数据库 Entity 和跨成员 contracts 不得直接暴露 Eino 类型。
- Eino Compose 负责任务内部的确定性数据流编排；PostgreSQL 任务领取、状态持久化、重试、幂等和恢复仍由现有 Worker 框架负责，禁止用内存 Graph 状态代替持久化任务状态。
- Eino Callback 接入现有 logger/metrics/Trace，但默认只记录组件名、耗时、数量、模型和错误分类，不记录完整正文、向量、密钥或敏感元数据。
- 不修改成员二负责的 Router、Memory、Plan-Execute 和 ModelFactory 实现。
- 不修改成员三负责的 Agent Core、ReAct、ToolRegistry、ToolExecutor、SSE 和 MCP 实现。
- 你只提供 `RetrievalService`、`DocumentService`、`CitationService` 及 Mock；工具封装由成员三完成。
- 长耗时解析、分段、Embedding 和索引必须进入 Worker，不能阻塞 HTTP 请求。
- 数据库是业务事实来源；Redis 不保存唯一业务状态。
- 禁止 AutoMigrate。已存在的 Migration 不得回写；如确需修正，追加新的 up/down Migration。
- PostgreSQL 全文、向量、锁、批量写入使用参数化原生 SQL；普通 CRUD 可使用 GORM。
- 数据库事务不得包含 MinIO、HTTP、Embedding 或 Reranker 调用。
- 所有外部调用必须设置超时；所有批量大小、文件大小、TopK、Token 数必须有上限。
- 身份只能从认证中间件/运行时上下文获得，不信任请求体中的 `user_id`。
- URL 导入必须防 SSRF：只允许 HTTP/HTTPS，拒绝本机、内网、链路本地、云元数据地址，限制重定向、响应大小和超时。
- 当前仓库说明要求不新增或保留新的 `*_test.go`；除非用户明确取消该限制，否则不要创建永久测试文件。仍需运行现有测试、构建和静态检查。
- 不得把“代码已编写”描述为“运行态已验证”。只报告实际执行成功的命令。
- 不要仅为了“使用框架”而把简单校验、事务、SQL、权限判断包装成 Eino 节点；Eino 用于文档语义处理、索引、检索和可观察编排，传统业务 CRUD 继续使用现有分层。
- Eino/eino-ext 必须固定明确版本并写入 `go.mod/go.sum`；禁止依赖 `main`、伪造 API 或复制官方示例后未经编译验证。

每个任务包结束时必须输出：

1. 修改文件清单；
2. 已实现行为；
3. 实际执行的验证命令和结果；
4. 未验证项及原因；
5. 风险或待协作事项；
6. 下一任务包建议，但未经用户同意不要自动扩展到下一包。

统一验证命令：

```powershell
$taskGoFiles = @(
  git diff --name-only --diff-filter=ACMR -- '*.go'
  git diff --cached --name-only --diff-filter=ACMR -- '*.go'
  git ls-files --others --exclude-standard -- '*.go'
) | Sort-Object -Unique
$taskGoFiles | ForEach-Object { gofmt -w $_ }
go test ./...
go vet ./...
go build ./cmd/...
git diff --check
git status --short
```

如果命令因环境、数据库、MinIO、模型服务或网络不可用而失败，要保留完整错误摘要并区分“代码失败”和“环境未就绪”。

每个新增资源按当前完整开发规范使用以下顺序，任务包中的文件清单不代表可以跳过该顺序：

```text
需求/API/数据库对齐
→ contracts 影响确认
→ Migration 审计或追加（禁止回写既有 Migration）
→ Entity
→ HTTP Request/Response DTO
→ Repository 接口与实现
→ Service 接口、业务错误映射与事务编排
→ Controller/Routes
→ internal/app 依赖注入 + Router 挂载
→ Worker 注册（长任务）
→ API/数据库/架构/配置文档同步
```

当前仓库按负责人要求不保留测试文件。`go test ./...` 仍作为包编译和现有测试探测命令执行，但如果输出没有测试用例，交付必须明确写“未执行自动化行为测试和运行态验收”，不得将命令退出码 0 等同于功能通过。

## 2. 最终目标与明确边界

最终必须能够演示：

```text
知识库创建
→ 文件/URL/手工文档导入
→ 异步解析
→ 清洗与分段
→ 中文关键词字段构建
→ Embedding
→ 关键词和向量索引
→ RRF 混合检索
→ Reranker
→ 知识充分性判断
→ RetrievalResult + Citation
```

成员一主要交付：

- `DocumentService`；
- `DocumentProcessService`；
- `RetrievalService`；
- `CitationService`；
- `RetrievalService Mock`；
- 知识库、目录、文档、导入、搜索相关 API；
- 文档处理 Worker；
- MinIO 完整对象操作；
- PostgreSQL 全文和 pgvector 检索实现；
- 给 Memory 复用的 Embedding/pgvector 基础方案。

不在本计划范围：

- 前端页面重构；
- Chat/Agent 最终回答生成；
- ReAct 循环和 Tool Calling；
- Router、Planner、Executor、Reviewer；
- Memory 的提取、合并和生命周期；
- MCP Server 和联网搜索工具。

### 当前仓库基线（执行前必须复核）

- 已实现 Auth/User 的 Entity、Repository、Service、Controller，可作为新模块的分层和错误处理范例；
- `internal/contracts` 已有 Document、Retrieval、Citation、ModelFactory 和稳定错误码；
- `000003`、`000004` 已创建知识配置、文档、Chunk、Vector 和 ImportTask 等 P0 表，默认优先映射现有表而不是新建重复表；
- `pkg/objectstore/minio.go` 当前主要提供连接与健康检查，业务对象操作仍待扩展；
- 通用 Worker Runner、Registry、RoutedSource、幂等和心跳已存在，但尚未注册文档业务任务；
- 当前 `internal/api.Dependencies` 只注入 Auth/User 和健康检查，成员一 Service 需按最小接口扩展；
- `go.mod` 当前没有 Eino/eino-ext 依赖，必须在任务包 00 完成版本核对后再引入；
- 旧的 `pkg/errors`、`pkg/response` 已移除，禁止按旧示例恢复；
- 当前不保留测试文件，必须如实区分编译验证、自动化行为测试和真实依赖运行态验收。

## 3. 当前架构下的落位方案

当前仓库已经固定 MVC 模块化单体、协议无关应用错误和 API 层 HTTP 映射。成员一代码必须落入现有职责目录；Eino 属于 Service 内部的 RAG 实现细节，不新增一个与 Service 平级且职责重叠的业务层。

```text
internal/
├── api/v1/
│   ├── knowledgebase/
│   ├── directory/
│   ├── document/
│   ├── importtask/
│   └── search/
├── model/
│   ├── entity/
│   └── dto/{request,response}/
├── repository/
├── service/
│   └── rag/
│       ├── einoadapter/
│       ├── pipeline/
│       ├── loader/
│       ├── transformer/
│       ├── indexing/
│       ├── retrieval/
│       └── observability/
└── worker/
    └── document/
        ├── source.go
        └── handler.go
```

落位规则：

- 知识库、目录、文档、导入和搜索用例接口/编排放 `internal/service`；
- Eino 组件、Adapter、Compiled Graph、RAG 算法和 Callback 放 `internal/service/rag`；
- SQL、GORM、pgvector、全文检索和 `SKIP LOCKED` 只放 `internal/repository`；Eino 自定义 Indexer/Retriever 通过最小 Repository 接口访问数据，组件本身不持有 GORM 查询逻辑；
- Document Worker 的 Source/Handler 放 `internal/worker/document`，通用 Runner/Registry 保持不含业务逻辑；
- Controller 继续按资源放 `internal/api/v1/<resource>`；
- MinIO 客户端等无业务语义能力留在 `pkg/objectstore`，不得把文档状态机放入 `pkg`；
- 新增上述子目录时同步更新 `docs/ARCHITECTURE.md` 和根目录开发规范的目录职责，禁止只改代码不改架构文档。

### 3.1 当前错误与响应链

所有成员一接口统一遵循：

```text
Repository 可识别数据错误
→ Service 映射为 internal/apperror.AppError
→ AppError.Code 使用 internal/contracts/error_code.go
→ Controller 调用 internal/api/response.Failure
→ internal/api/httperror 映射 HTTP Status 和公开 Message
```

具体约束：

- Repository 不返回 HTTP 状态码，也不依赖 `internal/api`；
- Service 不拼接面向客户端的错误文本；内部原因使用 `%w`/`Cause` 保留；
- Controller 不自行判断错误文本，不维护局部错误码到状态码映射；
- 已移除的 `pkg/errors` 和 `pkg/response` 不得重新创建或引用；
- 新错误码确有必要时，同时更新 contracts、`httperror`、根 API 文档和简版 `docs/API.md`；
- RAG 常用现有错误码优先复用：`INVALID_ARGUMENT`、`INVALID_STATE`、`UNSUPPORTED_FILE_TYPE`、`FORBIDDEN`、`RESOURCE_NOT_FOUND`、`DUPLICATE_RESOURCE`、`INDEX_VERSION_CONFLICT`、`PAYLOAD_TOO_LARGE`、`KNOWLEDGE_INSUFFICIENT`、`MODEL_CALL_FAILED`、`UPSTREAM_TIMEOUT`、`SERVICE_UNAVAILABLE`。

### 3.2 Eino 复用决策矩阵

DeepSeek 实现任何 RAG 能力前，必须先按“官方现成组件 → 官方接口的自定义实现 → 普通 Go 业务逻辑”顺序决策。

| 能力 | 优先方案 | 不能直接复用时 | 禁止做法 |
|---|---|---|---|
| 本地文件加载 | `eino-ext` File Loader | MinIO Loader 实现 `document.Loader` | 另建与 Eino 无关的 Loader 接口 |
| MinIO/S3 加载 | 验证 `eino-ext` S3 Loader 是否兼容 MinIO、流式限制和 endpoint 配置 | 自研 `MinIOLoader` 实现 `document.Loader` | 下载完整大文件到内存 |
| URL 加载 | 安全下载器通过校验后再交给 Eino Parser/Loader | 自研 `SafeWebLoader` 实现 `document.Loader` | 未做 SSRF 防护直接使用通用 Web Loader |
| TXT 解析 | Eino/eino-ext Text Parser | 自定义 Parser，但输出 `schema.Document` | 输出私有文档结构后重复转换多次 |
| Markdown 解析/分段 | Eino Markdown Header Splitter + Recursive Splitter | 自定义 `document.Transformer` 补足标题路径、Token 上限 | 完全手写且不实现 Eino Transformer |
| PDF/DOCX 解析 | 优先评估当前 eino-ext 官方 Parser | 官方能力不足时实现 Eino Parser/Loader 适配器 | 为使用框架而牺牲页码、标题或安全限制 |
| 清洗 | 可组合的自定义 `document.Transformer` | Eino Lambda（仅类型不适合 Transformer 时） | 把清洗塞进 Controller/Repository |
| Embedding | Eino `embedding.Embedder`；通过适配器调用成员二 ModelFactory | `ContractsEmbeddingAdapter` 实现 Eino Embedder | 绕开 ModelFactory 读取模型密钥 |
| PostgreSQL/pgvector 写索引 | 自研 `PostgresIndexer` 实现 Eino `indexer.Indexer`，委托 Repository 批量持久化 | 无 | 在 Eino 组件中直接散落 GORM/SQL 或因缺少官方 PG 组件而更换数据库 |
| 关键词检索 | 自研 `PostgresKeywordRetriever` 实现 Eino `retriever.Retriever`，委托 Repository 全文查询 | 无 | 在 Service/Eino 组件内拼接 SQL |
| 向量检索 | 自研 `PgVectorRetriever` 实现 Eino `retriever.Retriever`，委托 Repository pgvector 查询 | 无 | 省略用户、知识库和 active version 过滤 |
| RRF 融合 | Eino Graph/Lambda 节点 | 自定义纯函数并以 Lambda 接入 Graph | 将 RRF 藏在 Controller 中 |
| Reranker | 优先评估 eino-ext ScoreReranker 是否适配已冻结模型 | `ContractsRerankerTransformer` 实现 Eino Transformer | 成员一自行实现第二套模型工厂 |
| Citation/知识判断 | Eino Lambda 节点调用 Memora 自研纯逻辑 | 普通 Go 逻辑，由 Graph 节点包装 | 把 Citation 交给 LLM 猜测生成 |
| 流程编排 | Eino `compose.Graph`；纯线性且类型稳定时可用 Chain | 无 | 用 Graph 代替 Worker 的持久任务状态机 |
| 观测 | Eino Callbacks 适配 Zap/metrics/Trace | 自定义 Callback Handler | 在回调中记录完整正文和向量 |

### 3.3 两条 Eino 编排

必须显式构建并在应用初始化时编译两条可复用编排；不能为每个请求重复 Compile。

#### 文档加工 Graph

```text
Input(ProcessInput)
→ validate（Lambda，校验归属/版本/限制）
→ load（Document Loader）
→ normalize metadata（Lambda）
→ clean（Document Transformer）
→ split（官方/自定义 Document Transformer）
→ tokenize（自定义 Transformer）
→ index（自定义 Postgres Indexer，内部调用 Eino Embedder）
→ summarize result（Lambda）
→ Output(ProcessOutput)
```

Worker Handler 在 Graph 外负责写入阶段状态、执行 Graph、处理重试和最终 active version 切换。Graph 内任何节点失败都必须返回错误，不能自行把数据库任务标成成功。

#### 混合检索 Graph

```text
Input(RetrievalInput)
→ validate（Lambda）
→ branch by mode
   ├── keyword retriever ─┐
   └── vector retriever ──┤
→ RRF fusion（Lambda）
→ rerank（Transformer/Lambda）
→ knowledge evaluation（Lambda）
→ citation mapping（Lambda）
→ Output(RetrievalResult)
```

Hybrid 模式允许关键词和向量分支并行；keyword/vector 单模式必须跳过无关分支。分支合并必须保留 KeywordRank、VectorRank、原始分数和 Chunk 元数据。

### 3.4 Eino 与 Memora 的类型边界

统一在 `internal/service/rag/einoadapter` 中转换，其他包不得各自重复转换：

```text
HTTP DTO / contracts / entity
        ↕ adapter
Eino schema.Document + component options
        ↕ custom Indexer/Retriever
PostgreSQL / pgvector
```

`schema.Document.MetaData` 必须使用集中定义的常量键，至少覆盖：

- `user_id`；
- `knowledge_base_id`；
- `document_id`；
- `chunk_id`；
- `chunk_no`；
- `index_version`；
- `heading_path`；
- `source_location`；
- `keyword_rank/score`；
- `vector_rank/score`；
- `rrf_score`；
- `reranker_score`；
- `document_title`；
- `document_updated_at`。

适配器取值必须做类型和缺失校验，禁止对 `MetaData` 直接做不安全类型断言。

## 4. 任务包 00：基线审计与契约冻结

### 目标

在写真实业务代码前，确认文档、数据库和 contracts 不冲突，并让成员三可以先用 Mock 联调。

### 执行步骤

1. 运行统一验证命令，记录修改前基线。
2. 使用 `go list -m -versions github.com/cloudwego/eino` 和所需 eino-ext 子模块核对可用版本；选择与 Go 1.25 兼容的稳定版本并固定，记录选择依据。不得凭记忆硬编码版本。
3. 做最小编译探针，确认锁定版本中的 `schema.Document`、Loader、Transformer、Embedder、Indexer、Retriever、Compose 和 Callback API；计划正文与实际源码冲突时以锁定版本源码为准，并记录差异。
4. 核对以下现有契约：
   - `internal/contracts/document.go`；
   - `internal/contracts/retrieval.go`；
   - `internal/contracts/citation.go`；
   - `internal/contracts/model.go`；
   - `internal/contracts/error_code.go`。
5. 核对 `internal/apperror`、`internal/api/httperror`、`internal/api/response` 的当前调用方式，列出成员一所有预期错误及复用的 contracts 错误码。
6. 对照 API 文档确认请求与响应字段、枚举、默认值、分页和错误码。
7. 仅在确有缺失时，兼容性扩展 contracts；不得随意改名或删除已有字段。新增错误码必须同步 HTTP 映射和 API 文档。
8. 先更新 `docs/ARCHITECTURE.md`，登记 `internal/service/rag` 与 `internal/worker/document` 的职责和依赖方向，再创建这些目录。
9. 定义成员一内部 `DocumentProcessService` 接口，至少包含：创建导入任务、重试、重新索引、查询处理状态。
10. 在 `internal/service/rag/einoadapter` 定义 contracts ↔ Eino 的单一转换边界和 metadata key 常量。
11. 设计 `ContractsEmbeddingAdapter`：包装 `contracts.EmbeddingModel`，实现锁定版本的 Eino `embedding.Embedder`，负责 float32/float64 安全转换和维度校验。
12. 设计 `ContractsRerankerTransformer`：包装 `contracts.Reranker`，实现 Eino Transformer 或以 Lambda 节点接入，保留输入文档 metadata。
13. 实现可注入的 `RetrievalService Mock`：
   - 返回确定性结果；
   - 同时覆盖有结果和 `insufficient` 两种情况；
   - 不调用数据库或模型；
   - 明确标注只用于联调。
14. 给成员二/三列出接口冻结说明，包含调用方向、Eino 不外泄边界、超时、错误语义和数据归属要求。

### 验收标准

- 现有 contracts 可编译；
- Mock 实现满足 `contracts.RetrievalService`；
- 成员三无需了解你的 Repository 即可调用 Mock；
- 没有实现或复制 Tool 层逻辑；
- Eino 版本被明确固定，最小组件/Graph 能成功编译；
- 新目录职责已进入架构文档；
- 输出一份契约冻结记录、错误码映射清单和 Eino 组件选型记录。

## 5. 任务包 01：知识库、默认目录、搜索配置与默认 Agent 配置

### 目标

完成知识库、默认目录、搜索配置和 Agent 默认配置的原子创建闭环，为文档业务提供可靠的归属边界。成员一只负责知识库创建时写入 Agent 默认配置，Agent 配置业务规则和工具授权仍由对应成员负责。

### 计划文件

- `internal/model/entity/knowledge_base.go`；
- `internal/model/entity/search_config.go`；
- `internal/model/entity/document_directory.go`；
- `internal/model/entity/agent_config.go`（仅映射既有表和支持默认行创建，不扩展 Agent 业务）；
- 对应 request/response DTO；
- 对应 Repository 接口与实现；
- 对应 Service 接口与实现；
- `internal/api/v1/knowledgebase/*`；
- `internal/api/v1/directory/*`；
- `internal/app/server.go`；
- `internal/api/router.go`。

### 执行步骤

1. 先审计 `000003_knowledge_configuration` 与数据库设计；P0 现有表满足需求时不得新增表或回写 Migration，确需修正时使用当前最大编号之后的新 up/down Migration。
2. 实体字段严格映射 Migration，不使用 AutoMigrate。
3. 实现知识库创建、列表、详情、更新、软删除。
4. 创建知识库时，通过 Service 控制的短事务一次创建 `knowledge_bases + 默认 document_directories + search_configs + agent_configs`；不得在事务中执行 MinIO 或模型调用。
5. 跨 Repository 事务使用明确、可注入的 Transactor/UnitOfWork 能力；Controller 不开启事务，Repository 不决定业务事务边界。
6. 实现搜索配置读取与更新，对 TopK、RRF K、阈值进行范围校验。
7. 实现目录树读取和目录创建。
8. 所有资源查询在 Repository 同一查询中强制包含 `resource_id + user_id + knowledge_base_id（适用时）+ status/deleted_at`。
9. 防止目录挂到其他用户或其他知识库。
10. Service 将 Repository 的 not found/conflict/state 错误映射成 `internal/apperror`；Controller 统一调用 `internal/api/response`。
11. 在 `internal/app/server.go` 构造 Repository、Service 和 Controller 依赖，在 `internal/api/router.go` 只挂载路由。

### 验收标准

- 用户 A 无法读取或修改用户 B 的知识库和目录；
- 删除知识库为软删除，不直接删除 MinIO 对象；
- 知识库、默认目录、搜索配置和 Agent 默认配置要么全部创建，要么全部回滚；
- 搜索配置始终存在且数值有上限；
- API 使用 `internal/api/response`，错误码来自 contracts，HTTP 映射来自 `internal/api/httperror`；
- server、worker、migrate 三个入口均可构建。

## 6. 任务包 02：文档 CRUD 与 MinIO 对象能力

### 目标

支持手工文档、文件元数据和安全的对象存储操作，但暂不执行解析。

### 计划文件

- `internal/model/entity/document.go`；
- 文档 request/response DTO；
- `internal/repository/document_*`；
- `internal/service/document_*`；
- `internal/api/v1/document/*`；
- `pkg/objectstore/minio.go`。

### MinIO 必备能力

- `PutObject`：流式上传，限制大小，接受明确 content type；
- `GetObject`/`OpenObject`：返回可关闭流，不一次性读入内存；
- `StatObject`；
- `RemoveObject`；
- 统一规范 object key，至少包含用户 ID、知识库 ID、任务 ID；
- 错误必须包装且不得泄漏 AccessKey/SecretKey。

### 执行步骤

1. 实现手工创建文档、文档列表、详情和软删除。
2. 文档详情只能通过 `user_id + knowledge_base_id/document_id` 查询。
3. 文件上传先创建 `import_tasks`，再流式上传 MinIO，最后更新任务对象信息。
4. MinIO 上传成功但数据库更新失败时，执行补偿删除；补偿失败需记录日志。
5. 数据库事务与 MinIO I/O 分离。
6. 对文件名、扩展名、MIME、文件大小和空文件进行校验。
7. 哈希使用流式 SHA-256，不将完整文件读入内存。
8. Repository 的文档详情、删除和状态查询必须在同一查询中包含 DocumentID、当前 UserID、KnowledgeBaseID、状态/软删除过滤。
9. Service 将文件过大、格式不支持、资源不存在、重复和 MinIO 上游失败分别映射为现有 contracts 错误码；Controller 只调用 `response.Success/Failure`。

### 验收标准

- Markdown/TXT 文件可以安全上传；
- 文档和任务正确记录 MinIO bucket/object key；
- 上传失败不会生成“成功”任务；
- 不存在路径穿越和任意 object key 注入；
- 大文件处理不依赖 `io.ReadAll`。
- Service 错误不携带 HTTP 状态，MinIO 内部错误不会直接返回客户端。

## 7. 任务包 03：ImportTask 状态机与 Worker 接入

### 目标

让 HTTP 只创建任务，由现有 Worker 领取并执行；建立后续文档流水线的调度基础。

### 计划文件

- `internal/model/entity/import_task.go`；
- `internal/repository/import_task_*`；
- `internal/service/document_process_*`；
- `internal/worker/document/source.go`；
- `internal/worker/document/handler.go`；
- `internal/app/worker.go`。

### 执行步骤

1. 实现任务创建、列表、详情和显式重试。
2. `ImportTaskSource.Reserve` 使用 PostgreSQL 行锁领取任务，避免多个 Worker 重复领取；优先考虑 `FOR UPDATE SKIP LOCKED`。
3. 状态转换限定为：

```text
pending → running → succeeded
                  ↘ failed
pending/running → skipped
failed → pending（显式重试）
```

4. Worker Job 的幂等键包含任务 ID 和处理版本。
5. `Complete/Retry/Fail` 必须回写 `import_tasks`，不得创建新的通用业务任务表。
6. Handler 必须响应 context 取消，并记录 `current_step` 和安全的失败原因。
7. Worker 初始化时显式注册任务类型，不使用隐式 `init()`。
8. 对卡在 running 状态且超过租约/超时的任务定义恢复策略。
9. `internal/worker/document.Source` 只依赖 ImportTask Repository；`Handler` 只依赖 `DocumentProcessService`，不得直接拼 SQL、构建 Gin 响应或自行创建 Eino Graph。
10. `internal/service/rag/pipeline` 定义 Graph 节点和边，暴露接收依赖的构造函数；`internal/app/worker.go` 只构造 Repository/组件、调用 Pipeline 构造函数取得 Compiled Graph、注入 Service/Source/Handler，并通过现有 `RegisterJob` 注册。通用 Runner/Registry 不感知文档业务。
11. Worker 失败原因作为内部错误链安全落库；HTTP 查询任务状态时再由 Service/API 错误链完成协议映射。

### 验收标准

- 两个 Worker 不会同时处理同一个任务；
- 重复执行不生成重复 Document；
- 失败任务能够显式重试；
- API 请求不会等待解析或 Embedding 完成；
- `/health/workers` 行为不被破坏。

## 8. 任务包 04：基于 Eino 的 Markdown/TXT 最小文档加工链路

### 目标

先用 Markdown 和 TXT 打通 Eino“Loader → Transformer → Indexer”最小链路，暂不支持 PDF/DOCX/URL。

### 计划组件

- Eino `document.Loader` 和 Loader 选择器；
- eino-ext Text Parser/File Loader（锁定版本可用时）；
- eino-ext Markdown Header Splitter；
- eino-ext Recursive Splitter；
- 实现 Eino `document.Transformer` 的 Cleaner；
- Eino Compose 文档加工 Graph；
- Token/字符计数器；
- `DocumentProcessService` 真实编排。

### 统一中间结构

Eino 节点之间统一使用 `[]*schema.Document`；业务输入输出使用 Memora 类型，由 adapter 转换。每个 Eino Document 必须稳定携带：

```text
schema.Document
├── ID = 可稳定复现的 document/chunk 标识
├── Content
└── MetaData
    ├── document_id / chunk_no / index_version
    ├── heading_path / source_location
    ├── content_version / chunk_version
    └── chunk_config_hash
```

### 分段默认策略

- 优先按 Markdown 标题和自然段边界切分；
- 再按最大 Token/字符上限切分；
- 保留有限重叠，避免上下文断裂；
- 空白片段不入库；
- 每个 Chunk 保存 `heading_path`、`source_location`、`context_title`；
- 分段参数序列化后计算稳定 `chunk_config_hash`；
- 相同输入与配置必须产生确定性顺序和内容。

具体数值优先读取项目配置；若文档没有冻结，先采用保守默认值并集中定义，禁止散落魔法数字。

### 执行步骤

1. 先验证 eino-ext File/S3 Loader 是否满足 MinIO、流式和限制要求；满足则直接使用，不满足则实现 `MinIOLoader`，但必须符合 Eino `document.Loader`。
2. 根据受信 MIME 与扩展名选择 Eino Loader/Parser，禁止按用户传入类型盲选。
3. 使用 `compose.Graph` 构建 load → clean → split → tokenize → persist 的确定性处理图；在应用初始化时 Compile 一次并注入 Worker Handler。
4. 优先复用 eino-ext Markdown Header Splitter 和 Recursive Splitter；如 metadata 或 Token 规则不足，使用组合 Transformer 补充，不直接 fork 官方源码。
5. 自定义 Cleaner、Tokenizer 必须实现 Eino Transformer，正确保留并扩展 `schema.Document.MetaData`。
6. 通过 Eino Callbacks 上报组件耗时、输入/输出数量和错误类型；禁止记录正文。
7. Worker 在 Graph 外更新 `documents.processing_status`：parsing、cleaning、chunking。
8. 生成新的 `content_version/chunk_version/index_version`。
9. 持久化节点在短事务中批量插入 `document_chunks`，但暂不切换 `active_index_version`。
10. 任何节点失败均向上返回错误，由 Worker 记录 `failure_step/failure_reason`，旧 active 版本继续可用。

### 验收标准

- Markdown 标题层级进入 `heading_path`；
- TXT 能稳定分段；
- 相同输入重复处理不会产生不可控重复数据；
- 失败时文档为 failed，旧索引不受影响；
- Chunk 包含字符数、Token 数、版本和来源位置；
- Graph 只在初始化时 Compile，Worker 调用已编译 Runnable；
- 至少 Loader、Splitter/Transformer 和编排实际使用 Eino，而非仅导入依赖。

## 9. 任务包 05：中文关键词字段与全文索引

### 目标

生成适用于 PostgreSQL `simple` 配置的中文 `fts_tokens`，并实现可过滤的关键词检索。

### 执行步骤

1. 实现可替换的 `ChineseTokenizerTransformer`，满足锁定版本的 Eino `document.Transformer`；必要的分词器内核可以是普通 Go 接口，但对编排暴露 Eino Transformer。
2. 选择维护良好的中文分词实现；新增依赖前说明许可证、维护状态和二进制体积影响。
3. 规范化中英文大小写、标点和空 Token，保留必要技术词汇。
4. 将分词结果以空格分隔写入 `document_chunks.fts_tokens`。
5. 在 `internal/repository` 实现参数化全文检索能力；`PostgresKeywordRetriever` 满足 Eino `retriever.Retriever` 并只负责 Eino options/metadata 与 Repository 查询参数/结果的适配。Repository SQL 必须过滤：
   - `user_id`；
   - `knowledge_base_id`；
   - 可选 `document_ids`；
   - `documents.deleted_at IS NULL`；
   - `document_chunks.index_version = documents.active_index_version`。
6. 排名使用 PostgreSQL 稳定排序函数，并用 Chunk ID 作为最终确定性 tie-breaker。
7. 对 Query 空值、TopK 和 DocumentIDs 数量设置上限。
8. 将 DB 返回行统一映射成 `schema.Document`，把 rank、score、Chunk/Document/版本信息放入集中定义的 metadata keys。
9. 正确处理 Eino Retriever 公共 TopK/Threshold options 以及自定义的 UserID、KnowledgeBaseID、DocumentIDs、IndexVersion options；身份类 option 只能由 Service 注入。
10. 实现 Eino Retriever Callbacks，记录耗时、TopK 和结果数量，不记录 Query 全文或 Chunk 正文。

### 验收标准

- 中文问题能够召回包含相关中文词的 Chunk；
- 不会召回旧索引版本；
- 文档范围过滤有效；
- 相同数据和查询的排序稳定；
- 查询计划能够使用 GIN 索引；
- `PostgresKeywordRetriever` 可作为 Eino Graph 节点独立编译和调用。

## 10. 任务包 06：Eino Embedding、PostgresIndexer、pgvector 与索引版本切换

### 目标

通过成员二提供的 `contracts.ModelFactory` 获取 EmbeddingModel，经适配器转为 Eino Embedder，由自定义 Eino PostgresIndexer 批量生成并保存向量，最后安全激活新索引。

### 执行步骤

1. Service 只依赖 `contracts.ModelFactory`，不要实现或复制成员二的模型工厂。
2. 根据知识库/文档配置获取 `contracts.EmbeddingModel`，使用任务包 00 的 `ContractsEmbeddingAdapter` 转成 Eino `embedding.Embedder`。
3. 适配器必须处理 Eino `[][]float64` 与 contracts `[][]float32` 的转换、NaN/Inf、返回数量和维度校验，并支持 Eino callbacks/options。
4. 在 `internal/repository` 实现 Chunk/Vector 批量持久化接口；`PostgresIndexer` 满足 Eino `indexer.Indexer`，读取 `schema.Document` 与 metadata，使用 `indexer.WithEmbedding` 注入 Embedder，生成向量后委托 Repository 批量写入 `document_vectors`。Indexer 自身不持有 GORM/原生 SQL。
5. 对 Chunk 内容批量调用 Eino Embedder：
   - 批大小可配置且有上限；
   - 每次调用有超时；
   - 校验返回数量和维度；
   - 禁止记录完整敏感正文和模型密钥。
6. 批量写入 `document_vectors`，状态从 pending 到 ready/failed。
7. 在 `internal/repository` 实现 cosine distance 的参数化 pgvector 查询；`PgVectorRetriever` 满足 Eino `retriever.Retriever` 并委托该 Repository，执行与关键词检索相同的归属和版本过滤。Retriever 自身不拼 SQL。
8. `PgVectorRetriever` 必须正确处理 Eino Retriever 的 TopK、ScoreThreshold、Embedding 公共 options，以及由 Service 安全注入的租户过滤 options。
9. 根据实际冻结的 Embedding 维度追加 HNSW/IVFFlat Migration；不得修改 `000004`。
10. 在文档加工 Graph 中使用 `AddIndexerNode`/等价锁定版本 API 接入 PostgresIndexer；不要在 Lambda 中绕过 Indexer 接口直接写向量。
11. 只有当新版本 Chunk、关键词字段和全部必要向量成功后，才在短事务内更新：
   - `documents.active_index_version`；
   - `documents.embedding_model_id`；
   - `documents.chunk_config_hash`；
   - `documents.processing_status = succeeded`。
12. 新版本失败时保留旧 active 版本。
13. 设计给 Memory 复用的 Eino Embedder adapter 和 pgvector Repository 辅助能力，但不要实现 Memory 业务。

### 验收标准

- 向量维度不符时明确失败，不写入损坏向量；
- 向量检索只能读取 ready 且 active 的向量；
- 新索引完整成功前不会对外可见；
- 重建失败后旧索引仍能检索；
- 没有绕过 ModelFactory 自建模型配置逻辑；
- `PostgresIndexer` 和 `PgVectorRetriever` 分别满足 Eino 标准组件接口并触发 callbacks；
- Graph 中实际使用 Eino Embedding/Indexer 节点。

## 11. 任务包 07：Eino 混合检索 Graph、RRF、Reranker 与知识判断

### 目标

用 Eino Compose Graph 编排关键词 Retriever、向量 Retriever、RRF、Reranker、知识判断和 Citation，最终由 Memora Service 适配为 `contracts.RetrievalService`。

### 推荐内部结构

```text
RetrievalService（Memora 对外边界）
└── Compiled Eino Retrieval Graph
    ├── QueryValidatorLambda
    ├── PostgresKeywordRetriever（Eino Retriever）
    ├── PgVectorRetriever（Eino Retriever）
    ├── RRFFusionLambda
    ├── RerankerTransformer / Lambda
    ├── KnowledgeEvaluatorLambda
    └── CitationBuilderLambda
```

### 执行步骤

1. 定义内部 `RetrievalInput/Output`，通过 adapter 与 contracts 转换；Eino 类型不得泄漏到 API 或 Tool 边界。
2. 使用 Eino `compose.Graph` 构建并在应用初始化时 Compile；编译失败必须导致依赖初始化失败，不能运行时静默降级成未编排实现。
3. 校验 `RetrievalRequest`：身份、知识库、模式、Query、TopK、配置上限。
4. 支持三种模式：keyword、vector、hybrid，并使用 Graph 分支控制选择节点。
5. Hybrid 模式通过 Eino Graph 并行执行两个 Retriever 分支；并发必须有边界并继承 context。若锁定版本的 Graph 并行能力不满足限制，可在单个 Lambda 内使用 `errgroup`，但两个底层检索器仍必须实现 Eino Retriever。
6. 使用 Chunk ID 合并两路 `schema.Document` 并保留原始 rank/score metadata。
7. RRF 作为纯函数 Lambda 节点，采用冻结公式：

```text
rrf_score(chunk) = Σ 1 / (rrf_k + rank)
```

8. RRF 后截取 `rrf_top_k`，再调用成员二 ModelFactory 提供的 Reranker。
9. 优先评估 eino-ext ScoreReranker 与已冻结 Reranker 是否匹配；匹配则直接复用，不匹配则使用 `ContractsRerankerTransformer`，但必须保持 Eino Transformer/Lambda 节点形态。
10. Reranker 返回的 index 必须做越界、重复和缺失校验；异常时按冻结策略选择“降级到 RRF”或“整体失败”，并通过 Eino Callback 与现有 metrics 可观察。
11. 最终截取请求 TopK 和配置 `reranker_top_k` 的较小安全值。
12. 知识判断和 Citation 映射分别作为 Lambda 节点；Citation 来源只能取可信数据库 metadata，不能交给模型自由生成。
13. 基于有效结果数量、阈值和最高分判断 `knowledge_status`。
14. 返回结果不能包含其他用户、其他知识库或已删除文档。
15. `RetrievalService.Retrieve` 只负责校验/适配、调用已编译 Runnable 和错误映射，不重复实现 Graph 内算法。
16. Graph/组件返回可识别的内部哨兵错误并保留错误链；`RetrievalService` 统一映射成 `internal/apperror`。模型失败、上游超时、服务不可用和索引冲突分别使用已有 contracts 错误码。
17. 正常检索无有效依据时返回 `RetrievalResult.KnowledgeStatus = insufficient`，不要默认把它转成 HTTP 422；只有 API/内部契约明确要求显式失败的场景才使用 `KNOWLEDGE_INSUFFICIENT`。

### 排序稳定性

最终排序至少依次使用：

1. Reranker score 降序；
2. RRF score 降序；
3. 最佳原始 rank 升序；
4. Chunk ID 升序。

### 验收标准

- 三种模式均能返回合法 `RetrievalResult`；
- Hybrid 能显示 KeywordRank、VectorRank 和融合分数；
- Reranker 失败行为符合冻结策略；
- 无有效结果时返回 `insufficient`，而不是伪造引用；
- 成员三可直接将真实实现替换 Mock；
- Eino Callback 能看到两个 Retriever、RRF/Reranker 节点的耗时与错误；
- 代码中不存在绕过 Graph 的第二套生产检索路径。

## 12. 任务包 08：DocumentService 与 CitationService

### 目标

为成员三的 DocumentReadTool 提供安全、有限、可继续读取的正文服务。

### 执行步骤

1. 实现 `contracts.DocumentService.Read`。
2. 强制校验：
   - UserID；
   - KnowledgeBaseID；
   - DocumentID；
   - 文档归属；
   - 文档状态；
   - 软删除状态；
   - MaxTokens 上限。
3. 支持 Section、Cursor 和 MaxTokens。
4. Cursor 必须不可伪造或至少严格校验文档/版本/偏移，不允许跨文档读取。
5. 返回 `next_cursor`、`truncated` 和 Citation。
6. 不允许一次返回无限正文；也不允许用户通过极大 MaxTokens 绕过上限。
7. CitationService 统一构建引用，避免 Retrieval 和 DocumentRead 产生不一致结构。
8. Retrieval Graph 通过 Lambda Adapter 调用同一个 CitationService；DocumentRead 直接调用该 Service，禁止维护两套引用映射。

### 验收标准

- 小 MaxTokens 会截断并返回下一游标；
- 使用下一游标可以继续读取且不重复/漏读不可接受范围；
- 篡改游标或跨用户读取被拒绝；
- 引用包含正确文档、Chunk/位置和更新时间；
- 服务层不依赖 Gin Context。

## 13. 任务包 09：PDF、DOCX 和 URL 导入

### 目标

在 Markdown/TXT 链路稳定后扩展格式，不改变后续 Cleaner/Chunker/Indexer 接口。

### PDF/DOCX

1. 先核对锁定版本 eino-ext 是否提供满足要求的 PDF/DOCX Parser/Loader；满足正文、标题、来源位置和安全限制时直接使用。
2. 官方 Eino 组件能力不足时，自研适配器必须实现 Eino Parser/Loader 接口并输出 `schema.Document`，新增解析依赖前说明取舍。
3. Parser 必须设置页数、解压大小、压缩比和解析时长上限。
4. PDF 的 `source_location` 至少保存页码；DOCX 尽量保存段落/标题信息。
5. 对加密、损坏、扫描型 PDF 返回明确失败，不伪造空文档成功。
6. 防止 zip bomb 和异常内存消耗。

### URL

1. 先评估 eino-ext WebURL Loader，但只有能注入安全 HTTP Client、限制重定向/响应体并完成 SSRF 校验时才可直接使用；否则实现 `SafeWebLoader`，满足 Eino `document.Loader`。
2. 只接受 HTTP/HTTPS。
3. DNS 解析后拒绝 loopback、private、link-local、multicast、unspecified 和云元数据地址。
4. 每次重定向重新校验目标，限制重定向次数。
5. 限制连接、首字节、总请求超时和响应体大小。
6. 只接受允许的 Content-Type；优先使用 eino-ext HTML Transformer/Splitter，能力不足时实现自定义 Transformer，之后进入统一 Cleaner。
7. 保存最终 URL、标题、抓取时间和来源位置。

### 验收标准

- PDF、DOCX、URL 都进入同一后续分段和索引链路；
- 三种新增来源都通过 Eino Loader/Parser/Transformer 组件接入现有 Graph，不另建旁路流水线；
- 内网 URL、`localhost`、`127.0.0.1`、`169.254.169.254` 等被拒绝；
- 超大、压缩炸弹、损坏和超时输入安全失败；
- 失败信息对用户可理解但不泄露内部网络信息。

## 14. 任务包 10：API 补齐、联调与最终验收

### 目标

补齐成员一 API，完成与成员二、成员三的接口联调和验收材料。

### 当前架构接入方式

- 在 `internal/api.Dependencies` 增加最小 Service 接口字段，不把 Repository、GORM、MinIO Client 或 Eino Runnable 暴露给 Router；
- `internal/app/server.go` 负责构造并注入所有依赖；`internal/api/router.go` 只创建 Controller 和注册 Routes；
- Controller 从 Auth Middleware 获取 UserID，绑定 Path/Query/Body 后调用 Service，并统一使用 `internal/api/response`；
- Service 返回 `internal/apperror`，不得返回 HTTP 状态；新增错误码必须先进入 contracts，再由 `internal/api/httperror` 映射；
- 本计划不修改已收敛后的前端架构，除非用户另行要求前端联调。

### 必备 API

```text
POST   /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases/{kb_id}
PATCH  /api/v1/knowledge-bases/{kb_id}
DELETE /api/v1/knowledge-bases/{kb_id}
GET    /api/v1/knowledge-bases/{kb_id}/search-config
PUT    /api/v1/knowledge-bases/{kb_id}/search-config

GET    /api/v1/knowledge-bases/{kb_id}/directories/tree
POST   /api/v1/knowledge-bases/{kb_id}/directories

POST   /api/v1/knowledge-bases/{kb_id}/documents
GET    /api/v1/knowledge-bases/{kb_id}/documents
GET    /api/v1/documents/{document_id}
DELETE /api/v1/documents/{document_id}
GET    /api/v1/documents/{document_id}/processing
POST   /api/v1/documents/{document_id}/retry-processing
POST   /api/v1/documents/{document_id}/reindex
GET    /api/v1/documents/{document_id}/index-versions

POST   /api/v1/knowledge-bases/{kb_id}/imports/files
POST   /api/v1/knowledge-bases/{kb_id}/imports/url
GET    /api/v1/knowledge-bases/{kb_id}/import-tasks
GET    /api/v1/import-tasks/{task_id}
POST   /api/v1/import-tasks/{task_id}/retry

POST   /api/v1/knowledge-bases/{kb_id}/search
POST   /api/v1/knowledge-bases/{kb_id}/search/test
```

### 联调清单

- 成员三的 `KnowledgeSearchTool` 能调用真实 RetrievalService；
- 成员三的 `DocumentReadTool` 能调用真实 DocumentService；
- Tool 不能通过参数覆盖 UserID 和 KnowledgeBaseID；
- 成员二可以通过冻结接口复用 Embedding 能力；
- Reranker 和 Embedding 均通过成员二 ModelFactory 获取；
- 全链路携带 context、Request ID/Trace ID，并输出必要指标；
- 日志不记录原文全文、Token、密钥和 Authorization；
- Eino Callback 已适配现有 Zap/metrics/Trace，能够按 Graph/Node/Component 观察耗时、结果数量、降级和错误；
- Eino Graph 的节点/边定义位于 `internal/service/rag/pipeline`；实例依赖由 `internal/app` 统一组装并在启动期编译，不在 App 中实现业务规则，也不在 Controller 或每次请求中临时创建。

### 最终运行态验收

使用真实 PostgreSQL、pgvector、Redis、MinIO、Embedding 和 Reranker 环境，至少准备：

- 1 个知识库；
- 1 个 Markdown；
- 1 个 TXT；
- 1 个 PDF；
- 1 个 DOCX；
- 1 个允许访问的 URL；
- 10 个有答案问题；
- 5 个明确无答案问题；
- 2 个不同用户用于越权验证。

逐项演示：

1. 导入任务从 pending 到 succeeded；
2. 文档产生 Chunk、关键词和 ready 向量；
3. keyword/vector/hybrid 三种模式可用；
4. RRF 与 Reranker 排序信息完整；
5. 有答案问题返回正确 Citation；
6. 无答案问题返回 insufficient；
7. 重建期间旧索引仍可用；
8. 重建成功后 active index 原子切换；
9. 失败任务可重试且不会重复创建文档；
10. 跨用户、跨知识库和篡改游标访问被拒绝。

## 15. 建议里程碑

| 里程碑 | 任务包 | 可交付演示 | 建议工作量 |
|---|---|---|---|
| M1 契约可联调 | 00 | Mock + contracts 冻结 | 1～2 天 |
| M2 知识业务可用 | 01～03 | 知识库、文档上传、异步任务 | 5～8 天 |
| M3 最小 RAG 链路 | 04～06 | Markdown/TXT → Chunk → 关键词/向量 | 7～10 天 |
| M4 完整检索 | 07～08 | RRF → Reranker → Citation/Read | 5～7 天 |
| M5 格式与联调 | 09～10 | PDF/DOCX/URL + 团队联调 | 5～8 天 |

总建议工作量：23～35 个有效开发日。若工期紧张，P0 优先完成任务包 00～08，PDF/DOCX/URL 可在 Markdown/TXT 链路稳定后再补。

## 16. 风险与决策门

以下事项如果尚未冻结，DeepSeek 不得静默猜测并大范围实现，必须先在任务报告中列出建议和影响：

1. Embedding 模型及固定维度；
2. Reranker 服务和失败降级策略；
3. 中文分词库；
4. Chunk Token 计算方式、大小和重叠；
5. PDF/DOCX 解析库；
6. URL 导入允许范围和最大响应大小；
7. 索引旧版本保留数量和清理时机；
8. 当前“不保留测试文件”规则是否继续执行；
9. Eino 与实际采用的 eino-ext 子模块版本，以及版本升级策略。

默认处理原则：优先通过接口隔离变化；集中配置默认值；先实现安全、确定性的最小能力，不在业务代码中绑定单一厂商。

## 17. 每个任务包可复用的提示词模板

将以下内容和本文一起交给 DeepSeek，并替换任务包编号：

```text
请在当前 Memora 仓库执行《Memora 成员一 RAG 与知识处理模块执行计划（DeepSeek 版）》中的“任务包 XX”。

要求：
1. 开始前阅读总指令、任务包目标和仓库开发规范；
2. 先检查 git status 和相关现有代码，不覆盖用户已有修改；
3. 本次只实现任务包 XX，不提前实现后续任务；
4. 先列出准备修改的文件和实现顺序，再开始修改；
5. 严格执行“Eino 官方组件优先；不满足时实现 Eino 标准接口；传统 CRUD/权限/事务不强行 Eino 化”；
6. 使用 Eino 时先以 go.mod 锁定版本源码核对 API，禁止凭记忆伪造组件或方法；
7. 遵守当前错误架构：contracts 定义稳定错误码、apperror 表达协议无关应用错误、httperror 负责 HTTP 映射、api/response 负责 Envelope；
8. Eino 实现放 internal/service/rag，SQL 放 repository，文档任务 Source/Handler 放 internal/worker/document，外部依赖只在 internal/app 组装；
9. 遇到接口或模型选择不明确时，优先使用可注入适配器隔离，并明确报告假设；
10. 完成后运行 gofmt、go test ./...、go vet ./...、go build ./cmd/...、git diff --check；
11. 当前不保留测试文件；不得声称未实际执行的自动化测试或运行态验收已经通过；
12. 最后输出使用了哪些 Eino 组件、哪些自定义 Eino 组件，以及修改文件、实现行为、验证结果、遗留问题和下一包建议。
```

## 18. 成员一最终完成定义（Definition of Done）

只有同时满足以下条件，成员一模块才可标记完成：

- 文档导入、解析、清洗、分段、Embedding、索引和检索全链路可运行；
- 文档加工和混合检索两条生产链路实际由 Eino Compose 编排，不是仅引入依赖或保留示例代码；
- 可复用部分使用 Eino/eino-ext Loader、Parser、Transformer、Embedding 等现成组件；不可复用部分实现 Eino Indexer/Retriever/Transformer/Loader 接口；
- Eino `schema.Document`、Options、Callbacks 和 metadata 约定集中适配，未泄漏到 HTTP DTO、Entity 和跨成员 contracts；
- Eino 实现位于 `internal/service/rag`，数据访问位于 `internal/repository`，文档任务位于 `internal/worker/document`，依赖由 `internal/app` 统一组装；
- 知识库创建在短事务中原子创建知识库、默认目录、搜索配置和 Agent 默认配置；
- Service 使用 `internal/apperror` 和 contracts 错误码，HTTP 状态/公开消息只由 `internal/api/httperror` 映射，Controller 统一使用 `internal/api/response`；
- Markdown、TXT、PDF、DOCX 和 URL 的成功/失败行为明确；
- 关键词、向量、Hybrid、RRF、Reranker 全部可用；
- RetrievalService、DocumentService、CitationService 符合冻结契约；
- 成员三已用真实服务替换 Mock 并完成工具联调；
- 数据归属、软删除、active index 和向量状态过滤完整；
- Worker 支持并发领取、幂等、超时、重试和取消；
- URL 导入具备完整 SSRF 防护；
- 旧索引在新索引失败时仍可用；
- 构建、现有测试、vet 和 diff check 通过；
- 真实依赖环境下完成端到端演示；
- API、数据库、配置和交付文档同步更新；
- 所有未验证项均明确记录，没有用静态实现冒充运行态通过。
