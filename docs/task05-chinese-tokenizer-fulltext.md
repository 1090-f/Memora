# 任务包 05 交付记录 · 中文关键词字段与全文索引

> **历史方案说明**：本文记录的应用层 N-gram + PostgreSQL `tsvector` 已被迁移 000020 取代。当前实现由 ParadeDB `pg_search` 直接索引原文，Go 侧 N-gram 已删除；本文仅用于追溯早期决策。

> 日期：2026-08-05
> 范围：成员一任务包 05。中文 fts_tokens 生成 + PostgreSQL 全文检索 + PostgresKeywordRetriever。
> 依赖：eino v0.9.13、eino-ext splitter（任务包 04 已锁定）

## 1. 修改文件清单

**新增**：
- `internal/service/rag/tokenizer/tokenizer.go`（可替换分词内核：N-gram + 英文切分）
- `internal/service/rag/transformer/chinese_tokenizer.go`（`ChineseTokenizerTransformer` 实现 `document.Transformer`）
- `internal/service/rag/retrieval/keyword_retriever.go`（`PostgresKeywordRetriever` 实现 `retriever.Retriever`）
- `internal/repository/keyword_search_repository.go`（参数化全文检索）

**修改**：
- `internal/service/rag/einoadapter/metadata.go`（新增 `MetaFTSTokens` 常量）
- `internal/service/rag/pipeline/document_pipeline.go`（接入 tokenize 节点 + fts_tokens 落库）
- `docs/ARCHITECTURE.md`（登记 tokenizer/retrieval 目录）

## 2. 已实现行为

### 分词器（可替换内核）
- `tokenizer.Tokenizer` 普通 Go 接口 + `NgramTokenizer` 确定性实现：
  - 中文（CJK）按 1-2 gram 切分（unigram + bigram）；
  - 英文/数字连续串保留为整体并转小写；
  - 中英混合段分别处理（如 `go语言` → `go` + 语言/言中/中文 ngram），无噪声 token；
  - 单字符英文/数字过滤，中文单字保留；
  - Token 去重、去空。
- **选型决策**：不引入 gse（词典约 30MB：dict 23.7MB + idf 6.2MB）/gojieba（cgo），采用零依赖确定性 N-gram，满足 PostgreSQL `simple` 配置检索需求；内核可替换。

### 中文分词 Transformer
- `ChineseTokenizerTransformer` 实现 Eino `document.Transformer`：分词结果空格连接写入 metadata `fts_tokens`；空结果报错（防止空索引）。

### Pipeline 集成
- 文档加工图新增 `tokenize` 节点（enrich 之后、persist 之前）；persist 节点校验并写入 `document_chunks.fts_tokens`。

### 全文检索
- `KeywordSearchRepository.Search`：参数化 SQL，过滤 `user_id + knowledge_base_id + 可选 document_ids + documents.deleted_at IS NULL + index_version = active_index_version`；tsquery 用 OR 连接 Token 提升召回；`ts_rank` 排序 + Chunk ID 确定性 tie-breaker；LIMIT TopK。
- `PostgresKeywordRetriever`：实现 Eino `retriever.Retriever`，只做 options/metadata 适配：
  - 公共 `WithTopK`/`WithScoreThreshold`；
  - 自定义 `WithKeywordScope`（UserID/KBID/DocumentIDs/IndexVersion，仅 Service 注入）；
  - 结果映射 `schema.Document`，rank/score/版本写入集中 metadata keys；
  - Callbacks 记录耗时与结果数，不记录 Query 全文/正文。

## 3. 验证

- **分词探针（已执行通过）**：`Eino ReAct 如何调用工具` → `[eino react 如 何 调 用 工 具 如何 何调 调用 用工 工具]`；`go语言中文分词测试` → `[go 语 言 中 文 分 词 测 试 语言 言中 中文 文分 分词 词测 测试]`；`Hello World 2026` → `[hello world 2026]`。探针完成后删除。
- `go build ./...`、`go vet ./...`、`go test ./...`、`go build ./cmd/...`、`git diff --check`：全部通过。
- **运行态检索验收未执行**：需真实 PostgreSQL 验证 GIN 索引使用、uuid 扫描、ts_rank 排序。

