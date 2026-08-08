# 任务包 00 交付记录 · 契约冻结（成员一）

> 日期：2026-08-05
> 仓库：Memora
> 范围：成员一 RAG 与知识处理模块的契约冻结、错误码映射与 Eino 组件选型
> 基线：`go1.25.0`，Eino `v0.9.13`（go.mod 已固定）

## 1. 契约冻结结论

跨模块对象只能复用 `internal/contracts`。本次在**纯追加、不删除/不改名已有字段**的前提下冻结成员一相关契约。

### 1.1 本次冻结（未改动，保持现状）

| 契约 | 结论 |
|---|---|
| `contracts.RetrievalService` / `RetrievalRequest` | 冻结，未改动 |
| `contracts.DocumentService` / `DocumentReadRequest` | 冻结，未改动 |
| `contracts.EmbeddingModel` / `Reranker` / `ModelFactory` | 冻结，成员二所有 |
| `contracts.ErrorCode` 主体 | 冻结（见 §3 错误码映射） |
| `contracts.RetrievalMode`（keyword / vector / hybrid） | 冻结，以 `vector` 为准 |

### 1.2 本次追加（兼容性扩展）

| 文件 | 新增字段/类型 | 依据 |
|---|---|---|
| `retrieval.go` | `SearchConfig.RerankerModelID` | API 6.x、DB `search_configs.reranker_model_id` |
| `retrieval.go` | `RetrievalItem`：`DocumentTitle / DirectoryID / SourceLocation / KeywordScore / VectorScore / RRFRank / RerankerScore / FinalRank / DocumentUpdatedAt`（均 omitempty） | API 9.1 结果项 |
| `retrieval.go` | `RetrievalResult`：`Query / Mode / ElapsedMS`（omitempty） | API 9.1 |
| `citation.go` | `Citation.KnowledgeBaseID`（omitempty） | API 12.6 知识库引用 |
| `document_status.go`（新增） | `DocumentSourceType`、`DocumentProcessingStatus` 枚举 | DB `000004` 的 `source_type` / `processing_status` |

> 说明：`RetrievalItem.Score` 由 `*float64` 语义保持 `float64` 不变；新增分数/排名字段均为可选指针，避免破坏已有调用方。

## 2. 接口冻结说明（给成员二/三）

### 2.1 调用方向

- 成员三：`KnowledgeSearchTool` → `contracts.RetrievalService.Retrieve`；`DocumentReadTool` → `contracts.DocumentService.Read`。
- 成员二：`ModelFactory.GetEmbeddingModel / GetReranker` 提供给成员一适配为 Eino 组件。
- **Eino 不外泄边界**：`schema.Document`、Eino Options、Callbacks、metadata 只在 `internal/service/rag`（含 `einoadapter`）内部交换；HTTP DTO、Entity 与跨成员 contracts 不得出现 Eino 类型。
- 成员一 Mock 位于 `internal/service/rag/mock`，成员三无需了解任何 Repository 即可调用。

### 2.2 超时与错误语义

- 所有外部调用（Embedding、Reranker、MinIO）必须设置超时；`contracts.ModelFactory` 返回的模型实例由调用方负责超时与重试边界。
- Service 只返回 `internal/apperror.AppError`（Code 来自 contracts，Cause 保留内部链），不携带 HTTP 状态码。
- 检索无有效依据时返回 `RetrievalResult.KnowledgeStatus = "insufficient"`，默认不转 HTTP 422；仅契约显式要求时才用 `KNOWLEDGE_INSUFFICIENT`。
- 错误码映射：见 §3。

### 2.3 数据归属

- 身份只能来自认证中间件/运行时上下文；Repository 必须在同一查询中组合 `resource_id + user_id + knowledge_base_id + 状态 + 软删除`，禁止先查后验。
- 检索结果不得包含其他用户、其他知识库或已删除文档；向量检索只读 `status='ready'` 且 `index_version = documents.active_index_version` 的向量。

## 3. 错误码映射清单

成员一预期复用 `internal/contracts/error_code.go` 的稳定错误码；HTTP 状态与公开消息只由 `internal/api/httperror` 映射（下表 HTTP 列来自 `httperror/mapper.go` 现状）。

