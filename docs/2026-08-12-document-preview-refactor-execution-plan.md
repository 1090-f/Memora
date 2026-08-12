# Memora 文档预览体系具体改造执行方案

> 编制日期：2026-08-12
> 依据：`Memora_文档预览体系修改方案.md`、当前 Memora 后端与 `web/admin` 实现
> 改造边界：只重构 Preview 层；不改变 DocumentPipeline 的解析、清洗、分块、Embedding 与索引语义

## 1. 执行结论

本次改造采用“统一描述器 + 独立预览产物 + 独立异步 Worker + Viewer 路由”的方案，但结合当前代码作以下落地调整：

1. `GET /api/v1/documents/:document_id/preview` 改为返回 Preview Descriptor；现有正文读取逻辑迁移到 `/preview/text`。
2. `GET /documents/:id/rendered` 不再同步调用 LibreOffice；迁移期作为 `/preview/rendered` 的兼容别名，只读取已生成产物。
3. 新增一张 `document_previews` 表，同时承担 Preview Artifact 元数据和异步任务状态，不再额外建立 preview task 表。
4. 将现有 `task_outbox` 泛化为可投递多种任务事件，Preview 使用独立 Redis Stream 和 Consumer Group，避免 Office 转换阻塞文档解析任务。
5. DOCX、PPTX 后台转 PDF；PDF 和图片直接读取 Original Artifact，不生成重复预览文件。
6. XLSX 默认生成结构化 `sheet-data.json.zst`，PDF 作为备用预览。当前 `ParsedDocument.Table` 没有 Sheet 名称，因此 MVP 不直接把它当成完整工作簿预览数据源，而由 Preview Worker 从原始 XLSX 提取 Sheet 数据。
7. 前端只依据 `preview_type`、`status`、`content_url` 和 `fallbacks` 选择 Viewer，不再识别扩展名或 MIME 类型来决定后端接口。
8. Preview 失败只更新 `document_previews`，绝不修改 `documents.processing_status`、活动索引或导入任务终态。

最终职责如下：

```text
Original Artifact
  ├─ DocumentPipeline ──> Parsed Artifact ──> Chunk / Index / RAG
  └─ Preview Scheduler ─> Preview Worker ───> Preview Artifact

GET /documents/:id/preview
  └─ PreviewService ──> Preview Descriptor ──> PDF / Markdown / Text / Image / Table Viewer
```

## 2. 当前项目现状与差距

| 项目 | 当前实现 | 目标状态 | 主要修改位置 |
| --- | --- | --- | --- |
| 统一预览入口 | `/preview` 直接返回 `{content, format}` | `/preview` 返回 Descriptor | `internal/api/v1/document`、`internal/model/dto/response/document.go` |
| 解析正文 | `documentService.Preview` 从 Parsed Artifact 读取并重写图片 URL | 原逻辑保留，改为 `GetTextPreview` | `internal/service/document_service.go` |
| PDF 预览 | `/rendered` 返回原始 PDF | Descriptor 指向 `/original?inline=true` | Preview Router |
| Office 预览 | `/rendered` 缓存未命中时在 HTTP 请求内同步转换 | Preview Worker 异步转换，GET 只读产物 | `document_service.go`、`office_converter.go`、`internal/background` |
| Office 缓存 | `rendered/{user}/{doc}/v{content_version}.pdf` | `derived/.../preview-{render_hash}/rendered.pdf` + manifest | 新 Preview Artifact Store |
| LibreOffice 并发 | `OfficeConverter` 内部不可配置 `sync.Mutex` | Worker 内配置化 semaphore | 新 Preview renderer/processor |
| XLSX | 与 DOCX/PPTX 一样转 PDF | Table Viewer 为主、PDF 为 fallback | XLSX renderer、Table API、前端 TableViewer |
| 表格协议 | 有 headers/rows/cells，但没有 Sheet 名称、Sheet 顺序 | Preview Artifact 明确保留工作簿/Sheet 信息 | XLSX Preview Artifact 协议 |
| 前端路由 | `DocumentViewer.tsx` 判断 PDF/DOCX/XLSX/PPTX/图片扩展名 | 仅判断 Descriptor 的 `preview_type` | `web/admin/src/features/document` |
| PDF Viewer | 浏览器 iframe + Blob URL | MVP 引入 PDF.js Viewer；必要时可先保留 iframe | 新 `PdfViewer.tsx` |
| 异步基础设施 | 单一文档解析 Stream；Outbox 外键绑定 `import_tasks` | 通用 Outbox 发布器 + 解析/预览两个 Stream | migration、`internal/background` |
| Preview 状态 | 无独立状态 | pending/processing/ready/failed/unsupported | `document_previews` |
| Content API | 前端在 `/preview` 失败后读取 Chunk `/content` | 仅供 Agent/长文结构化读取，不作为视觉预览 fallback | 前端、API 文档 |

### 2.1 必须保留的现有能力

