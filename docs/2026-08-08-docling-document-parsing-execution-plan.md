# Memora 文档解析、解析产物与结构化分块改造计划

> 执行对象：DeepSeek / 后端开发人员  
> 核心边界：Python Docling 只负责文档解析；所有 RAG 分块策略留在 Go Worker 侧  
> 支持格式：TXT、Markdown 保持 Go 解析；PDF、DOCX 使用仓库内 Python `document-parser`

## 1. 最终目标

```text
原始文件（MinIO）
  ↓
Go Document Worker
  ↓
ParserRouter
  ├── TXT / Markdown → Go TextParser
  └── PDF / DOCX     → Python document-parser → Docling
  ↓
ParsedDocument（统一、稳定、无 Chunk）
  ↓
Go 协议 / 哈希 / 大小 / 引用校验
  ↓
Parsed Artifact 保存到 MinIO
  ↓
DocumentNormalizer
  ↓
可选 AssetEnricher（Go 编排 OCR / Vision 服务）
  ↓
Go StructureAwareChunker
  ├── 标题和 Block 分组
  ├── Embedding Tokenizer 计数
  ├── 超长 Block 拆分
  ├── 过短相邻块合并
  ├── 表格专用分块
  └── 图片与正文关联
  ↓
ParsedChunk
  ↓
ChunkCleaner
  ↓
TokenCounter（只记录 token_count，不再切块）
  ↓
Chunk 持久化
  ↓
Eino Embedding → Eino Indexer
  ↓
PostgreSQL + pgvector
```

完成后必须满足：

1. 调整 Chunk size、overlap、表格拆分策略或 Embedding 模型时，可以复用已有 Parsed Artifact，不重新运行 Docling；
2. Python 服务不输出 `ParsedChunk`，不依赖 RAG Chunk 参数和 Embedding tokenizer；
3. Go 对 TXT、Markdown、PDF、DOCX 使用统一 `ParsedDocument → Chunk` 边界；
4. PDF、DOCX 解析失败不静默回退到 Eino；
5. 只有解析产物、Chunk、Embedding 和索引全部成功后才切换 `active_index_version`。

## 2. 职责边界

### 2.1 Python document-parser 负责

- PDF、DOCX 转换为内存中的 `DoclingDocument`；
- OCR、版面、阅读顺序、标题层级识别；
- 提取标题、段落、列表、代码、公式、表格、图片和 caption；
- 返回页码、bbox、Docling self-ref 等来源信息；
- 表格识别：位置、行列、单元格、表头、合并关系；
- 图片识别：位置、原图/裁剪图、页码、caption；
- 通过 `DoclingAdapter` 转成 Memora `ParsedDocument` 协议；
- 返回解析警告和固定版本信息。

Python 明确不负责：

- Chunk size、overlap、过短合并和超长拆分；
- HybridChunker、HierarchicalChunker 等 RAG 分块；
- Embedding tokenizer 和 token 上限；
- 表格按行拆分、重复表头；
- 决定图片与哪段正文组成 Chunk；
- Chunk、向量或业务数据库持久化；
- 主动访问 MinIO、PostgreSQL、Redis；
- OCR/Vision 等资产二次增强策略。

### 2.2 Go Worker 负责

- 从 MinIO 读取原始文件；
- 调用 Python 服务并校验响应；
- 保存、查找和复用 Parsed Artifact；
- 执行 `DocumentNormalizer`；
- 编排可选的图片 OCR/Vision `AssetEnricher`；
- 执行 `StructureAwareChunker`；
- 表格和图片的 Chunk 策略；
- 调用与 Embedding 模型对齐的 tokenizer；
- 执行 `ChunkCleaner` 和最终 token_count 记录；
- Chunk 持久化、Embedding、Indexer 和版本切换。

### 2.3 不做的内容

- 不建立通用文档解析平台或插件市场；
- Python 服务不建立任务队列和业务数据库；
- 不把 Docling 原始 DTO 直接暴露给 Go RAG Pipeline；
- 不把完整 Docling JSON 当作 Go 的长期稳定协议；
- 不允许远程 Vision/OCR 默认开启；
- 第一阶段不做图片向量和多模态索引；
- 不修改公开上传 API 和前端；
- 不提交模型权重、真实业务文档和大型 fixture。

