# Memora 文档处理与分块重构方案 V2.0

> Structured-IR-preserving Canonical Chunking
>
> 基于现有 Memora 实现及 Qavor / DocMind 参考方案修订
> 版本：V2.0 · 2026-08-24
>
> 实施状态：自 2026-09-04 起，Canonical Chunker 已成为唯一生产分块主链路，默认策略为 `auto`；Legacy Chunker 仅用于可选差异对照。

## 摘要

Memora 不改造成 Markdown-only 系统。重构后的核心链路为：

```text
ParsedDocument（结构化事实源）
        ↓ normalize / enrich
CanonicalDocument（Nodes + Markdown View + SourceMap）
        ↓ profile / route / chunk
Chunk（Content + SourceSpans + Strategy Metadata）
        ↓
Keyword Index + Embedding / pgvector
```

本方案保留当前 `ParsedDocument` 中的 Block、Table、Asset、Page、BBox 与对象引用能力，引入 `CanonicalDocument` 作为稳定的分块中间表示。Canonical Markdown 仍然存在，但定位为 CanonicalDocument 的标准文本视图，而不是唯一数据载体。

重构首先解决当前来源映射粒度不足和分块层职责过重的问题，然后在保持现有分块行为可对照、可回滚的前提下，引入 Structured、Paragraph、Recursive 三种基础策略。Parent-Child 仅在基础链路和离线评估成熟后实施。

**最终结论：** Memora 采用“保留结构化事实源的 Canonical-first chunking”，而不是简单的 Markdown-first chunking。

## 1. 背景与当前问题

### 1.1 当前能力基础

当前 `ParsedDocument` 已经是稳定、无 Chunk 概念的解析协议，包含：

- 文档级标题、Markdown、页数和元数据；
- 按阅读顺序排列的 Blocks；
- Block 的类型、正文、Markdown、HeadingPath、Page、BBox 与 DoclingRef；
- 独立 Table 对象，包括 Caption、Headers、Rows、Cells、rowspan/colspan；
- 独立 Asset 对象，包括图片、ObjectKey、OCR、Caption、Description、Page 与 BBox。

当前 `StructureAwareChunker` 能处理标题边界、普通段落、代码、公式、表格、图片、Caption 和 XLSX 特例。这说明 Memora 的问题不是“没有结构”，而是分块器直接依赖解析协议，承担了过多结构解释和来源回填职责。

### 1.2 需要解决的真实问题

1. **分块层与解析层耦合较深。** Chunker 直接遍历 Blocks、Tables、Assets，并理解 Parser 的引用关系。
2. **来源定位粒度不足。** 当前 Chunk 只有一个主要 Page/BBox；多 Block、多页合并和超长拆分不能表达精确的片段来源。
3. **跨格式策略扩展成本较高。** 当前主要依赖一个结构感知策略，只有 XLSX 有显式特化。
4. **统一文本表示不稳定。** 文档级和 Block 级虽然已有 Markdown 字段，但不同 Parser 的生成语义并未形成可版本化的 Canonical 契约。
5. **策略效果缺乏统一评估基线。** 当前没有金标检索集和稳定的策略对比工具。

### 1.3 现阶段已知正确性问题

以下问题应在新策略比较前修复或通过测试明确锁定：

- 多来源 Chunk 只保留一个 Page/BBox；
- 超长文本拆分后，各子块复制整组 BlockIDs，缺少块内字符范围；
- `repeat_table_header` 配置参与哈希，但当前实现始终重复表头；
- 超长表格单行按完整 MaxTokens 拆分后再追加表头，最终仍可能超限；
- 当前启发式 tokenizer 未与实际 Embedding 模型严格对齐；
- 同一文档并发重处理可能竞争相同的下一索引版本。

## 2. 设计目标与非目标

### 2.1 设计目标

- 保留 `ParsedDocument` 作为解析层唯一结构化事实源；
- 建立跨格式稳定、可版本化的 CanonicalDocument；
- 让分块策略依赖通用语义节点，而不是 Parser 内部细节；
- 支持多页、多 Block、多资产的精确来源映射；
- 保持表格、图片、代码和公式的专用处理能力；
- 支持策略自动选择、人工覆盖、决策记录与版本重建；
- 重构过程可影子运行、可对比、可回滚；
- 用离线指标证明策略变化，而不是只比较代码结构。

### 2.2 非目标