- `DocumentPipeline` 节点顺序和 RAG 主链路保持不变。
- Parsed Artifact 的路径、校验、图片资产和 HMAC 签名机制保持不变。
- `/original` 继续负责下载/内联原文件。
- `/assets/:asset_id` 继续为 Markdown/Docling 图片提供签名访问。
- `/knowledge-bases/:kb_id/documents/:id/content` 继续提供 token cursor、citation 和 Agent 文档读取。
- Markdown/ZIP 延迟入队和附件确认机制不受影响。

### 2.2 不能直接照搬原方案的部分

1. 当前 `documents` 没有通用 metadata 字段，无法把可靠的预览状态简单塞入 metadata；因此需要独立表。
2. 当前 `task_outbox.aggregate_id` 外键指向 `import_tasks`，不能直接写入 Preview 任务 ID；需要迁移为通用 Outbox。
3. 当前 Parsed XLSX 表格没有 Sheet 名称，无法满足 Sheet Tab；因此 XLSX Preview Artifact 需从原工作簿提取，不能仅把 `ParsedDocument.Tables` 原样返回。
4. 当前前端没有 PDF.js，仅使用 iframe；若要实现统一、稳定的页码和加载状态，需要新增 PDF Viewer 组件和依赖。

## 3. 目标 API 契约

### 3.1 Preview Descriptor

```http
GET /api/v1/documents/:document_id/preview
Authorization: Bearer <token>
```

文档存在时始终返回 HTTP 200。预览是否完成由业务字段 `status` 表达，避免将正常的异步处理中状态当成 HTTP 错误。

```json
{
  "document_id": "doc-id",
  "content_version": 1,
  "preview_type": "pdf",
  "status": "processing",
  "content_url": null,
  "media_type": "application/pdf",
  "original_url": "/api/v1/documents/doc-id/original",
  "retry_after_ms": 2000,
  "fallbacks": [
    {
      "preview_type": "markdown",
      "status": "ready",
      "content_url": "/api/v1/documents/doc-id/preview/text",
      "media_type": "application/json"
    },
    {
      "preview_type": "download",
      "status": "ready",
      "content_url": "/api/v1/documents/doc-id/original",
      "media_type": "application/octet-stream"
    }
  ],
  "error": null
}
```

枚举固定为：

```go
type PreviewType string

const (
    PreviewTypeText     PreviewType = "text"
    PreviewTypeMarkdown PreviewType = "markdown"
    PreviewTypePDF      PreviewType = "pdf"
    PreviewTypeImage    PreviewType = "image"
    PreviewTypeTable    PreviewType = "table"
    PreviewTypeDownload PreviewType = "download"
    PreviewTypeNone     PreviewType = "none"
)

type PreviewStatus string

const (
    PreviewStatusPending     PreviewStatus = "pending"
    PreviewStatusProcessing  PreviewStatus = "processing"
    PreviewStatusReady       PreviewStatus = "ready"
    PreviewStatusFailed      PreviewStatus = "failed"
    PreviewStatusUnsupported PreviewStatus = "unsupported"
)
```

约束：

- `content_url` 只在对应资源可读取时返回；前端不得自行拼 URL。
- `fallbacks` 是有序数组，前端选择第一个 `ready` 项。
- `error` 仅在 `failed/unsupported` 时返回稳定错误码和安全消息。
- URL 是 API 相对路径，前端仍通过现有 Axios 客户端携带 Bearer Token；不能直接把受保护 URL赋给 `<iframe>` 或 `<img>`。
- `processing` 时返回 `retry_after_ms`，前端据此轮询 Descriptor，不轮询二进制资源。

### 3.2 文本预览

将当前 `/preview` 的正文能力迁移为：

```http
GET /api/v1/documents/:document_id/preview/text
```

响应保持兼容：

```json
{
  "content": "# parsed markdown",
  "format": "markdown"
}
```

实现继续复用当前能力：

- 手工文档直接读取 `documents.content`；
- 文件/URL 文档读取完整 Parsed Artifact；
- Docling 图片占位符重写为签名 Asset URL；
- Markdown 图片引用重写为签名 Asset URL；
- 不从 Chunk 反向拼接全文。

### 3.3 PDF/Office 二进制预览

```http
GET /api/v1/documents/:document_id/preview/rendered
```

- PDF 的 Descriptor 不指向该接口，而是直接指向 `/original?inline=true`。
- DOCX/PPTX/XLSX PDF fallback 指向该接口。
- 接口只读取状态为 `ready` 且校验通过的 Preview Artifact，禁止在请求内启动 LibreOffice。
- 非 ready 状态返回 `409 PREVIEW_NOT_READY` 并携带 `Retry-After`；前端正常情况下不会在 Descriptor ready 前请求此接口。
- 保留旧 `/documents/:id/rendered` 一个发布周期，内部调用相同只读逻辑，并在 API 文档标记 deprecated。

### 3.4 XLSX Table API

```http
GET /api/v1/documents/:document_id/preview/table?sheet_index=0&row_offset=0&row_limit=200
```

