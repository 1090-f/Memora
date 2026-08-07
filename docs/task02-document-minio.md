# 任务包 02 交付记录 · 文档 CRUD 与 MinIO 对象能力

> 日期：2026-08-05
> 范围：成员一任务包 02。手工文档、文件元数据与安全的对象存储操作；不执行解析（解析在任务包 04）。
> 基线：Eino v0.9.13（go.mod 已锁定），go1.25.0

## 1. 修改文件清单

**新增**：
- `pkg/objectstore/key.go`（对象 key 规范 + 文件名净化，防路径穿越）
- `internal/model/entity/document.go`、`import_task.go`
- `internal/model/dto/request/document.go`、`internal/model/dto/response/document.go`
- `internal/repository/document_interface.go`、`document_repository.go`（Document + ImportTask 最小能力）
- `internal/service/document_interface.go`、`document_service.go`
- `internal/api/v1/document/{controller,routes}.go`

**修改**：
- `pkg/objectstore/minio.go`（PutObject/OpenObject/StatObject/RemoveObject + ObjectInfo + ErrObjectNotFound）
- `internal/contracts/document_status.go`（追加 `ImportTaskStatus` 枚举，对齐 000004）
- `internal/app/server.go`、`internal/api/router.go`（DI 注入 Documents）

## 2. 已实现行为

- **手工文档 CRUD**：`POST /knowledge-bases/{kb_id}/documents`、`GET .../documents`（page/page_size/keyword/directory_id/processing_status/source_type 过滤）、`GET /documents/{id}`、`DELETE /documents/{id}`。
- **文件导入**：`POST /knowledge-bases/{kb_id}/imports/files`（multipart，files[]，单次最多 20 个，单文件最大 50MB）：
  1. 校验扩展名（md/txt/pdf/docx）、空文件、大小；
  2. 创建 `import_tasks`（status=pending）；
  3. 流式上传 MinIO（不 `io.ReadAll`），上传同时流式计算 SHA-256；
  4. 更新任务 MinIO bucket/object key/source_hash。
- **补偿逻辑**：MinIO 上传成功但 DB 更新失败 → 删除已上传对象（删除失败记录日志）；上传失败 → 删除任务记录。
- **MinIO 能力**：`PutObject`（限大小、显式 content type）、`OpenObject`（可关闭流）、`StatObject`、`RemoveObject`；错误包装不泄漏 AccessKey/SecretKey。
- **对象 key 规范**：`documents/{user_id}/{kb_id}/{task_id}/{safe_file_name}`；文件名净化防路径穿越与任意 key 注入。
- **归属过滤**：文档详情/删除/任务查询均在同一 SQL 组合 `id + user_id + kb_id(适用时) + deleted_at IS NULL`。

## 3. 验证

- `go build ./...`、`go vet ./...`、`go test ./...`、`go build ./cmd/...`、`git diff --check`：全部通过。
- **运行态验收：未执行**（需真实 PostgreSQL + MinIO）。

## 4. 决策与偏差记录

1. **`duplicate_policy` 默认值**：迁移 `000004` 默认 `create_new`，API 文档 8.4 写 `skip`。本包服务层默认 `skip`（与 API 文档一致），DB 默认值不改（禁止回写 Migration）；显式传值不受影响。记入契约冻结记录待三方确认。
2. **上传失败错误码**：MinIO 上传失败映射 `UPSTREAM_TIMEOUT`（上游依赖错误），DB 失败映射 `INTERNAL_ERROR`。
3. **手工文档 processing_status=pending**：手工文档正文直接入库，仍进入异步处理管线（任务包 04 起），与 API 8.1 响应一致。
4. **ImportTask 状态机**：任务包 02 只创建任务与对象信息；领取/重试/恢复在任务包 03 实现。
5. **文件大小限制**：单文件 50MB、单次 20 个、正文 2MB，为保守默认值集中定义，未读取配置（配置项后续可加）。

## 5. 复查修正记录（2026-08-05 第二轮审查）

| # | 问题 | 严重度 | 修复 |
|---|---|---|---|
| 1 | `UpdateObjectInfo` 写入 `updated_at`，但 `import_tasks` 表无该列 → 上传必失败 | 严重 | 移除该字段，仅更新 bucket/object key/source_hash |
| 2 | controller 文件句柄泄漏：循环中校验失败提前 return 时，defer 尚未注册，已打开文件泄漏 | 严重 | defer 移到循环前注册，统一关闭 |
| 3 | MinIO 上传失败一律映射 `UPSTREAM_TIMEOUT`，非超时错误（连接拒绝/权限）语义错误 | 中 | 改映射 `SERVICE_UNAVAILABLE` |
| 4 | `mimeTypeOf(fileName, ext)` 的 `fileName` 参数从未使用 | 低 | 移除死参数 |
| 5 | `CreateManual` 先校验目录后校验知识库，kb 不存在时误报目录错误 | 低 | 调整校验顺序：先 kb 后目录 |
| 6 | `UploadFiles` 单文件失败时，已创建的任务/对象残留（半成品） | 中 | 新增 `compensateUploads`：失败时回滚本次已创建任务与对象 |
| 7 | `putObjectWithHash` 无显式超时，依赖 minio 内部 30s/1min 默认 | 中 | 增加 `minioUploadTimeout = 5min` 上下文超时 |
| 8 | service 持有未使用的 `Transactor`（事务禁包 MinIO I/O，设计矛盾） | 低 | 移除依赖，注释说明补偿策略 |
| 9 | MinIO 上传失败后 `tasks.Delete` 错误被静默忽略 | 低 | 补日志 |
| 10 | controller `closers` 用 `interface{ Close() error }` | 低 | 改为 `io.Closer` |

确认无问题项：
- minio-go multipart 按 partSize（默认 16MB）分块读取，不整体 `io.ReadAll`，流式上传安全；
- `Get`/`Delete` 文档无 kb 过滤符合 API 8.3/8.x 路径（无 kb_id 参数），user+doc 过滤足够；
- List 的 `processing_status` 未校验枚举，非法值仅返回空列表，无安全危害；
- `BuildObjectKey` 路径穿越防护：文件名净化 + `path.Join`。

## 6. 风险 / 待协作

- `imports/url`（API 8.5）属任务包 09，本包未实现。
- 删除文档不删除 MinIO 对象（MinIO 清理属后台任务，任务包 03/10 处理）。
- 文件哈希用于 duplicate_policy=skip 的去重判断，实际去重逻辑在任务包 03 实现。

## 7. 下一步

任务包 03：ImportTask 状态机与 Worker 接入。