## 3. 精确运行流程

### 3.1 首次处理

```text
1. 用户上传文件
2. API 将原始文件保存到 MinIO
3. 创建 Document Process Task
4. Worker 计算 content_version、parse_config_hash
5. Worker 查找兼容 Parsed Artifact
6. Artifact 不存在：调用对应 Parser
7. 获得 ParsedDocument
8. Go 校验 schema、source hash、引用和资源限制
9. Go 保存解析产物与提取图片到 MinIO
10. Artifact manifest 完整写入后才视为解析成功
11. DocumentNormalizer 规范化 Blocks
12. 可选 AssetEnricher 补充图片 OCR/Vision 信息
13. StructureAwareChunker 生成 ParsedChunk
14. ChunkCleaner 做最终文本清理
15. TokenCounter 写入 token_count
16. 持久化 Chunk
17. Eino Embedding
18. Eino Indexer
19. 全部成功后切换 active_index_version
```

### 3.2 重新分块或更换 Embedding 模型

```text
已有 Parsed Artifact
  ↓
校验 source_hash + schema_version + parser_version + parse_config_hash
  ↓
直接加载 ParsedDocument
  ↓
使用新的 chunk_config / tokenizer
  ↓
重新 Chunk、Embedding、Index
```

只要原文件内容和解析配置未变化，就不能重新调用 Docling。

### 3.3 何时必须重新解析

满足任意条件时重新调用 Parser：

- 原始文件 `source_hash` 改变；
- `ParsedDocument` schema 主版本不兼容；
- Docling/Adapter 升级且明确改变解析语义；
- OCR 语言、表格识别或图片提取等 parse config 改变；
- Artifact 缺失、损坏或哈希校验失败。

下列变化不应触发重新解析：

- Chunk size、overlap、merge threshold 改变；
- 表格大表拆分行数改变；
- 图片与正文关联策略改变；
- Embedding 模型或 tokenizer 改变；
- ChunkCleaner 规则改变；
- Index 配置改变。

## 4. 仓库目录

Python 服务采用最小结构。不要为了“分层完整”预先拆出大量目录；只有单个文件明显过大、存在两个以上独立实现，或测试隔离确有需要时再拆分。

```text
services/document-parser/
├── app.py                 # FastAPI、配置、health、parse endpoint
├── schemas.py             # 请求、ParsedDocument、错误协议
├── docling_adapter.py     # Docling 初始化、转换和结构适配
├── tests/
├── pyproject.toml
├── uv.lock 或 requirements.lock
├── Dockerfile
└── README.md
```

第一版生产代码优先控制在 3 个 Python 文件。允许后续按实际复杂度拆出 `rapid_ocr.py`、`table_extractor.py` 或 `picture_extractor.py`，但不得一开始为每个概念建立空目录和转发层。

这里的“文件少”仅代表组织简洁，不代表减少以下能力：

- ParsedDocument schema version；
- Block/Table/Asset 结构和引用校验；
- 文件大小、页数、响应和图片总量限制；
- 模型常驻和并发上限；
- 超时、取消、安全错误和测试；
- 固定依赖版本与可重复构建。

Go 代码：

```text
internal/service/rag/parser/
├── parser.go
├── router.go
├── text_parser.go
├── python_parser.go
├── contract.go
├── artifact_store.go
├── validator.go
└── *_test.go

internal/service/rag/normalizer/
├── document_normalizer.go
└── document_normalizer_test.go

internal/service/rag/chunking/
├── chunker.go
├── structure_aware_chunker.go
├── markdown_strategy.go
├── block_strategy.go
├── table_strategy.go
├── picture_strategy.go
├── token_splitter.go
└── *_test.go

internal/service/rag/transformer/
├── chunk_cleaner.go
└── chunk_enricher.go

internal/service/rag/asset/
├── enricher.go
├── noop_enricher.go
└── *_test.go
```