```json
{
  "document_id": "doc-id",
  "content_version": 1,
  "sheets": [
    {"index": 0, "name": "Sheet1", "row_count": 1200, "column_count": 18},
    {"index": 1, "name": "统计", "row_count": 32, "column_count": 8}
  ],
  "active_sheet": 0,
  "row_offset": 0,
  "row_limit": 200,
  "rows": [
    {"row": 0, "cells": [{"column": 0, "value": "日期"}, {"column": 1, "value": "金额"}]}
  ],
  "merged_cells": [{"start_row": 0, "start_column": 0, "row_span": 1, "column_span": 2}],
  "next_row_offset": 200
}
```

规则：

- `sheet_index` 默认 0；越界返回参数错误。
- `row_limit` 默认 200，范围 20～500。
- 返回稀疏单元格并同时返回工作表维度，前端据此保留空单元格。
- 公式 MVP 返回缓存值或公式文本，不执行公式。
- 合并单元格只展示左上角值，其余占位由 `merged_cells` 描述。
- 超过配置资源上限时将 Table Preview 标记 failed，Descriptor 自动降级到 PDF。

### 3.5 重试接口

```http
POST /api/v1/documents/:document_id/preview/retry
```

- 只允许重试当前内容版本中 `failed` 的派生预览。
- `ready` 返回幂等成功；`processing` 返回当前状态；`unsupported` 不可重试。
- 重试只清空 Preview 错误并重新入队，不修改文档加工状态。

## 4. 文件类型路由与降级策略

| 来源/格式 | 主预览 | 主数据源 | 后台任务 | fallback 顺序 |
| --- | --- | --- | --- | --- |
| 手工 TXT | text | `documents.content` | 无 | none |
| 手工 Markdown | markdown | `documents.content` | 无 | none |
| TXT | text | Parsed Artifact | 无 | original download |
| Markdown | markdown | Parsed Artifact + Asset URL | 无 | original download |
| URL | markdown | Parsed Artifact | 无 | source URL |
| PDF | pdf | Original Artifact | 无 | parsed markdown → download |
| DOCX | pdf | `rendered.pdf` | LibreOffice | parsed markdown → download |
| PPTX | pdf | `rendered.pdf` | LibreOffice | parsed markdown → download |
| XLSX | table | `sheet-data.json.zst` | XLSX extractor | rendered PDF → parsed markdown → download |
| 图片 | image | Original Artifact | 无 | OCR markdown → download |
| ZIP | 按已解出的主文档类型 | 当前任务已保存的主文档对象 | 同主文档 | 同主文档 |
| 未知格式 | none | 无 | 无 | download（若有原文件） |

Preview Router 的格式识别优先级：

1. `source_type=manual` 使用 `content_format`；
2. URL 使用 Parsed Artifact 的 `source.format`；
3. 文件优先使用 `original_file_name` 扩展名，MIME 仅作交叉校验；
4. ZIP 当前上传流程已把主文档名称、MIME 和对象写入任务/文档，因此按主文档路由；
5. 扩展名和 MIME 冲突时记录 warning，并优先使用受支持的扩展名；禁止前端参与判断。

## 5. 数据库设计

新增迁移：

```text
scripts/migrations/000017_document_previews.up.sql
scripts/migrations/000017_document_previews.down.sql
```

### 5.1 `document_previews`

```sql
CREATE TABLE document_previews (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    document_id uuid NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    content_version int NOT NULL CHECK (content_version > 0),
    preview_type varchar(20) NOT NULL
        CHECK (preview_type IN ('pdf', 'table')),
    status varchar(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'unsupported')),
    render_hash varchar(64) NOT NULL,
    renderer varchar(64) NOT NULL,
    renderer_version varchar(128) NOT NULL,
    object_key text,
    manifest_key text,
    media_type varchar(128),
    object_size bigint CHECK (object_size IS NULL OR object_size >= 0),
    attempt int NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    error_code varchar(64),
    error_message text,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(document_id, content_version, preview_type, render_hash)
);

CREATE INDEX idx_document_previews_lookup
    ON document_previews(document_id, content_version, preview_type, updated_at DESC);

CREATE INDEX idx_document_previews_worker
    ON document_previews(status, created_at)
    WHERE status IN ('pending', 'processing');
```

只为需要派生产物的 PDF/table 建记录。text、markdown、image 和原始 PDF 的状态由 Preview Router 根据原文件/Parsed Artifact 动态计算，避免无意义的数据行。

### 5.2 泛化 `task_outbox`

当前 `task_outbox.aggregate_id` 外键只允许引用 `import_tasks`。迁移中：

```sql
ALTER TABLE task_outbox
    DROP CONSTRAINT task_outbox_aggregate_id_fkey;
```

保留 UUID `aggregate_id`，由 `event_type` 决定其含义：

- `document.parse`：`aggregate_id = import_tasks.id`；
- `document.preview.render`：`aggregate_id = document_previews.id`。

Preview 调度必须在同一数据库事务内完成：