- 不删除或弱化 `ParsedDocument`；
- 不将 Markdown 视为完整、无损的文档模型；
- P0/P1 阶段不引入 LLM Semantic Chunking；
- 不在第一阶段实现复杂的智能 Router；
- 不在来源映射和基础检索评估完成前实现 Parent-Child；
- 不要求一次性修改所有已有文档和索引。

## 3. 设计原则

| 原则 | 说明 |
|---|---|
| 结构事实与文本视图分离 | ParsedDocument 保存解析事实，Markdown 负责统一展示和通用文本处理 |
| Chunker 不重新解析 | 标题、表格、图片类型由 Canonical Nodes 明确表达，不从字符串重新猜测 |
| 来源是一等数据 | Chunk 的来源不是附属 Page 字段，而是多个可求交、可追踪的 SourceSpan |
| 生成内容必须可识别 | 重复表头、标题前缀、行范围等标记为 generated，不伪装成原文 |
| 策略必须可版本化 | Renderer、Normalizer、Router、Chunker、Tokenizer 的变化均能触发正确层级重建 |
| 迁移必须可对照 | 新旧分块器在相同输入上并行对比，未达验收标准前不切换生产路径 |
| 先确定性后智能化 | 先完成结构、段落和递归策略，再评估语义分块或模型路由 |

## 4. 目标架构

```text
Document Import
      ↓
Parser Router
      ↓
ParsedDocument ───────────────→ Parsed Artifact
      ↓ normalize / OCR / enrich
Canonical Renderer
      ├──────────────→ Canonical Markdown View
      ├──────────────→ Canonical Nodes
      └──────────────→ SourceMap
      ↓
Canonical Validator
      ↓
Document Profiler
      ↓
Chunk Strategy Router
   ┌──────────┼───────────┐
   ↓          ↓           ↓
Structured  Paragraph  Recursive
   └──────────┼───────────┘
              ↓
Specialized Splitters
Table / Code / Formula / Picture
              ↓
Chunk + SourceSpans + Decision Metadata
              ↓
Chunk Clean / Token Count / Persist
              ↓
Keyword Index + Embedding / pgvector
```

### 4.1 职责边界

| 组件 | 负责 | 不负责 |
|---|---|---|
| Parser | 恢复原始文档结构和对象关系 | 决定 Chunk 边界 |
| Normalizer/Enricher | 文本规范化、OCR 与资产描述增强 | 选择分块策略 |
| Canonical Renderer | 生成通用语义节点、Markdown 视图和来源映射 | 根据检索效果切块 |
| Profiler/Router | 提取特征并选择策略 | 修改原始结构 |
| Chunker | 决定检索粒度和上下文重叠 | 重新猜测表格或图片对象 |
| Source Mapper | 将 Chunk 区间映射回原始来源 | 生成检索文本 |
| Indexer | 建立关键词和向量索引 | 改变 Chunk 内容 |

## 5. 核心数据模型

### 5.1 CanonicalDocument

```go
type CanonicalDocument struct {
    SchemaVersion   string
    RendererVersion string

    Markdown string          // 标准文本视图，不是唯一事实源
    Nodes    []CanonicalNode // 有序语义节点
    SourceMap []SourceSpan
    Profile  DocumentProfile

    ContentHash string
}
```

### 5.2 CanonicalNode

```go
type CanonicalNode struct {
    ID          string
    Kind        NodeKind
    StartByte   int
    EndByte     int

    Text        string
    Markdown    string
    HeadingPath []string

    BlockIDs  []string
    TableRef  string
    AssetRefs []string
    Sources   []SourceRef

    Atomic    bool
    Generated bool
    Metadata  map[string]any
}
```

首版 NodeKind 至少包括：

```text
heading / paragraph / list_item / code / formula
table / table_row / picture / caption
footnote / page_header / page_footer / unknown
```

`Atomic=true` 表示优先保持完整；超限时调用对应专用 splitter，而不是普通递归切分。

### 5.3 SourceRef 与 SourceSpan

```go
type SourceRef struct {
    BlockID   string
    TableRef  string
    AssetRef  string
    Page      int
    BBox      []float64
    DoclingRef string
}

type SourceSpan struct {
    StartByte int
    EndByte   int
    Sources   []SourceRef
    Generated bool
    Reason    string // heading_prefix / repeated_header / row_range / overlap 等
}
```

规则：

- offset 统一使用 UTF-8 byte offset；
- 区间采用 `[start, end)`；
- SourceMap 必须支持一对多、多对一和空来源生成区间；
- 规范化和渲染产生的新文本必须标记 `Generated`；
- Chunk overlap 保留真实来源，同时标记重复原因；
- 表格重复表头和自动行范围不能冒充唯一原文位置。

