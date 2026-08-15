# 任务包 04 交付记录 · Eino Markdown/TXT 最小文档加工链路

> 日期：2026-08-05
> 范围：成员一任务包 04。基于 Eino 的 Markdown/TXT 文档加工链路（Loader → Transformer → Indexer/持久化）。
> 依赖：eino v0.9.13、eino-ext markdown/recursive splitter（伪版本 v0.0.0-20260803030130-90a15623ddb6，已锁定）

## 1. 修改文件清单

**新增**：
- `internal/service/rag/loader/minio_loader.go`（`MinIOLoader` 实现 `document.Loader`）
- `internal/service/rag/transformer/cleaner.go`（`Cleaner` 实现 `document.Transformer`）
- `internal/service/rag/transformer/chunk_enricher.go`（`ChunkEnricher`：编号/计数/heading_path/chunk_config_hash）
- `internal/service/rag/transformer/heading_enricher.go`（组合 Markdown Header + Recursive Splitter）
- `internal/service/rag/transformer/metadata_init.go`（`MetadataInitTransformer` + `DocMeta`）
- `internal/service/rag/pipeline/document_pipeline.go`（文档加工 Graph，初始化时 Compile）
- `internal/model/entity/document_chunk.go`
- `internal/repository/document_chunk_repository.go`（批量插入短事务）
- `internal/service/document_processor_interface.go`

**修改**：
- `internal/repository/document_interface.go`（`DocumentChunkRepository` 接口）
- `internal/service/document_process_service.go`（`ProcessImportTask` 接入 Graph + 状态流转）
- `internal/service/document_process_interface.go`（注释更新）
- `internal/app/worker.go`（构造 pipeline + `defaultChunkConfig`）
- `internal/app/server.go`（`NewDocumentProcessService` 新签名）
- `go.mod`/`go.sum`（锁定 eino-ext splitter）

## 2. 已实现行为

### 文档加工 Graph（初始化时 Compile 一次）
```text
Input(ProcessInput{ObjectKey, DocMeta})
→ load（Lambda：MinIOLoader 读 MinIO → []*schema.Document + 注入业务元数据）
→ clean（Cleaner Transformer）
→ split（HeadingEnricher：Markdown Header Splitter → Recursive Splitter）
→ enrich（ChunkEnricher：chunk_no/char_count/token_count/heading_path/context_title/chunk_config_hash）
→ persist（Lambda：ChunkWriter 批量落库 document_chunks）
→ Output(ProcessOutput{ChunkCount})
```

### 关键实现
- **MinIOLoader**：流式读取 MinIO 对象（不 ReadAll），按扩展名走 Eino ExtParser（fallback TextParser），实现 `document.Loader` 接口；eino-ext FileLoader 只支持本地文件故未采用。
- **分段策略**：Markdown 按 `#~#####` 标题切分（标题层级组合为 `heading_path` 数组）；超长块按 Recursive Splitter（chunk_size=1000 字符、overlap=100，rune 计数，中英文分隔符）细分；标题上下文随 metadata 保留。
- **chunk_config_hash**：`sha256(ChunkConfig JSON)` 稳定哈希，写入每个 Chunk 与文档行。
- **active_index_version 安全切换**：加工开始只置 processing_status=parsing；全部 Chunk 成功落库后才更新 active_index_version + succeeded + chunk_config_hash；失败时旧版本继续可用。
- **失败处理**：Graph 失败 → 任务 failed + 文档 processing_status=failed/failure_step=failure_reason，旧索引不受影响。
- **幂等**：同任务重复执行复用文档行，不重复创建；同版本 chunk 冲突 DoNothing。
- **状态**：parsing → succeeded/failed（Worker 在 Graph 外更新）。

### 验证
- **运行态探针（已执行并通过）**：`go run -tags probe ./cmd/probe04/` → `PROBE OK: 2 chunks produced`——Graph 编译通过，Markdown 样例成功产出 2 个 Chunk（探针已完成使命删除）。
- `go build ./...`、`go vet ./...`、`go test ./...`、`go build ./cmd/...`、`git diff --check`：全部通过。
- **真实环境验收未执行**：需 PostgreSQL + MinIO 验证完整落库链路。