1. 幂等 upsert `document_previews`；
2. 新建 `task_outbox` 事件；
3. 提交事务。

Down migration 先删除 `document.preview.render` 事件和 `document_previews`，再恢复原外键，保证可回滚。

## 6. Preview Artifact 设计

### 6.1 对象路径

沿用当前 Parsed Artifact 的 `content-{version}` 分层：

```text
derived/{user_id}/{document_id}/content-{content_version}/
├── parse-{parse_config_hash}/
│   ├── parsed-document.json.zst
│   ├── assets/
│   └── manifest.json
├── preview-{office_render_hash}/
│   ├── rendered.pdf
│   └── manifest.json
└── preview-{xlsx_render_hash}/
    ├── sheet-data.json.zst
    └── manifest.json
```

不采用原方案中缺少 `content_version` 层级的路径，确保与项目现有 ArtifactKeyPrefix 一致，也便于按内容版本 GC。

### 6.2 Render Hash

```text
render_hash = SHA256(canonical_json({
  source_sha256,
  content_version,
  preview_type,
  renderer,
  renderer_version,
  strategy_version,
  render_options
}))
```

建议策略常量：

- `office-pdf-v1`；
- `xlsx-table-v1`。

LibreOffice 的 `renderer_version` 在启动时通过 `soffice --version` 获取并缓存；XLSX renderer 使用代码中的稳定适配器版本。修改输出语义时必须提升 strategy/renderer version。

### 6.3 Manifest

```json
{
  "artifact_schema_version": "1.0",
  "document_id": "doc-id",
  "content_version": 1,
  "source_sha256": "...",
  "preview_type": "pdf",
  "render_hash": "...",
  "renderer": "libreoffice",
  "renderer_version": "...",
  "strategy_version": "office-pdf-v1",
  "object": {
    "key": ".../rendered.pdf",
    "sha256": "...",
    "size": 123456,
    "media_type": "application/pdf"
  },
  "created_at": "2026-08-12T00:00:00Z"
}
```

保存顺序与 Parsed Artifact 一致：

1. 生成临时本地文件；
2. 校验内容；
3. 上传产物对象；
4. 最后上传 `manifest.json`；
5. 再将数据库状态改为 `ready`。

读取时校验 manifest、source hash、render hash、对象存在性、大小和产物魔数。PDF 同时校验 `%PDF-` 文件头及 `%%EOF`；Zstd JSON 校验解压上限和 schema。

### 6.4 旧缓存处理

现有 `rendered/{user}/{document}/v{version}.pdf` 没有 source hash、renderer version 和 manifest，不自动认领为新 Artifact：

- 兼容 `/rendered` 在迁移期只读取新 Artifact；缺失时返回 not ready，不再生成旧缓存。
- 新 Worker 按需重新生成 Office Preview Artifact。
- 发布稳定后增加一次性脚本或生命周期规则清理旧 `rendered/` 前缀。
- 不在用户请求中同步搬迁或删除旧对象。

## 7. 后端模块与精确改动

### 7.1 新增 Preview 包

```text
internal/service/preview/
├── types.go                 # PreviewType/Status/Descriptor/Artifact manifest
├── service.go               # GetDescriptor/GetText/OpenRendered/OpenTable/Retry
├── router.go                # 文档格式 → Viewer 与 fallback
├── scheduler.go             # 幂等创建任务并写 Outbox
├── processor.go             # Worker 任务领取、执行、回写
├── artifact_store.go        # Preview Artifact 保存/解析/完整性校验
└── renderer/
    ├── renderer.go          # Renderer 接口
    ├── libreoffice.go       # 从现有 OfficeConverter 迁移
    └── xlsx.go              # 工作簿结构化提取
```

核心接口：

```go
type Service interface {
    GetDescriptor(ctx context.Context, userID, documentID string) (*Descriptor, error)
    GetText(ctx context.Context, userID, documentID string) (*TextPreview, error)
    OpenRendered(ctx context.Context, userID, documentID string) (*PreviewFile, error)
    GetTable(ctx context.Context, userID, documentID string, query TableQuery) (*TablePage, error)
    Ensure(ctx context.Context, documentID string) error
    Retry(ctx context.Context, userID, documentID string) error
}

type Processor interface {
    Process(ctx context.Context, previewID string) error
}

type Renderer interface {
    Type() PreviewType
    RendererInfo(ctx context.Context) (name, version string, err error)
    Render(ctx context.Context, input RenderInput) (*RenderResult, error)
}
```

当前 `documentService.Preview`、`loadParsedDocument`、图片占位符重写逻辑迁到 Preview Service；上传、列表、删除等文档业务继续留在 `document_service.go`。迁移时优先搬迁代码而不是复制两份实现。

### 7.2 Repository

新增：

```text
internal/model/entity/document_preview.go
internal/repository/document_preview_interface.go
internal/repository/document_preview_repository.go
```

Repository 至少提供：