### 5.4 Chunk

```go
type Chunk struct {
    Content       string
    HeadingPath   []string
    TokenCount    int
    SourceSpans   []SourceSpan

    Strategy      string
    StrategyVersion string
    Level         string // leaf / parent，P2 后启用
    ParentID      string // P2 后启用

    BlockIDs      []string // 兼容冗余字段
    TableRefs     []string
    AssetRefs     []string
}
```

首阶段保留 `BlockIDs/TableRefs/AssetRefs`，便于兼容现有 API；`SourceSpans` 成为更精确的事实字段。

## 6. Canonical Renderer 规则

| ParsedDocument 内容 | Canonical Node / Markdown View | 来源规则 |
|---|---|---|
| HeadingPath / Heading | `# / ## / ###` 标题节点 | 使用 Parser 已恢复的真实层级，不重新正则猜测 |
| Paragraph | 普通 Markdown 段落 | 保留 BlockID、Page、BBox |
| ListItem | `-` 或有序列表节点 | 保留列表顺序和层级 metadata |
| Code | 围栏代码块 | 整体为 Atomic Node |
| Formula | 块级公式或原始表达 | 整体为 Atomic Node |
| Table | GFM 视图 + Table Node | TableRef 指向完整 Table 对象，Markdown 不替代 Cells |
| Picture | 图片占位 + Caption/OCR/Description | AssetRef 指向资产；生成描述需标记来源类型 |
| Caption | Caption Node | 保留与 Table/Picture 的显式关联 |
| Header/Footer | 默认不进入检索正文 | 来源保留，策略可配置是否包含 |
| Unknown | 原文节点 | 禁止静默丢弃 |

### 6.1 表格要求

- Table Node 必须保留 headers、rows、cells、rowspan/colspan 和 table ID；
- 小表可作为整体 Atomic Node；
- 大表按行切分，每个子块可重复 Caption 和表头；
- 重复表头及“第 a-b 行”为 generated span；
- 单行超限时，预算必须扣除标题、Caption、表头和行范围前缀；
- XLSX 必须保持 Sheet/Table 独立性；后续应补充稳定的 sheet name/sheet ID。

### 6.2 图片要求

- OCR、Caption、Description 分别记录内容来源，不只拼成匿名字符串；
- Asset ID、ObjectKey、Page、BBox 不进入 Markdown 文本，但保留在 Node 和 SourceMap；
- 无文字图片可以不生成检索 Chunk，但资产仍保留；
- 与邻近正文关联时记录关联原因和距离；
- 关联范围默认要求 HeadingPath 相同且页码距离不超过配置阈值。

## 7. 分块策略

### 7.1 Structured Chunker（默认主策略）

适用于标题结构可靠的文档。

1. 按标题树和一级章节建立硬边界；
2. 标题路径作为上下文前缀并计入 Token 预算；
3. 章节内按 Node 顺序贪心聚合；
4. Atomic Node 优先完整保留；
5. 超长 Atomic Node 调用专用 splitter；
6. 超短块仅在同一章节、来源相邻且不跨 Atomic 边界时合并；
7. overlap 仅用于长文本内部拆分，不跨结构边界。

### 7.2 Paragraph Chunker

适用于标题较少但段落边界可靠的文章、网页正文、小说和部分 OCR 文档。

- 按段落累积至目标 Token 区间；
- 超限时优先在句子边界切分；
- 不把表格、代码、公式混入普通段落聚合；
- 可使用相邻段落小范围 overlap，但必须保留来源区间。

### 7.3 Recursive Fallback Chunker

适用于纯 TXT、弱结构和低质量 OCR。

```text
\n\n → \n → 句末标点 → 空格 → Token 硬切
```

每一级分隔都必须使用同一 Tokenizer 预算，并生成准确的 Canonical byte span。

### 7.4 Specialized Splitters

以下 splitter 由多种策略共享：

- TableSplitter；
- CodeSplitter；
- FormulaSplitter；
- PictureContextAssembler；
- TokenAwareTextSplitter。

它们消费 Canonical Node，而不是重新扫描 Markdown 字符串判断类型。

### 7.5 Parent-Child（后续增强）

仅对长文档且标题结构明显的文档开启：