Python 文件不得放入 Go `pkg/`。

## 5. Python HTTP API

只提供内部同步接口：

```text
GET  /health/live
GET  /health/ready
POST /v1/parse
```

默认使用常驻 FastAPI 进程，而不是由 Go 为每个文档执行一次 `python parse_document.py`。原因是 Docling/OCR 模型需要跨请求复用；命令行模式可以保留为本地诊断入口，但不能作为 Worker 默认生产调用方式。即使采用 FastAPI，三个生产 Python 文件也足够承载第一版，不需要复杂目录结构。

`POST /v1/parse` 使用 `multipart/form-data`：

- `file`：PDF 或 DOCX；
- `options`：解析选项 JSON；
- 不包含 chunk size、overlap、tokenizer 或 Embedding model；
- 拒绝未知格式、空文件、超限文件和伪造格式；
- MIME 仅辅助判断，必须验证扩展名和文件签名/容器格式。

请求选项示例：

```json
{
  "schema_version": "1.0",
  "ocr_languages": ["zh", "en"],
  "do_ocr": true,
  "table_structure": true,
  "extract_pictures": true,
  "include_bboxes": true
}
```

禁止请求携带任意模型路径、任意远程 URL、shell 参数或插件名称。

## 6. ParsedDocument 稳定协议

Python 只返回 ParsedDocument，不返回 ParsedChunk：

```json
{
  "schema_version": "1.0",
  "parser": {
    "name": "docling",
    "version": "固定版本",
    "adapter_version": "1.0"
  },
  "source": {
    "file_name": "example.pdf",
    "format": "pdf",
    "sha256": "...",
    "size": 123456
  },
  "document": {
    "title": "示例文档",
    "markdown": "# 示例文档\n...",
    "page_count": 10,
    "metadata": {}
  },
  "blocks": [],
  "tables": [],
  "assets": [],
  "warnings": []
}
```

同一 schema 主版本只能新增可选字段，删除字段或改变字段语义必须升级主版本。Go 只接受明确支持的版本。

### 6.1 Block

```json
{
  "id": "block-000012",
  "type": "paragraph",
  "text": "正文",
  "markdown": "正文",
  "heading_path": ["第一章", "1.1 背景"],
  "source": {
    "page": 3,
    "bbox": [10.0, 20.0, 200.0, 100.0],
    "docling_ref": "#/texts/8"
  },
  "table_ref": null,
  "asset_refs": []
}
```

第一版 Block 类型：

```text
title, heading, paragraph, list_item, code, formula,
table, picture, caption, footnote, page_header, page_footer, unknown
```

Block 顺序必须是文档阅读顺序。未知 label 映射为 `unknown` 并写 warning，不能丢弃正文。

### 6.2 Table

```json
{
  "id": "table-000001",
  "caption": "表 1 销售数据",
  "page_start": 3,
  "page_end": 4,
  "bbox": [10.0, 20.0, 300.0, 220.0],
  "headers": [["地区", "销售额"]],
  "rows": [["华东", "100"]],
  "cells": [],
  "row_count": 1,
  "column_count": 2,
  "markdown": "| 地区 | 销售额 |..."
}
```

DoclingAdapter 只负责准确表达表格结构，不决定表格如何分 Chunk。

### 6.3 Asset

```json
{
  "id": "asset-000003",
  "kind": "picture",
  "mime_type": "image/png",
  "sha256": "...",
  "width": 1200,
  "height": 800,
  "page": 4,
  "bbox": [10.0, 20.0, 300.0, 220.0],
  "caption": "图 2 系统架构",
  "data_base64": "...",
  "omitted": false,
  "metadata": {}
}
```

Python 不生成 Vision description，也不决定图片关联的 Chunk。图片二进制必须有单图、数量和总大小限制，日志禁止记录 base64。

## 7. DoclingAdapter

Python 内部正确流程：

```text
PDF / DOCX
  ↓
Docling SDK
  ↓
DoclingDocument（内存对象）
  ↓
DoclingAdapter
  ↓
Memora ParsedDocument
```