## 4. 决策与偏差记录

1. **分词器自研（零依赖）**：gse 词典 ~30MB（dict 23.7MB + idf 6.2MB），gojieba 需 cgo；N-gram 对 `simple` 配置检索足够且确定性可控。计划 §16 风险门第 3 项"中文分词库"以本记录冻结。
2. **tsquery 用 OR**：提升中文 N-gram 召回（bigram 稀疏），rank 保证排序质量。
3. **检索器暂未接线**：任务包 07 混合检索 Graph 接入 `PostgresKeywordRetriever`；本包已完成接口与独立编译/调用能力。
4. **`IndexVersion` 选项**：为 nil 时 SQL 用 `active_index_version`；显式传入可检索指定版本（测试用）。
5. **SQL 中 `to_tsquery` 参数占位顺序**：已按 SELECT 先出现原则调整 args 顺序，避免错位。

## 5. 复查修正记录（2026-08-05 第二轮审查）

| # | 问题 | 严重度 | 修复 |
|---|---|---|---|
| 1 | TopK 无上限，恶意大参数可致全表扫描 | 严重 | 新增 `MaxKeywordTopK=100`、`MaxKeywordDocumentIDs=200` 上限校验 |
| 2 | ScoreThreshold 过滤后 rank 序号断裂（1,3,5 空洞） | 重要 | 用实际加入数量 `len(docs)+1` 作 rank |
| 3 | `MetadataInitTransformer` 死代码（元数据注入在 load Lambda 内联完成） | 重要 | 删除 Transformer 实现，保留 `DocMeta` 结构 |
| 4 | 纯符号 Chunk（`---`/`===`）分词零 Token 导致整个文档失败 | 严重 | 改为过滤该 Chunk，不入库而非报错 |
| 5 | `char_count`/`token_count`/`context_title` 字面量键散落 | 低 | 补 `MetaCharCount/MetaTokenCount/MetaContextTitle` 常量统一 |
| 6 | keyword search 未用 `dbFromContext`（事务一致性） | 低 | 统一走 `dbFromContext` |
| 7 | 检索结果未携带文档更新时间 | 低 | metadata 写入 `MetaDocumentUpdAt` |

确认无问题项：
- SQL 占位符顺序：SELECT tsquery/user/kb/WHERE tsquery/docIDs/LIMIT 与 args `[query,userID,kbID,query,...docIDs,topK]` 逐位匹配正确；
- tsquery 用 `|` 连接 N-gram token，`simple` 配置下中文单字/bigram 均为合法词位；
- 分词器输出仅含字母数字中文，无 tsquery 语法字符注入风险；
- Loader 写入的 `MetaDocumentID`（对象 key）随后被 injectMeta 覆盖为真实文档 ID，无冲突；
- `heading_path` 为 `[]string` 写入，`headingPathOf` 类型断言匹配；
- TXT 无标题时整个文本一段，Recursive 按 1000 字符稳定切分。

## 6. 风险 / 待协作

- pgx uuid→string 扫描、numeric→float64 的 ts_rank 映射需真实 DB 验证。
- 查询 Token 若被 tsquery 特殊字符污染（N-gram 已过滤字母数字中文，风险低），可在 Repository 层防御。
- fts_vector 是 GENERATED STORED 列，fts_tokens 写入后自动生成，无需额外索引步骤。
- 全符号文档（所有 Chunk 被过滤）最终 ChunkCount=0 → 文档标记 failed；记录为决策，后续可改 succeeded。

## 7. 下一步

任务包 06：Eino Embedding、PostgresIndexer、pgvector 与索引版本切换。