- Child 用于向量和关键词精准召回；
- Parent 保存章节级上下文；
- 命中 Child 后按配置扩展 Parent；
- Parent 不应与 Child 在最终排序中重复占位；
- 必须同时设计数据库关系、检索展开、去重、Token 预算与版本兼容。

## 8. DocumentProfile 与策略 Router

### 8.1 Profile 特征

首版采用确定性特征，不调用 LLM：

- heading_count、heading_depth、heading_coverage；
- paragraph_count、avg_paragraph_tokens、paragraph_length_variance；
- table_ratio、picture_ratio、code_ratio；
- OCR 文本比例和空白/乱码比例；
- page_count、document_tokens；
- source_format；
- Parser warnings 和可选置信度；
- 是否存在可靠 HeadingPath。

### 8.2 Router 最小规则

```text
manual_override exists
    → use override
else reliable heading tree
    → structured
else reliable paragraph boundaries
    → paragraph
else
    → recursive_fallback
```

Router 输出必须包含：

```go
type ChunkDecision struct {
    Strategy string
    Version  string
    Features map[string]any
    Reasons  []string
    ManualOverride bool
}
```

策略、版本和关键决策进入 `chunk_config_hash`。首版 Router 只做纯函数规则，未经过离线评估前不引入模型分类。

## 9. Pipeline 与 Artifact 版本化

### 9.1 建议流水线

```text
load_source
→ resolve_parsed_artifact
→ parse_if_missing
→ validate_parsed_document
→ ocr_assets
→ persist_parsed_artifact
→ document_normalize
→ asset_enrich
→ canonical_render
→ validate_canonical_document
→ document_profile
→ chunk_strategy_route
→ chunk
→ chunk_clean
→ token_count
→ persist_chunks
→ embed_and_index
→ publish_active_index_version
```

### 9.2 分层哈希

| Hash | 至少包含 | 变化后重建范围 |
|---|---|---|
| parse_config_hash | ParseOptions、Parser/Adapter 版本 | ParsedDocument 及下游全部 |
| canonical_config_hash | Parsed Artifact hash、Normalizer、Enricher、Renderer 版本与配置 | CanonicalDocument、Chunk、Index |
| chunk_config_hash | canonical hash、策略/Router 版本、Token 参数、Tokenizer、表格/图片策略 | Chunk、Index |
| index_config_hash | chunk hash、Embedding 模型、索引参数 | Index |

Canonical 内容不得覆盖 Parser 原始的 `Document.Markdown`。初期可运行时生成；需要缓存时保存为独立 Artifact，例如：

```text
derived/{user}/{document}/content-{version}/
  parse-{parse_hash}/
    parsed-document.json.zst
    manifest.json
  canonical-{canonical_hash}/
    canonical-document.json.zst
    canonical.md
    source-map.json.zst
    manifest.json
```

## 10. 持久化与 API 演进

### 10.1 第一阶段：兼容扩展

- 在现有 `document_chunks.source_location` JSONB 中增加 `source_spans`；
- 保留现有 `page/bbox/block_ids/table_refs/asset_refs/content_types`；
- 增加 `strategy`、`strategy_version` 和可选 `decision` metadata；
- 暂不删除旧字段，保证现有引用 API 继续工作；
- Canonical Artifact 使用对象存储，不急于新增 Artifact 数据库表。

### 10.2 Parent-Child 阶段

新增 migration，而不是修改已发布 migration：

- `parent_chunk_id` 或独立 `document_chunk_relations`；
- `chunk_level`、`chunk_role`；
- 父子查询索引；
- 检索结果 Parent 展开与去重 API；
- 新旧索引版本兼容策略。

### 10.3 发布一致性

- 新版本 Chunk 和向量全部成功后才切换 `active_index_version`；
- 为同一文档加工增加 fencing/锁，避免两个任务构建同一版本；
- 失败版本不得污染旧 active；
- 重试前只清理当前目标版本；
- active 切换和任务成功状态应具备明确事务或补偿边界。

## 11. 分阶段实施计划

### P0：基线与正确性修复

目标：在改变抽象前，建立可信对照。

- 建立固定测试文档集和当前 Chunk 快照；
- 修复或锁定多来源、表格前缀预算、表头开关、Tokenizer 等问题；
- 增加真实 Parser 调用计数、失败不切 active、重试幂等测试；
- 增加同文档并发加工 fencing；
- 明确 byte offset、SourceRef 和 generated span 语义。

验收：现有策略在固定数据集上确定性运行；失败不会改变 active 版本。

### P1：CanonicalDocument 契约