- `FindCurrent`：按 document/content version/type/hash 查询；
- `EnsurePendingWithOutbox`：事务性 upsert + outbox；
- `ClaimPendingByID`：`pending → processing`，递增 attempt；
- `MarkReady`；
- `MarkFailed`；
- `Requeue`；
- `RecoverStale`；
- `ListHistoricalForGC`。

所有状态迁移使用带旧状态的条件更新，防止两个 Consumer 同时执行同一转换。

### 7.3 Background Manager

将当前单文件 `internal/background/manager.go` 拆为：

```text
internal/background/
├── manager.go
├── outbox_publisher.go
├── document_consumer.go
└── preview_consumer.go
```

Outbox Publisher 按事件类型路由：

```text
document.parse          → config.document_consumer.stream
document.preview.render → config.preview.consumer.stream
```

未知事件类型不得错误投递到文档解析 Stream：记录错误并保留未发布状态，等待修复。

Preview Consumer：

1. 从 Preview Stream 读取 `preview_id`；
2. 原子领取 pending 记录；
3. 使用独立 ProcessingTimeout；
4. 执行对应 renderer；
5. 成功后写 Artifact 并 MarkReady；
6. 失败且未达到 MaxAttempts 时 Requeue + Outbox；
7. 达上限后 MarkFailed；
8. ACK Redis 消息。

Preview 和 Document Consumer 使用不同 Stream/Group/并发配置，Office 慢任务不会占用文档解析 Consumer。

### 7.4 调度时机

采用双触发、同一幂等入口：

1. 导入任务已创建/复用 Document 且原文件对象可用后，best-effort 调用 `PreviewScheduler.Ensure`。调用失败只记日志，不能返回文档加工失败。
2. `GetDescriptor` 发现当前内容版本缺少应有的 Preview 行时，再次调用 `Ensure` 自愈并返回 `processing`。

这样 Preview 可与 DocumentPipeline 并行，也能覆盖升级前已经存在的 Office 文档。不要把 LibreOffice 节点加入 Eino Graph。

### 7.5 LibreOffice Renderer

从现有 `internal/service/office_converter.go` 迁入 Preview renderer，并修改：

- 删除 converter 内部的固定 `sync.Mutex`；
- Processor 按 `max_concurrency` 使用 semaphore；
- timeout 从固定 5 分钟改为配置，但默认仍为 5 分钟；
- 每个转换使用独立临时目录和独立 LibreOffice UserInstallation；
- 保留 PDF 头、尾和最小体积校验；
- 禁止读取全部超大 PDF 到内存后上传，改为文件流上传并边读边计算 SHA256；
- 日志只记录 preview/document ID、耗时、文件大小和错误分类，不记录原文内容。

建议 LibreOffice 参数：

```text
--headless
--norestore
--nodefault
--nolockcheck
-env:UserInstallation=file://<isolated-profile-dir>
--convert-to pdf
--outdir <temp-dir>
<source-file>
```

### 7.6 XLSX Renderer

为避免修改 RAG ParsedDocument 契约，MVP 在 Go Preview Worker 中使用工作簿读取库（例如 `excelize`）直接生成 Preview Artifact：

- 列出 Sheet 顺序与名称；
- 读取工作表维度；
- 提取单元格显示值/公式文本；
- 提取合并区域；
- 生成稀疏行列 JSON；
- Zstd 压缩后写入 `sheet-data.json.zst`。

资源上限全部配置化，建议默认：

```yaml
max_sheets: 100
max_rows_per_sheet: 100000
max_columns_per_sheet: 500
max_cells: 500000
max_uncompressed_bytes: 67108864
```

任一上限超出时返回 `PREVIEW_TABLE_TOO_LARGE`，Table 状态 failed；PDF fallback 任务仍独立执行。不要让 Table 失败覆盖 PDF 状态。

二期若确认 Docling 能稳定返回 Sheet identity，可将 Sheet 元数据作为 ParsedDocument 的可选字段复用，但不作为本次 P0 前置条件。

### 7.7 Controller 与路由

修改：

```text
internal/api/v1/document/controller.go
internal/api/v1/document/routes.go
internal/api/router.go
internal/model/dto/response/document.go
internal/app/server.go
```

Controller 注入独立 `preview.Service`，文档 CRUD 仍注入 `DocumentService`。新增方法：

- `PreviewDescriptor`；
- `PreviewText`；
- `PreviewRendered`；
- `PreviewTable`；
- `RetryPreview`。

`Rendered` 暂时保留为 deprecated alias。`api.Dependencies` 和 `ServerApp.Initialize` 增加 PreviewService/Processor/Repository 的装配。

### 7.8 错误码

在 `internal/contracts/error_code.go` 和 `internal/api/httperror/mapper.go` 增加：

```text
PREVIEW_NOT_READY
PREVIEW_RENDER_TIMEOUT
PREVIEW_RENDER_FAILED
PREVIEW_UNSUPPORTED
PREVIEW_ARTIFACT_MISSING
PREVIEW_ARTIFACT_CORRUPTED
PREVIEW_TABLE_TOO_LARGE
```

