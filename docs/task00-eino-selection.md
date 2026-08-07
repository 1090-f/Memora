# 任务包 00 交付记录 · Eino 组件选型

> 日期：2026-08-05
> 锁定版本：`github.com/cloudwego/eino v0.9.13`（已写入 go.mod/go.sum）
> 依据：`go list -m -versions` 实际核对，非记忆；Go 1.25.0 兼容

## 1. 版本选择

- 在 `v0.3.x ~ v0.10.0-alpha.x` 全版本列表中，选择最新稳定非预览版 **v0.9.13**。
- `eino-ext` 为多 module 仓库，根 module 仅 `v0.0.1-alpha`；各组件（loader/file、splitter/markdown、splitter/recursive、document/loader/url、document/transformer/reranker 等）以伪版本发布，**依赖锁定源码 main 分支 API**（当前目标 `eino v0.6.0`）。
- 决策：**任务包 00 只固定 eino 核心 v0.9.13**；eino-ext 具体子模块与伪版本在任务包 04（Markdown/TXT 链路）引入时，以 `go list -m -json -versions <component>` 核对后锁定，不凭记忆硬编码。
- 风险：eino v0.9.13 与 eino-ext 子模块声明的最低版本（v0.6.0）存在大版本差，任务包 04 引入时若 API 不兼容，需在 go.mod 中显式校验；已记入第 16 节风险门。

## 2. 复用决策矩阵（按计划 §3.2 复核）

| 能力 | 方案 | 状态 |
|---|---|---|
| 本地文件加载 | eino-ext `document/loader/file` | 任务包 04 评估 |
| MinIO/S3 加载 | eino-ext `document/loader/s3`（验证兼容性）或自研 `MinIOLoader` | 任务包 04 评估 |
| URL 加载 | eino-ext `document/loader/url` 仅可注入安全 HTTP Client + SSRF 校验时使用，否则自研 `SafeWebLoader` | 任务包 09 |
| TXT/Markdown 分段 | eino-ext `splitter/markdown`、`splitter/recursive` | 任务包 04 评估 |
| 清洗 | 自研 `document.Transformer` | 任务包 04 |
| Embedding | **`ContractsEmbeddingAdapter` 实现 `embedding.Embedder`**（已交付） | 冻结 |
| pgvector 写索引 | 自研 `PostgresIndexer` 实现 `indexer.Indexer` | 任务包 06 |
| 关键词检索 | 自研 `PostgresKeywordRetriever` 实现 `retriever.Retriever` | 任务包 05 |
| 向量检索 | 自研 `PgVectorRetriever` 实现 `retriever.Retriever` | 任务包 06 |
| RRF | 纯函数 Lambda | 任务包 07 |
| Reranker | eino-ext `document/transformer/reranker` 或 **`ContractsRerankerTransformer`**（已交付） | 冻结/07 |
| 编排 | `compose.Graph`（初始化时 Compile 一次） | 04/07 |

## 3. 锁定版本 API 核实记录（源码 v0.9.13）

以下签名均以 `go list -m -versions` + 模块缓存源码实际核对：

| API | 锁定签名 |
|---|---|
| `schema.Document` | `{ID, Content string; MetaData map[string]any}`，含 `Score()/WithScore()` 等类型化访问器 |
| `document.Loader` | `Load(ctx, document.Source{URI}, ...LoaderOption) ([]*schema.Document, error)` |
| `document.Transformer` | `Transform(ctx, []*schema.Document, ...TransformerOption) ([]*schema.Document, error)` |
| `embedding.Embedder` | `EmbedStrings(ctx, []string, ...Option) ([][]float64, error)` |
| `indexer.Indexer` | `Store(ctx, []*schema.Document, ...Option) ([]string, error)`；`WithEmbedding(emb)` |
| `retriever.Retriever` | `Retrieve(ctx, query string, ...Option) ([]*schema.Document, error)`；`GetCommonOptions`、`WithTopK`、`WithScoreThreshold`、`WithEmbedding`、`WrapImplSpecificOptFn` |
| `compose.NewGraph[I,O]` | `AddLoaderNode / AddTransformerNode / AddIndexerNode / AddRetrieverNode / AddLambdaNode / AddEdge / Compile(ctx, ...GraphCompileOption)` |
| 自定义 Option 注入 | 通过 `WrapImplSpecificOptFn` + `GetImplSpecificOptions` 传租户过滤（UserID/KnowledgeBaseID/IndexVersion） |

**差异记录**：执行计划正文写 `Compile()`，锁定源码实为 `Compile(ctx context.Context, ...GraphCompileOption)`；探针已按源码修正并通过编译。

## 4. 已交付适配器

### `ContractsEmbeddingAdapter`（einoadapter/embedding_adapter.go）
- 包装 `contracts.EmbeddingModel` 实现 `embedding.Embedder`；
- `float32 ↔ float64` 转换、NaN/Inf 校验、返回数量与维度校验；
- 不接触模型密钥，模型只能来自成员二 `ModelFactory`。

### `ContractsRerankerTransformer`（einoadapter/reranker_transformer.go）
- 包装 `contracts.Reranker` 实现 `document.Transformer`；
- query 从文档 MetaData `MetaQuery` 键读取，组件保持无状态、可单例编译；
- Reranker 返回 index 做越界/重复校验，保留输入文档 metadata 并写 `reranker_score`。

### `einoadapter/metadata.go`
- 集中定义 metadata 键常量与安全读写函数（`Get/Set MetaString/Int/Float/Any`），禁止不安全类型断言。

## 5. 待决策门（风险门 §16，需在对应任务包冻结）

1. Embedding 模型与维度（任务包 06）；
2. Reranker 服务与失败降级策略（任务包 07）；
3. 中文分词库（任务包 05）；
4. Chunk Token 计算方式/大小/重叠（任务包 04）；
5. PDF/DOCX 解析库（任务包 09）；
6. URL 导入允许范围与最大响应大小（任务包 09）；
7. 旧索引保留数量与清理时机（任务包 06）；
8. 测试文件规则（当前不保留，需用户确认）；
9. eino-ext 子模块伪版本与升级策略（任务包 04）。