目标：完成数据模型，不切换生产 Chunker。

- 定义 CanonicalDocument、Node、SourceMap schema；
- 实现 Heading、Paragraph、List、Code、Formula、Table、Picture、Caption Renderer；
- 实现 Canonical Validator；
- 建立 Renderer version 和 canonical hash；
- 为 SourceMap 增加 round-trip 测试。

验收：所有 Canonical 区间合法；每个非生成区间可追溯；相同输入生成稳定 hash。

### P2：影子运行

目标：验证 Canonical 层，不影响生产结果。

- 在现有 Chunker 旁生成 CanonicalDocument；
- 记录 Renderer warning 和来源覆盖率；
- 比较 Canonical 内容与 Parsed Blocks；
- 对表格、图片、多页文档做人工抽查；
- 可选持久化 Canonical Artifact。

验收：来源覆盖率和结构完整性达到阈值；生产 Chunk 完全不受影响。

### P3：迁移现有 StructureAwareChunker

目标：新 Chunker 消费 CanonicalDocument，但尽量保持旧行为。

- 将通用文本聚合迁移到 Canonical Nodes；
- 将表格、图片、代码、公式拆成专用 splitter；
- 输出多个 SourceSpans；
- 新旧 Chunker 双跑并生成边界差异报告；
- 使用 feature flag 按知识库或文档灰度切换。

验收：关键文档类型无来源信息退化；检索指标不低于基线；可一键回滚旧策略。

### P4：DocumentProfile 与三策略 Router

目标：在统一输入上实现 Structured、Paragraph、Recursive。

- Router 先采用确定性规则；
- 支持手动策略覆盖；
- 保存策略决策与原因；
- 在金标集上分别评估各策略；
- 只有在指标证明有收益后才启用 auto。

验收：auto 相对固定 Structured 策略没有超过阈值的召回下降，且弱结构文档有稳定收益。

### P5：Parent-Child 与后续实验

目标：扩展长文档上下文召回。

- 完成父子 Schema 和检索展开；
- 只对长且结构强的文档启用；
- 评估 Child 精准召回和 Parent 上下文增益；
- 再考虑 Semantic Chunking 或模型 Router 实验。

验收：答案支持率和长文档召回提升，延迟与上下文 Token 增幅在预算内。

## 12. 测试与评估体系

### 12.1 单元与契约测试

- Canonical Node 顺序、offset 和 UTF-8 边界；
- SourceMap 一对多、多对一及 generated span；
- 表格 Caption、表头、合并单元格与行范围；
- 图片 OCR/Caption/Description 与 AssetRef；
- overlap 来源映射；
- 纯图片 0 Chunk；
- Canonical hash 和 Chunk hash 稳定性；
- 旧 Parsed Artifact 命中后 Canonical 正确重建。

### 12.2 Pipeline 与故障测试

- 只改 Chunk 参数时不重新调用 Parser；
- Renderer 版本变化只重建 Canonical 及下游；
- Embedding 失败不切换 active；
- Chunk 持久化失败不切换 active；
- 重试不污染旧版本；
- 并发处理同一文档不会互相删除目标版本；
- migration up/down 和旧数据兼容。

### 12.3 离线检索评估

建立小型金标集：

```text
Question
Relevant Document
Relevant SourceSpan / Page / BBox
Expected Answerability
```

每个策略版本记录：

- Recall@5 / Recall@10 / Recall@20；
- MRR、nDCG；
- 引用 SourceSpan 命中率；
- 答案支持率；
- 平均/中位/P95 Chunk Token；
- Chunk 数量与索引体积；
- 检索延迟与 Parent 展开开销。

采用 paired comparison 比较旧 active 与候选版本，并设置最大允许回退阈值。

## 13. 灰度、回滚与可观测性

### 13.1 Feature Flags

- `chunk_strategy=structured|paragraph|recursive_fallback|auto`；
- `enable_canonical_chunk_diff`：仅影子运行 Legacy Chunker 并生成差异报告；
- `parent_child_enabled`。

支持按环境、知识库和文档覆盖。

### 13.2 可观测字段

- parser/adapter/renderer/chunker/router/tokenizer 版本；
- parse/canonical/chunk/index hash；
- Router features、decision 和 reasons；
- Node 数、来源覆盖率、generated span 比例；
- Chunk 数和 Token 分布；
- 无来源 Chunk、跨页 Chunk、超限失败数量；
- 新旧 Chunk 边界差异率。