数据库/MinIO/LibreOffice 原始错误不能直接暴露给客户端；`error_message` 保存经过截断和脱敏的用户可读信息，完整错误进入结构化日志。

## 8. 配置修改

`pkg/config/config.go` 新增：

```go
type PreviewConfig struct {
    Enabled  bool
    Consumer PreviewConsumerConfig
    Office   OfficePreviewConfig
    XLSX     XLSXPreviewConfig
}
```

示例配置：

```yaml
preview:
  enabled: true
  consumer:
    stream: "memora:document:preview"
    group: "memora-preview"
    concurrency: 2
    block_timeout: 5s
    processing_timeout: 10m
    claim_idle: 15m
    max_attempts: 3
  office:
    enabled: true
    max_concurrency: 1
    timeout: 5m
  xlsx:
    enabled: true
    max_sheets: 100
    max_rows_per_sheet: 100000
    max_columns_per_sheet: 500
    max_cells: 500000
    max_uncompressed_bytes: 67108864
```

同步修改：

- `configs/config.yaml.example`；
- `.env.example`；
- `pkg/config/loader.go` 的环境变量绑定；
- `Config.Validate()`；
- `pkg/config/config_test.go`。

生产建议 `office.max_concurrency=1`。即使提高 Preview Consumer 并发，Office semaphore 仍限制 soffice；XLSX 任务可占用其他 Worker。

## 9. 前端改造

### 9.1 文件结构

```text
web/admin/src/features/document/
├── api.ts
├── types.ts
└── components/
    ├── DocumentViewer.tsx
    └── preview/
        ├── PreviewHost.tsx
        ├── PdfViewer.tsx
        ├── MarkdownViewer.tsx
        ├── TextViewer.tsx
        ├── ImageViewer.tsx
        ├── TableViewer.tsx
        └── PreviewStatus.tsx
```

### 9.2 API 与类型

`DocumentPreview` 改为 Descriptor 类型，并新增：

- `DocumentTextPreview`；
- `DocumentTablePreview`；
- `PreviewFallback`；
- `PreviewError`。

API 方法：

```ts
getDocumentPreviewDescriptor(documentId)
getDocumentTextPreview(contentUrl)
getDocumentPreviewBlob(contentUrl)
getDocumentTablePreview(contentUrl, params)
retryDocumentPreview(documentId)
```

API 客户端必须接受后端返回的相对 `content_url`，不能通过扩展名选择固定接口。

### 9.3 `DocumentViewer` 简化

删除当前组件中的：

- `isPdf/isDocx/isXlsx/isPptx/isOffice/isImage` 后端路由判断；
- `getRenderedDocument` 的 10 分钟同步请求；
- `/preview` 失败后自动连续读取 `/content` 的人工预览 fallback；
- 依赖文件格式决定默认展示模式的逻辑。

保留“视觉预览/解析正文”切换，但可用模式由 Descriptor 的 primary + fallbacks 生成。

轮询规则：

- Descriptor 为 `pending/processing` 时按 `retry_after_ms`（默认 2 秒）轮询；
- 文档切换或组件卸载立即停止；
- ready/failed/unsupported 停止；
- 最长主动轮询 10 分钟，超时后展示手动重试与 ready fallback；
- 不轮询 `/preview/rendered` 二进制接口。

### 9.4 Viewer 细节

**PDF Viewer**

- 增加 `pdfjs-dist`；
- 通过现有认证 API 下载 Blob，再交给 PDF.js；
- 支持页码、上一页/下一页、缩放、加载和错误状态；
- MVP 不要求文本选区、目录和缩略图。

**Image Viewer**

- 通过认证 API 读取 Blob，生成 Object URL；
- effect cleanup 中可靠 revoke，不使用固定 60 秒定时回收；
- 限制最大展示区域，允许新窗口查看。

**Markdown/Text Viewer**

- 复用现有 `ReactMarkdown + remark-gfm` 和样式；
- 保留签名 Asset URL 图片点击查看；
- `/content` 不再作为普通预览的隐藏兜底。

**Table Viewer**

- MUI Tabs 展示 Sheet；
- 表头 sticky、横向滚动；
- 以 row cursor/offset 分页，MVP 每页 200 行；
- 单元格支持复制；
- 合并单元格按 API 描述展示；
- 大表后续可用项目已安装的 `react-virtuoso` 升级为虚拟滚动，但不阻塞 P0。

## 10. 安全与资源控制

- 所有 Descriptor/text/rendered/table/retry 接口先通过 `userID + documentID` 校验所有权。
- Asset 接口继续使用 HMAC；Preview 主资源继续使用 Bearer，不新增公开对象 URL。
- 临时目录使用系统临时目录创建，源文件名只取安全 basename。
- LibreOffice 使用参数数组执行，不经过 shell。
- 临时文件在成功、失败、超时和进程取消路径都清理。
- 读取 Artifact 使用压缩前/后大小限制，防止压缩炸弹。
- XLSX 禁止外部链接更新和宏执行，只读取数据。
- PDF 输出必须完成结构校验后才能发布 manifest/ready。
- 日志和 API 均不得包含单元格全文、文档正文、JWT、MinIO 凭据或对象签名。