| 场景 | contracts.ErrorCode | HTTP | httperror 已有映射 |
|---|---|---|---|
| 参数非法（查询为空、TopK 超限、配置越界） | `INVALID_ARGUMENT` | 400 | 是 |
| 文件类型不支持 | `UNSUPPORTED_FILE_TYPE` | 415 | 是 |
| 文件/请求过大 | `PAYLOAD_TOO_LARGE` | 413 | 是 |
| 资源不存在 | `RESOURCE_NOT_FOUND` | 404 | 是 |
| 资源重复（重名/重复任务） | `DUPLICATE_RESOURCE` | 409 | 是 |
| 越权访问 | `FORBIDDEN` | 403 | 是 |
| 未认证 | `UNAUTHORIZED` | 401 | 是 |
| 状态非法（任务已结束/文档状态不匹配） | `INVALID_STATE` | 409 | 是 |
| 索引版本冲突 | `INDEX_VERSION_CONFLICT` | 409 | 是 |
| 知识不足（契约显式失败场景） | `KNOWLEDGE_INSUFFICIENT` | 422 | 是 |
| 模型调用失败（Embedding/Reranker） | `MODEL_CALL_FAILED` | 502 | 是 |
| 上游超时（MinIO/模型/网络） | `UPSTREAM_TIMEOUT` | 504 | 是 |
| 服务暂不可用（依赖未就绪） | `SERVICE_UNAVAILABLE` | 503 | 是 |
| 未知内部错误 | `INTERNAL_ERROR` | 500 | 是 |

**结论**：成员一所有预期错误均可复用现有错误码，**无需新增错误码**，无需改动 `httperror/mapper.go` 与 `docs/API.md`。

## 4. 契约一致性审计发现（未在本包处理的记录）

| 类别 | 发现 | 处理 |
|---|---|---|
| 文档差异 | API 9.x 检索 mode 文档写 `semantic`，contracts 用 `vector` | 以 contracts 为准，需与文档同步修订 |
| 成员二/三范围 | SSE 事件命名/集合、`ToolCall/ToolResult` 字段命名、`AgentContext`/`ToolContext` 缺 `network_enabled` | **不改**，记入跨成员风险，由对应成员处理 |
| 成员三范围 | `AgentConfig` 缺业务开关字段（name/system_prompt 等） | **不改**，成员二负责 Agent 配置 |
| 数据库 | `import_tasks.duplicate_policy` 迁移默认 `create_new`，API 文档写 `skip` | 不改 Migration（禁止回写），任务包 03 在代码层显式校验 |
| 数据库 | DB 设计文档未列的 4 个唯一约束（迁移已有） | 不改，后续任务包按迁移约束实现 |
| 文档差异 | API 1.7 未列 `SERVICE_UNAVAILABLE`，但代码已用 | 现状保留，API 文档待补 |

## 5. 交付清单

- 新增：`internal/contracts/document_status.go`
- 修改：`internal/contracts/retrieval.go`、`internal/contracts/citation.go`（纯追加）
- 新增：`internal/service/document_process_interface.go`（`DocumentProcessService`）
- 新增：`internal/service/rag/einoadapter/metadata.go`（metadata 常量 + 安全读写）
- 新增：`internal/service/rag/einoadapter/embedding_adapter.go`（`ContractsEmbeddingAdapter`）
- 新增：`internal/service/rag/einoadapter/reranker_transformer.go`（`ContractsRerankerTransformer`）
- 新增：`internal/service/rag/mock/retrieval_mock.go`（`RetrievalService` Mock）
- 修改：`docs/ARCHITECTURE.md`（登记 `internal/service/rag` 与 `internal/worker/document` 职责）
- 修改：`go.mod` / `go.sum`（固定 `github.com/cloudwego/eino v0.9.13`）

## 6. 验证

- 探针：Eino `document.Loader/Transformer`、`embedding.Embedder`、`indexer.Indexer`、`retriever.Retriever`、`compose.NewGraph/Add*Node/AddEdge/Compile` 均编译通过（探针文件已完成使命后删除）。
- `go build ./...`：通过。
- 自动化行为测试与运行态验收：**未执行**（仓库当前不保留测试文件；`go test ./...` 仅有包编译探测，无测试用例）。