禁止执行无意义的 `DoclingDocument → JSON → 再反序列化`。Docling 原始 JSON 可以用于临时诊断，但不是 Go 的稳定协议，也不是默认持久化格式。

Adapter 必须：

- 保留阅读顺序、标题路径、caption、page、bbox 和 self-ref；
- 保留表格行列、单元格和合并关系；
- 保留图片原始数据或安全的 omitted 标记；
- 给所有 Block、Table、Asset 生成结果内稳定 ID；
- 验证所有引用存在；
- 不包含任何 Chunk 和 RAG 策略字段。

## 8. Parsed Artifact 保存与复用

### 8.1 Artifact 组成

解析产物不是一个包含无限 base64 的大 JSON，而是一个版本化 Artifact 集合：

```text
derived/{user_id}/{document_id}/content-{content_version}/
  parse-{parse_config_hash}/
    manifest.json
    parsed-document.json.zst
    assets/
      asset-000001.png
      asset-000002.jpg
```

保存顺序：

1. 校验 Python 响应；
2. 解码并保存 assets；
3. 将 ParsedDocument 中 `data_base64` 替换为 asset object key；
4. 保存压缩后的 `parsed-document.json.zst`；
5. 最后保存 `manifest.json`；
6. 只有 manifest 存在且全部哈希一致，Artifact 才算完整。

首次任务保存成功后可继续使用同一份已校验内存对象；重新分块任务从 Artifact 读取。两条路径必须通过同一反序列化/校验代码。

### 8.2 Manifest

至少记录：

```json
{
  "artifact_schema_version": "1.0",
  "parsed_document_schema_version": "1.0",
  "source_sha256": "...",
  "parser_name": "docling",
  "parser_version": "...",
  "adapter_version": "1.0",
  "parse_config_hash": "...",
  "parsed_document_sha256": "...",
  "assets": [],
  "created_at": "..."
}
```

Artifact key 必须可由 document/content version 和 parse config 确定性计算。MVP 可以不新增数据库表；不得依赖扫描整个 bucket 来发现 Artifact。

### 8.3 版本含义

- `content_version`：原始文档内容版本；
- `parser_version`：Docling 解析引擎版本；
- `adapter_version`：Docling → ParsedDocument 转换语义版本；
- `parse_config_hash`：OCR、表格识别、图片提取等解析配置；
- `chunk_version/chunk_config_hash`：Go RAG 分块策略；
- `index_version`：Chunk + Embedding + Index 的发布版本。

这些版本不得混为一个字段。

## 9. DocumentNormalizer

分块前执行，输入输出都是 ParsedDocument：

- Unicode 和换行规范化；
- 处理不可见字符和异常空白；
- 识别/移除重复页眉页脚；
- 规范 Block 类型、空 Block 和顺序；
- 规范 heading path；
- 修复安全可确定的 caption/Block 引用；
- 表格单元格文本规范化但不拆表；
- 保留 page、bbox、table_ref、asset_ref。

禁止：

- 将完整文档扁平化成纯 Markdown；
- 在这里按 token 分块；
- 删除表格行列结构；
- 将图片 base64 放回文档；
- 使用 RAG chunk size/overlap。

原来的通用 `Cleaner` 必须拆分或重命名，禁止分块前后都叫 Cleaner。

## 10. AssetEnricher

图片二次增强由 Go 编排，但实际 OCR/Vision 推理可以调用独立 Python/模型服务：

```text
ParsedDocument.Assets
  ↓
Go AssetEnricher
  ├── NoopEnricher（默认）
  ├── ImageOCREnricher（以后）
  └── VisionDescriptionEnricher（以后）
  ↓
EnrichedAsset
```

接口示意：

```go
type AssetEnricher interface {
    Enrich(ctx context.Context, doc *ParsedDocument) error
}
```

原则：