## 11. 幂等、失败恢复与 GC

### 11.1 幂等

- 相同 document/content version/type/render hash 只允许一行。
- 同一 Preview ID 只有一个 Consumer 能从 pending 领取。
- Redis 重复消息在发现 ready/processing/failed 终态后安全 ACK。
- 已 ready 的 Artifact 通过完整性校验后直接缓存命中。

### 11.2 恢复

- Preview Manager 启动时将超过 `claim_idle` 的 processing 恢复为 pending 并写新 Outbox。
- 转换超时使用 `PREVIEW_RENDER_TIMEOUT`。
- MinIO 上传失败不得提前写 manifest 或 ready。
- Descriptor 发现数据库 ready 但 Artifact 损坏时，将记录置 failed，返回 fallback，并允许重试。

### 11.3 GC

P1 增加定时 GC：

- 保留当前 `content_version` 对应的 Preview Artifact；
- 历史版本保留 7 天后删除；
- 删除软删除文档的派生对象；
- 删除无 manifest 的超时临时前缀；
- 数据库行删除和 MinIO 删除采用可重试流程，不在用户 DELETE 请求中扫描整个前缀。

## 12. 测试计划

### 12.1 Go 单元测试

新增/调整：

```text
internal/service/preview/router_test.go
internal/service/preview/service_test.go
internal/service/preview/scheduler_test.go
internal/service/preview/processor_test.go
internal/service/preview/artifact_store_test.go
internal/service/preview/renderer/libreoffice_test.go
internal/service/preview/renderer/xlsx_test.go
internal/repository/document_preview_repository_test.go
internal/background/preview_consumer_test.go
internal/api/v1/document/controller_test.go
```

覆盖：

- 每种格式路由和 fallback 顺序；
- 所有权校验；
- Descriptor pending/processing/ready/failed/unsupported；
- Ensure 并发幂等；
- Outbox 与 Preview 行事务一致性；
- Consumer 重复消息、重试、达到最大次数、租约恢复；
- Office 超时、无 soffice、坏 PDF、坏缓存、MinIO 失败；
- XLSX 多 Sheet、空格、宽表、合并单元格、公式、资源超限；
- source/content version/renderer version 改变导致 render hash 改变；
- Preview 失败不修改 `documents.processing_status` 和 active index。

现有 `document_service_test.go` 中同步 Rendered 测试迁入 Preview 包；原文件和 Parsed text 测试继续保留。

### 12.2 前端测试与检查

- Descriptor 类型分发；
- processing 轮询与卸载取消；
- fallback 选择；
- PDF/Image Blob URL 生命周期；
- Table Sheet 切换、分页、空单元格和合并单元格；
- Retry 操作；
- `pnpm typecheck`、`pnpm lint`、`pnpm build`。

### 12.3 集成验收样本

至少准备：

- 中文/英文 TXT、Markdown（含本地图片）；
- 原生 PDF、扫描 PDF；
- 含图片/表格的 DOCX；
- 多页 PPTX；
- 3 个 Sheet、宽表、空单元格、合并单元格、公式的 XLSX；
- 无 OCR 文本的图片；
- ZIP + Markdown + 相对图片附件；
- 故意损坏的 Office 文件；
- 超过 XLSX Table 上限的文件。

## 13. 分阶段实施顺序

### 阶段 0：冻结契约与建立回归基线

1. 将本方案中的 Descriptor、Table API、错误码写入 `docs/API.md`。
2. 为当前 `/preview` 文本、`/rendered`、Parsed Asset 和 Office 缓存补足回归测试。
3. 记录现有前后端构建和测试结果。

验收：无功能变化，现有测试通过。

### 阶段 1：Descriptor 与文本接口拆分

1. 新建 Preview 类型、Router 和 Service。
2. 把当前正文预览迁到 `/preview/text`。
3. `/preview` 改为 Descriptor。
4. 先支持无需派生产物的 manual/TXT/Markdown/URL/PDF/image。
5. Office 暂返回 processing/failed + parsed text fallback。

验收：前端可只通过 Descriptor 查看上述直接预览类型；DocumentPipeline 无改动。

### 阶段 2：Preview Artifact 与异步 Office

1. 执行 000017 migration。
2. 实现 Repository、Artifact Store、Scheduler、Processor。
3. 泛化 Outbox Publisher，增加独立 Preview Consumer。
4. 迁移 LibreOffice converter，移除 HTTP 同步转换。
5. 增加 DOCX/PPTX PDF Artifact 和重试。
6. 保留 `/rendered` 只读兼容别名。

验收：首次 GET Descriptor 不阻塞；Office 后台完成后变 ready；转换失败不影响 RAG。

### 阶段 3：XLSX Table Preview

