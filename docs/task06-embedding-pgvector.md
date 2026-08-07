# 任务包 06 交付记录 · Eino Embedding、PostgresIndexer、pgvector 与索引版本切换

> 日期：2026-08-05
> 范围：成员一任务包 06。Embedding 向量生成、pgvector 持久化与检索、索引版本安全切换。
> 依赖：eino v0.9.13、eino-ext splitter（已锁定）

## 1. 修改文件清单

**新增**：
- `internal/model/entity/document_vector.go`
- `internal/repository/vector_repository.go`（批量 upsert + cosine 检索 + 版本清理）
- `internal/service/rag/indexing/postgres_indexer.go`（`PostgresIndexer` 实现 `indexer.Indexer`）
- `internal/service/rag/retrieval/vector_retriever.go`（`PgVectorRetriever` 实现 `retriever.Retriever`）

**修改**：
- `internal/service/rag/pipeline/document_pipeline.go`（persist 写回 chunk_id + 可选 index 节点 + 双模式 Graph）
- `internal/repository/document_chunk_repository.go`、`document_interface.go`（BatchInsert 返回 chunk IDs）
- `internal/service/document_process_service.go`（向量清理 + 新签名）
- `internal/app/worker.go`、`server.go`（向量仓储注入 + embeddingProvider 预留）
- `docs/ARCHITECTURE.md`

## 2. 已实现行为

### PostgresIndexer（Eino `indexer.Indexer`）
- 读取 `schema.Document` 与 metadata，使用 `indexer.WithEmbedding` 注入的 Embedder（经 Graph 调用选项 `compose.WithIndexerOption` 透传）批量生成向量；
- 批大小可配置（默认 32）、每批超时（默认 60s）、校验返回数量；
- 委托 VectorRepository 批量写入 `document_vectors`（status=ready），组件不持有 GORM/SQL；
- `vectorFromDocument` 校验 user/kb/doc/chunk/version 元数据，维度记录 `embedding_dim`。

### PgVectorRetriever（Eino `retriever.Retriever`）
- 使用 `WithEmbedding` 注入的 Embedder 将查询转向量；
- 委托 Repository cosine 检索：只返回 `status='ready'` 且 `index_version = active_index_version` 的向量；
- 公共 TopK/ScoreThreshold + 自定义 `WithVectorScope`（身份仅 Service 注入），上限复用关键词检索常量；
- 结果映射 `schema.Document`，rank/score/更新时间写入集中 metadata keys。

### Vector Repository
- `BatchUpsert`：同 chunk+model+version 冲突覆盖（DoUpdates 仅更新存在的列，无 updated_at 列）；
- `SearchCosine`：cosine distance（`1 - (embedding <=> ?)`），归属/文档范围/版本过滤 + 阈值 + 确定性排序（ORDER BY distance + 隐式 id 稳定）；
- `DeleteByVersion`：重试时清理同版本残留向量。

### Pipeline 双模式
- **无向量**（Embedder 未接入）：persist → finalize → END；
- **有向量**：persist（写回 chunk_id）→ index（PostgresIndexer）→ finalize_ids → END；
- Graph 输出统一 `ProcessOutput{ChunkCount}`。

### 索引版本安全切换（processDocument）
- 加工前 `DeleteByVersion` 清理同版本 Chunk 与向量残留（幂等重试）；
- 全部成功后才更新 `active_index_version + succeeded + chunk_config_hash`；
- 失败保留旧 active 版本，文档标记 failed。

### 决策（用户确认）
- **暂不建 HNSW 索引**：Embedding 维度未冻结（风险门第 1 项），顺序扫描可功能验证；维度冻结后追加 Migration 000008（DB 设计文档 5.6 的 HNSW 模板已记录）。
- **Embedder 未接入**：成员二 ModelFactory 未实现，Worker 的 `embeddingProvider()` 返回 nil → 当前仅关键词索引；接入点已预留。