### 13.3 回滚

- 旧 Chunker 在 P3 完成前始终保留；
- 新策略写入新 index_version，不覆盖 active；
- 验证失败时不发布候选版本；
- 发布后发现问题可立即切回上一 active 版本；
- Canonical Artifact 与 Parsed Artifact 分离，回滚 Chunker 不需要重新解析源文档。

## 14. 风险与应对

| 风险 | 影响 | 应对 |
|---|---|---|
| Canonical 层过度设计 | 重构时间长、接口复杂 | 首版只覆盖现有稳定 Block 类型，不建立通用文档编辑模型 |
| Markdown 与 Node 不一致 | Chunk 边界和来源漂移 | 单一 Renderer 同时生成二者；Validator 校验 offset 与内容 |
| SourceMap 体积较大 | Artifact 和 JSONB 增长 | 合并连续同源 span；Chunk 只保存相交片段；必要时压缩 Artifact |
| Router 误判 | 召回下降 | 使用确定性规则、支持文档/知识库手动覆盖，并持续对照离线指标 |
| 双跑成本增加 | 导入延迟和资源占用 | 只在灰度环境或抽样文档双跑，设置采样率 |
| 表格/图片语义退化 | 专业文档检索质量下降 | Node 保留 Table/Asset 引用，禁止只从 Markdown 恢复 |
| 版本链不清晰 | 错误复用旧产物 | 分离 parse/canonical/chunk/index hash 并记录构建依赖 |
| Parent-Child 结果重复 | 排序和上下文浪费 | Child 命中后统一 Parent 展开、去重和预算控制 |

## 15. 关键决策记录

### ADR-1：不采用 Markdown-only

原因：Markdown 无法无损表达表格合并关系、资产状态和完整来源信息。ParsedDocument 继续作为结构化事实源。

### ADR-2：Canonical Markdown 是视图，不是唯一 IR

原因：普通文本策略适合消费 Markdown，但表格和图片策略需要稳定类型和对象引用。CanonicalDocument 同时提供 Nodes、Markdown 和 SourceMap。

### ADR-3：SourceMap 使用 UTF-8 byte offset

原因：与 Go 字符串切片、哈希和存储语义一致。所有区间采用 `[start,end)`，API 必须明确单位。

### ADR-4：先兼容迁移，再增加多策略

原因：输入表示和分块策略同时变化会导致回归无法归因。P3 先迁移现有策略，P4 再增加 Router。

### ADR-5：Parent-Child 延后

原因：其价值依赖来源映射、父子持久化、检索展开和评估体系，不能只作为 Chunk 字段局部实现。

## 16. 最终建议

本次重构的重点不是“把 ParsedDocument 转成 Markdown”，而是建立一个稳定的分块契约：

```text
结构化解析事实
    ↓
可读、可版本化、带精确来源的 CanonicalDocument
    ↓
可组合、可评估、可回滚的分块策略
```

推荐立即启动 P0 和 P1：先建立基线、修复来源模型，随后实现 CanonicalDocument 契约和 Renderer。只有在影子运行证明结构与来源完整后，才将生产 Chunker 切换到新输入。

这条路线既吸收 Qavor、DocMind 的统一输入优势，又不会牺牲 Memora 已经具备的 Table、Asset、Page、BBox 和引用定位能力。

## 附录 A：与原方案的主要差异

| 原方案 | V2.0 调整 |
|---|---|
| Canonical Markdown 作为统一分块输入 | CanonicalDocument 作为统一输入，Markdown 是标准视图 |
| SourceMap 主要记录 Markdown offset 到原 Block | SourceMap 支持多来源、generated、overlap 和一对多映射 |
| P0 直接迁移 Chunker 输入 | 先建立基线和影子运行，再做兼容迁移 |
| 三策略 Router 较早实施 | 先迁移现有策略，评估后再启用 Router |
| Chunk 只有 SourceSpans 概念草案 | 明确 byte offset、SourceRef、生成内容和兼容字段 |
| Canonical 缓存版本未展开 | 增加 canonical hash 和独立 Artifact 层 |
| ParentID 作为可选字段 | Parent-Child 作为完整的存储与检索能力后置实施 |

## 附录 B：一句话表达

> Memora 保留 ParsedDocument 作为结构化事实源，在其上生成带语义节点、Markdown 视图和 SourceMap 的 CanonicalDocument；分块器只依赖这个稳定契约，从而既统一多格式分块，又保留表格、图片和精确引用定位能力。