1. 实现 XLSX renderer 和 `sheet-data.json.zst`。
2. 增加 Table API 和资源上限。
3. XLSX 同时调度 table 主任务与 PDF fallback 任务。
4. 增加 TableViewer。

验收：多 Sheet 可切换、宽表可横向滚动、单元格可复制、PDF 可手动切换。

### 阶段 4：前端统一 Viewer

1. 重构 `DocumentViewer` 为 Descriptor 驱动。
2. 增加 PDF.js、Image、Markdown、Text、Table Viewer。
3. 删除文件扩展名到后端接口的路由逻辑。
4. 删除普通预览对 `/content` 的 fallback。
5. 完成轮询、错误、重试、下载交互。

验收：前端不出现 DOCX/XLSX/PPTX/PDF 路由分支，只出现 PreviewType 分支。

### 阶段 5：兼容清理与运维完善

1. API 文档标记旧 `/rendered` deprecated，稳定一个发布周期后删除。
2. 清理旧 `rendered/` 对象。
3. 增加 Preview 指标、告警和 GC。
4. 更新 README、ARCHITECTURE、FRONTEND 和部署配置。

## 14. 建议提交拆分

为降低评审和回滚风险，建议按以下提交拆分：

1. `docs: freeze preview descriptor and table contracts`
2. `feat(preview): add descriptor router and text endpoint`
3. `feat(preview): add preview artifact persistence`
4. `refactor(worker): route outbox events to dedicated streams`
5. `feat(preview): render office documents asynchronously`
6. `feat(preview): add xlsx structured preview artifact`
7. `feat(web): switch document viewer to preview descriptor`
8. `test(preview): add end-to-end format and failure coverage`
9. `docs: update preview architecture and deprecate rendered endpoint`

每个提交都应保证 Go 测试可运行；数据库迁移提交与依赖它的实体/Repository 同时合入。

## 15. 验收标准

全部满足才视为改造完成：

- [ ] `/preview` 对所有支持格式返回稳定 Descriptor。
- [ ] 前端不再根据文件扩展名决定调用 `/rendered`、`/original` 或文本接口。
- [ ] 普通 GET 请求内不存在 LibreOffice 调用。
- [ ] DOCX/PPTX Preview 可异步生成、缓存、重试和恢复。
- [ ] PDF 不产生重复 Preview Artifact。
- [ ] 图片默认展示原图，OCR 只用于正文 fallback/RAG。
- [ ] XLSX 默认 Table Viewer，支持 Sheet、横向滚动、分页和复制。
- [ ] XLSX PDF 是独立 fallback，Table 失败不覆盖 PDF 成功状态。
- [ ] Preview Artifact 与 source/content version/renderer version 绑定并有 manifest 完整性校验。
- [ ] Preview 失败不会改变文档索引成功状态、Chunk 或向量。
- [ ] `/content` 不再被前端用作普通视觉预览 fallback。
- [ ] 旧 `/rendered` 在兼容期只读，不再触发同步转换。
- [ ] Go 单测、集成测试、前端 typecheck/lint/build 全部通过。
- [ ] 配置示例、API 文档、架构文档和错误码文档同步完成。

## 16. 明确不在本次范围内

- Office 在线编辑、批注和协作；
- OnlyOffice、Collabora、kkFileView；
- PPT 动画和高保真播放；
- DOCX HTML 高保真重建；
- XLSX 公式计算和编辑；
- PPT/PDF 缩略图导航；
- 多实例 LibreOffice 集群；
- VLM 图片理解；
- 修改 DocumentPipeline、Chunk 策略或 Embedding 逻辑。

## 17. 主要风险与控制

| 风险 | 影响 | 控制措施 |
| --- | --- | --- |
| `/preview` 响应结构是破坏性变更 | 旧前端不可用 | 后端与前端同一发布；先增加 `/preview/text`；契约测试锁定 |
| Outbox 泛化错误投递 | 解析/预览任务串流 | event type 白名单路由；两个独立 Stream/Group；未知事件不发布 |
| LibreOffice 卡死或残留进程 | Worker 被占用 | context timeout、隔离 profile、semaphore、租约恢复 |
| XLSX 占用过多内存 | OOM/响应慢 | 单元格和解压大小上限、稀疏数据、异步生成、分页 API |
| DB ready 但 MinIO 产物不完整 | 永久坏缓存 | 产物先写、manifest 最后写、DB ready 最后更新、读取再校验 |
| Preview 调度失败 | Office 长期 processing/缺失 | 调度 best-effort + Descriptor 懒自愈 + 手动 retry |
| 用户切换文档时请求串扰 | 展示错误内容 | Query key 带 document/content version，取消轮询，Blob cleanup |
| 旧缓存占空间 | MinIO 膨胀 | 不自动认领；稳定后脚本/生命周期规则清理 |

本方案完成后，Memora 的解析链路和预览链路会保持真正独立：Docling/Go Parser 继续服务机器理解与 RAG，Preview Service 则为前端提供统一、可降级、可追踪、可缓存的人类阅读入口。