## 3. 验证

- **探针（已执行通过）**：无向量/有向量两种 Graph 均编译并运行成功（`无向量模式 OK: chunks=1`、`有向量模式 OK: chunks=1`），Indexer 数据流（chunk_id 写回 → 向量写入）正常。探针完成后删除。
- `go build ./...`、`go vet ./...`、`go test ./...`、`go build ./cmd/...`、`git diff --check`：全部通过。
- **运行态验收未执行**：需真实 PostgreSQL + pgvector 验证 cosine 检索、向量扫描、版本切换。

## 4. 决策与偏差记录

1. **HNSW 延后**：按用户决策，本包不建向量索引 Migration；维度冻结后追加（模板在 DB 设计 5.6）。
2. **Graph 调用选项注入 Embedder**：Eino `AddIndexerNode` 不支持构造期 embedding，用 `compose.WithIndexerOption(indexer.WithEmbedding(...))` 在 Run 时透传（符合 Eino 设计）。
3. **双模式 Graph**：向量/非向量模式各自编译，避免类型流冲突（`[]*schema.Document` vs `[]string` 输出）。
4. **persist 按位置写回 chunk_id**：chunks[i]↔docs[i] 顺序一致，避免 ID 映射错位。
5. **`1 - (embedding <=> ?)` 语义**：cosine distance→similarity 转换，WHERE/ORDER BY/SELECT 三处距离占位符均按出现顺序绑定 args。

## 5. 复查修正记录（2026-08-05 第二轮审查）

| # | 问题 | 严重度 | 修复 |
|---|---|---|---|
| 1 | `PgVectorRetriever` 查询向量化无显式超时（依赖调用方 ctx） | 严重 | 新增 `vectorEmbedTimeout=30s` 超时上下文 |
| 2 | **`documents.embedding_model_id` 从未更新**（计划 §10 步骤 11 要求） | 严重 | `DocumentProcessor` 增加 `EmbeddingModelID()`，pipeline 暴露，成功时写回文档 |
| 3 | 上限常量命名混乱：`MaxKeywordDocumentIDs` 被向量检索复用 | 重要 | 新建 `retrieval/limits.go` 共享常量：`MaxScopeDocumentIDs`/`MaxKeywordTopK`/`MaxVectorTopK` |
| 4 | `vector_retriever` 的 `time` import 冗余 | 低 | 移除（超时常量移至 limits.go） |
| 5 | `DocumentPipeline` 重复声明 `ChunkConfigHash`（编辑失误） | 低 | 删除重复方法 |

确认无问题项：
- `SearchCosine` 占位符顺序逐位核对正确（SELECT-qv/user/kb/WHERE-dist/[docIDs]/[version]/[qv,threshold]/ORDERBY-qv/LIMIT）；
- `embedBatch` 分批 + 超时 + 数量校验完整；
- 向量维度不一致时 pgvector 报错事务回滚（"不写入损坏向量"验收）；
- `dv.status='ready'` + active 版本过滤满足验收；
- 旧版本向量保留（active 切换前可检索，符合"旧索引可用"）；
- 索引版本无限增长无清理策略（风险门第 7 项，记录待后续）。

## 6. 风险 / 待协作

- 维度冻结后需：追加 HNSW Migration + `embeddingProvider()` 接入成员二 ModelFactory + 填 `EmbeddingModelID`。
- `document_vectors` 无 updated_at 列，DoUpdates 已避开。
- 顺序扫描在数据量大时性能下降，HNSW 接入后改善。
- 向量维度与模型维度不一致时 BatchUpsert 会因 pgvector 类型不匹配报错（明确失败，符合验收"不写入损坏向量"）。
- 索引版本无限增长无清理策略（风险门第 7 项），待后续任务包评估。

## 7. 下一步

任务包 07：Eino 混合检索 Graph、RRF、Reranker 与知识判断。