- Go 决定是否增强、调用哪个 provider、超时和失败策略；
- 推理实现不要求写成纯 Go；
- 默认 `NoopEnricher`，仅使用 Docling 提取的 caption、page 和位置；
- OCR/Vision 失败默认降级为原始 caption，但必须记录结构化 warning；
- 是否允许远程服务必须由管理员显式配置；
- Vision/OCR 配置不进入 `parse_config_hash`，进入单独 `asset_enrich_config_hash`；
- 改变图片增强策略不应重新运行 Docling。

若增强结果需要跨多次重新分块复用，可在 Artifact 旁保存独立、版本化的 `asset-enrichment.json`，不得覆盖原始 Parsed Artifact。

## 11. Go StructureAwareChunker

### 11.1 接口

```go
type ChunkOptions struct {
    MaxTokens       int
    MinTokens       int
    OverlapTokens   int
    RepeatTableHead bool
    StrategyVersion string
}

type StructureAwareChunker interface {
    Chunk(ctx context.Context, doc *ParsedDocument, opts ChunkOptions) ([]ParsedChunk, error)
}
```

`ParsedChunk`：

```go
type ParsedChunk struct {
    Content        string
    HeadingPath    []string
    SourceLocation SourceLocation
    ContentTypes   []string
    BlockIDs       []string
    TableRefs      []string
    AssetRefs      []string
    TokenCount     int
}
```

### 11.2 Tokenizer

Chunker 必须注入与 Embedding 模型一致的 tokenizer：

```go
type Tokenizer interface {
    Count(text string) (int, error)
    Split(text string, maxTokens, overlapTokens int) ([]string, error)
}
```

要求：

- Embedding 模型改变时重新选择 tokenizer 并重新 Chunk；
- 不允许默默使用字符数代替 token 数；
- tokenizer 不可用时任务失败，不生成可能超限的 Chunk；
- Chunk 完成后的 TokenCounter 使用同一个 tokenizer，仅写 `token_count`；
- 后置 TokenCounter 禁止再次拆分或合并 Chunk。

### 11.3 普通 Block 策略

```text
按 heading_path 分组
  ↓
保持 Block 阅读顺序
  ↓
计算 contextualized text token 数
  ↓
超过 MaxTokens：在当前 Block 内 token-aware 拆分
  ↓
低于 MinTokens：只与相邻且结构兼容的 Chunk 合并
```

规则：

- 不跨不相关一级标题合并；
- overlap 只发生在被拆分的长文本内部，不能复制整张表或图片；
- caption 与其表格/图片尽量保持同一 Chunk；
- code/formula 优先保持完整，超限才拆；
- 每个 Chunk 保留 Block IDs、页码和 bbox。

### 11.4 TXT / Markdown 策略

- TextParser 输出统一 ParsedDocument；
- Markdown 标题转换为 heading Block，或复用现有 Markdown Header Splitter 作为 Go 策略实现；
- 现有 Recursive Splitter 只能作为超长文本的底层实现，必须受 tokenizer 上限控制；
- 保持现有 TXT/Markdown 行为的回归测试；
- TXT/Markdown 也可保存 Parsed Artifact，保证统一重新分块入口。

## 12. 表格职责拆分

### 12.1 Docling / Python

负责回答：

- 表格在哪里；
- 属于哪些页；
- bbox 是什么；
- caption 是什么；
- 表头、行、列、单元格和合并关系是什么；
- 原始阅读顺序是什么。

不决定 Chunk。

### 12.2 Go TableStrategy

负责：

- 生成表格检索文本；
- 小表整体成为一个 Chunk；
- 大表按行分割；
- 每个子 Chunk 重复 caption 和表头；
- 使用 tokenizer 确保每个 Chunk 不超限；
- 单行自身超限时在单元格文本内部安全拆分；
- 保留 table_ref、行范围、页码和 bbox；
- 过短表格可以和紧邻的解释段落合并，但必须结构兼容。

表格确定性检索文本：

```text
标题路径
表格 Caption
表头
当前行范围
表格内容
```

## 13. 图片职责拆分

### 13.1 Docling / Python

负责：

- 找到图片；
- 提取图片或页面裁剪图；
- 返回 MIME、尺寸、哈希；
- 返回 page、bbox、caption；
- 建立 picture Block 和 asset_ref。