## 3. 决策与偏差记录

1. **eino-ext FileLoader 未采用**：只支持本地文件（os.Open），不满足 MinIO；自研 `MinIOLoader` 实现 `document.Loader` 接口。
2. **Loader 以 Lambda 入图**：Eino Graph 节点类型流需要类型一致，用 Lambda 适配 `ProcessInput → []*schema.Document`，Loader 仍为 Eino Loader 组件（在 Lambda 内被调用）；Cleaner/Splitter/Enricher 为真实 `AddDocumentTransformerNode`。
3. **加工状态粒度**：Graph 内不可干预状态，仅 parsing（开始）与 succeeded/failed（结束）两态，未细分 cleaning/chunking（计划 §8 步骤 7 允许的妥协，记为待优化）。
4. **TXT 无标题**：heading_path 为空，context_title 回退到文档标题。
5. **关键词索引**：该历史实现曾写入 `fts_tokens`；现已由迁移 000020 改为 ParadeDB 直接索引 Chunk 原文。
6. **server 进程 processor=nil**：HTTP 服务不执行加工（Worker 专属），任务包 03 行为保留。
7. **手工文档（source_type=manual）**：正文在 DB 不在 MinIO，本包不加工（停留 pending），后续任务包处理。

## 4. 风险 / 待协作

- 分段参数（1000/100）为保守默认值，`defaultChunkConfig` 变更需触发重新索引。
- 中文分词器（任务包 05）、向量（任务包 06）未接入。
- chunk_config_hash 与文档行的 `chunk_config_hash` 需保持一致（通过 `ChunkConfigHash()` 暴露，已接入）。
- eino-ext 伪版本与 eino v0.9.13 兼容性已用编译探针验证（splitter API 稳定）。

## 5. 复查修正记录（2026-08-05 第二轮审查）

| # | 问题 | 严重度 | 修复 |
|---|---|---|---|
| 1 | `Cleaner.cleanText` 行级 TrimSpace 破坏 Markdown 代码块/缩进代码与列表语义（探针实证缩进丢失） | 严重 | 改为不做行首 TrimSpace，仅清理行尾空白与合并空行；缩进交给分段器处理 |
| 2 | `BatchInsert` 的 `inserted=len(chunks)` 不反映真实插入数（DoNothing 冲突时虚报成功） | 重要 | 用 `result.RowsAffected` 返回实际插入数 |
| 3 | `ChunkEnricher` chunk_no 用过滤前索引（空白 chunk 导致编号空洞） | 重要 | 用过滤后 `len(out)` 连续编号，ID 与 chunk_no 一致 |
| 4 | `TrimHeaders: false` 标题行重复进入多个 chunk 正文 | 重要 | 改 `TrimHeaders: true`（标题已入 heading_path metadata） |
| 5 | 重试/部分失败后同索引版本 Chunk 残留 | 重要 | `processDocument` 加工前调用 `DeleteByVersion` 清理同版本残留 |
| 6 | **双重 Fail**：`markProcessFailed` 标任务 failed 后 Runner 的 Source.Fail 再调 FailTask → WHERE running 不命中报错 | 严重 | 拆分 `markDocumentFailed`（只更新文档），任务状态由 Runner Fail 路径统一回写 |

确认无问题项：
- MinIOLoader 无独立超时，但 Worker job ctx 30min 已覆盖；
- `objectKey` TrimPrefix 异常调用安全（无前缀则原样传递）；
- Loader metadata 被 injectMeta 覆盖无冲突；
- Cleaner 行尾清理不影响分词（空白是分隔符）。
- 探针 `go run -tags probe ./cmd/probe04b/`：代码块缩进保留 + chunk_no 连续编号均通过（探针完成后删除）。

## 6. 下一步

任务包 05：中文关键词字段与全文索引（ChineseTokenizer + PostgresKeywordRetriever）。