### 13.2 Go PictureStrategy

负责：

- 使用 caption 和可选 OCR/Vision description 生成检索文本；
- 根据 Block 顺序、页码、bbox、caption 关系选择相邻正文；
- 决定图片独立成 Chunk 还是合并到正文 Chunk；
- 保留 asset object key、asset_ref、page 和 bbox；
- 图片二进制不进入 Chunk、Embedding、日志和数据库正文。

第一阶段默认规则：

1. 有 caption/description 时生成图片文本；
2. 与同一标题路径下最近的前后正文关联；
3. 关联后超出 MaxTokens 则独立成 Chunk；
4. 无任何文字信息的图片只保存资产和 source reference，不生成空 Chunk。

## 14. ChunkCleaner

分块后、Embedding 前执行：

- trim；
- 规范最终换行和多余空白；
- 移除分块序列化产生的重复分隔符；
- 检查 Chunk 内容非空；
- 保留标题、表头、caption 等结构上下文；
- 不改变 Chunk 边界；
- 不做 token-aware split；
- 不删除 source location 和引用。

后续 TokenCounter 只重新确认并记录 token_count。如果发现超过 MaxTokens，应返回 Chunker bug 错误，不能在这里补救性切割。

## 15. Go Pipeline 接入

建议节点：

```text
resolve_artifact
  ↓
parse_if_missing
  ↓
validate_parsed_document
  ↓
persist_artifact
  ↓
document_normalize
  ↓
asset_enrich
  ↓
structure_chunk
  ↓
chunk_clean
  ↓
token_count
  ↓
persist_chunks
  ↓
embed_and_index
```

扩展 `ProcessInput`：

```go
type ProcessInput struct {
    ObjectKey string
    FileName  string
    MIMEType  string
    DocMeta   transformer.DocMeta
}
```

保持不变：

- `ChunkWriter.BatchInsert`；
- `PostgresIndexer`；
- `active_index_version` 成功后切换；
- 无 Embedding 模型时的现有关键词索引行为；
- ImportTask/Worker 重试职责。

Metadata key 集中放在 `internal/service/rag/einoadapter/metadata.go`，新增 parsed artifact、block/table/asset 引用时禁止散落字符串。

## 16. 配置

Go 示例：

```yaml
document_parser:
  base_url: "http://localhost:5001"
  timeout: 8m
  max_response_size: 134217728
  max_asset_bytes: 33554432
  ocr_languages: ["zh", "en"]

chunking:
  strategy_version: "structure-v1"
  max_tokens: 1000
  min_tokens: 100
  overlap_tokens: 100
  repeat_table_header: true

asset_enrichment:
  mode: "none"
  timeout: 2m
```

关键点：

- `document_parser` 不包含 Chunk 参数；
- `chunking` 配置进入 `chunk_config_hash`，不进入 `parse_config_hash`；
- tokenizer/Embedding model ID 参与 chunk config hash；
- asset enrichment 使用独立 config hash；
- Parser timeout 小于 Worker 总 timeout；
- Python 和 Go 大小限制一致或 Python 更严格；
- 密钥只来自环境变量/secret。

## 17. Docker 与 Windows

Python 服务保持常驻，但启动方式可选：

```text
Windows 本地调试：venv + uvicorn，Go 使用 http://localhost:5001
Compose 联调/部署：document-parser 容器，Worker 使用 http://document-parser:5001
```

Compose 要求：

- 构建 `services/document-parser/Dockerfile`；
- 固定 Python、Docling 和基础镜像版本；
- 默认 CPU，可提供独立 GPU profile；
- 模型缓存使用独立 volume；
- healthcheck 调用 `/health/ready`；
- 只有 Worker 依赖 parser，API 不依赖；
- 不挂载 Docker socket；
- 默认禁止远程模型服务。

## 18. 测试计划

### 18.1 Python

- PDF/DOCX 转换；
- OCR、阅读顺序、heading path、page、bbox；
- Table 行列、cells、caption 和跨页信息；
- Picture 提取、MIME、尺寸、hash、caption；
- 空文件、伪造格式、损坏文件、超限文件；
- 所有 Block/Table/Asset 引用完整；
- 响应中不存在 Chunk/RAG 字段；
- 临时目录清理、并发限制、请求取消；
- schema JSON contract 测试。

### 18.2 ArtifactStore

- 确定性 key；
- source/parser/adapter/parse config hash 校验；
- 先 assets、再 document、最后 manifest；
- 缺失/损坏 asset 拒绝复用；
- manifest 不完整拒绝复用；
- Chunk 配置变化仍命中同一个 Artifact；
- Parser 配置变化不命中旧 Artifact；
- 失败清理只影响当前未激活前缀。

### 18.3 DocumentNormalizer

- Unicode、空白、页眉页脚；
- Block 顺序和 heading path；
- 不丢失 table/asset/source 引用；
- 不生成 Chunk；
- 不破坏表格行列。

### 18.4 StructureAwareChunker

- 与 Embedding tokenizer 对齐；
- 超长 Block token-aware split；
- overlap 不跨结构边界；
- 过短相邻块合并；
- 不跨无关标题合并；
- code/formula 保持完整；
- ChunkCleaner 不改变 Chunk 数量；
- TokenCounter 不再拆分；
- 最终 Chunk 全部不超过 MaxTokens。

### 18.5 TableStrategy

- 小表整体 Chunk；
- 大表按行切分；
- 每个子 Chunk 重复 caption 和表头；
- 单行超限安全拆分；
- row range、table_ref、page、bbox 正确。

### 18.6 PictureStrategy / AssetEnricher

- 图片资产安全保存；
- caption 与附近正文关联；
- description 存在时进入 Chunk；
- 空图片文本不生成空 Chunk；
- OCR/Vision 失败按配置降级；
- 图片二进制不进入 Chunk、日志、错误和 Embedding。

### 18.7 Pipeline / Version

- TXT/Markdown 只调用 Go Parser；
- PDF/DOCX 只调用 Python Parser；
- 已有兼容 Artifact 时不调用 Parser；
- Parser/Artifact/Chunk/Embedding/Index 任一步失败不切换 active version；
- 重新分块只更新 chunk/index version；
- 原 active 版本资产和 Chunk 不被失败任务覆盖。

## 19. 集成 Fixture

至少包含：

1. 中文 TXT；
2. 含标题、代码、表格、图片引用的 Markdown；
3. 数字版中文 PDF；
4. 扫描型中英文 PDF；
5. 含多级标题、表格和图片的 DOCX；
6. 跨页大表格 PDF；
7. 含流程图、统计图、照片的 PDF/DOCX；
8. 空文件、损坏 PDF、伪造 PDF。

必须验证：第一次运行产生 Artifact；只修改 chunk size 后重新执行不产生新的 Docling 请求。

## 20. 实施顺序

### 任务包 1：协议与版本

- 定义 ParsedDocument、Block、Table、Asset、Manifest；
- 定义 Python Pydantic DTO 和 Go mirror DTO；
- 定义版本、hash 和兼容规则；
- Contract 测试。

### 任务包 2：Python 纯解析服务

- 使用 `app.py` 实现 FastAPI、health、配置、限制和错误模型；
- 使用 `schemas.py` 实现稳定协议；
- 使用 `docling_adapter.py` 完成 Docling 模型常驻、转换和结构适配；
- PDF/DOCX Blocks、Tables、Assets；
- 明确无 Chunk 代码和 Chunk 配置。

验收：第一版生产 Python 代码优先保持 3 个文件；如果必须拆分，提交说明具体原因，不得只为套用分层模板而拆分。

### 任务包 3：Go ParserRouter 与校验

- TextParser；
- PythonDocumentParser；
- 流式 multipart；
- schema、大小、hash、引用校验；
- Parser 错误分类。

### 任务包 4：Parsed Artifact

- ArtifactStore；
- MinIO 版本化 key；
- assets/document/manifest 原子完成语义；
- Artifact 查找、加载、复用和损坏检测；
- 证明修改 Chunk 配置不会重跑 Docling。

### 任务包 5：DocumentNormalizer

- 从旧 Cleaner 拆出分块前规则；
- Block、表格和引用规范化；
- 兼容测试。

### 任务包 6：Go StructureAwareChunker

- tokenizer 抽象；
- 普通 Block、Markdown、Table、Picture 策略；
- token-aware split、merge、overlap；
- ParsedChunk 和 source location；
- 全面单元测试。

### 任务包 7：ChunkCleaner 与 Pipeline

- 从旧 Cleaner 拆出分块后规则；
- 后置 TokenCounter 只计数；
- 接入 persist、Embedding、Indexer；
- 保持 active index version 语义。

### 任务包 8：AssetEnricher 扩展点

- Go 接口和 Noop 实现；
- caption 基础策略；
- 为 OCR/Vision provider 预留稳定接口；
- 不在第一阶段强制引入远程模型。

### 任务包 9：部署与端到端

- Dockerfile、Compose、模型缓存、health dependency；
- Windows venv 启动说明；
- 完整 fixture 测试；
- Re-chunk 不重跑 Docling 验收；
- 日志、临时文件、Artifact 和失败版本检查。

## 21. 验证命令

```bash
cd services/document-parser
python -m ruff check .
python -m mypy app
python -m pytest
```

```bash
gofmt -w <本次修改的 Go 文件>
go test ./internal/service/rag/...
go test ./internal/service/...
go test ./pkg/config/...
go test ./...
go vet ./...
```

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build document-parser
docker compose -f deploy/docker-compose.yml up -d document-parser
```

## 22. 完成定义

- [ ] Python 只输出 ParsedDocument，不输出 ParsedChunk；
- [ ] Python 不包含 chunk size、overlap 和 tokenizer；
- [ ] DoclingAdapter 保留 Block/Table/Asset 结构和来源；
- [ ] Parsed Artifact 在 Chunk 前完整保存；
- [ ] 修改 Chunk/Embedding 配置可复用 Artifact；
- [ ] 解析版本与 Chunk/Index 版本清晰分离；
- [ ] DocumentNormalizer 与 ChunkCleaner 职责分离；
- [ ] Go StructureAwareChunker 使用 Embedding tokenizer；
- [ ] 后置 TokenCounter 只记录、不切分；
- [ ] 小表整体、大表按行、子 Chunk 重复表头；
- [ ] 图片提取属于 Docling，图片增强和关联策略由 Go 编排；
- [ ] 图片资产保存到版本化 MinIO 路径；
- [ ] PDF/DOCX Parser 失败不 fallback；
- [ ] Artifact/Chunk/Embedding/Index 失败不切换 active version；
- [ ] TXT/Markdown 行为有回归测试；
- [ ] Python、Go、Compose 和端到端测试全部通过；
- [ ] 只改 Chunk size 的端到端测试中 Docling 调用次数为 0。

## 23. 给 DeepSeek 的执行要求

1. 严格按任务包顺序推进，每包完成后运行对应测试；
2. Python 代码出现 Chunker、Embedding tokenizer、chunk size 或 overlap 即视为职责越界；
3. 不允许先把 DoclingDocument 降级成 Markdown 后再反推结构；
4. 不允许 Go 直接依赖 Docling 原始 JSON 字段；
5. Artifact manifest 未完成时不得进入 Chunk；
6. ChunkCleaner 和后置 TokenCounter 不得改变 Chunk 边界；
7. 遇到 Docling API 不确定时查询固定版本源码和官方文档，不猜接口；
8. 不增加通用队列、插件系统、公共 API 或无必要数据库表；
9. 不提交密钥、模型缓存、真实文档或大型二进制；
10. 最终交付必须列出修改文件、固定依赖版本、协议示例、Artifact key、测试命令与结果、已知限制和未完成项。
11. Python 第一版优先采用 `app.py + schemas.py + docling_adapter.py`，禁止照搬大型服务模板制造无业务价值的目录和包装层。
