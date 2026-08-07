# 任务包 03 交付记录 · ImportTask 状态机与 Worker 接入

> 日期：2026-08-05
> 范围：成员一任务包 03。HTTP 只创建任务，Worker 领取并执行；建立文档流水线调度基础。
> 基线：Eino v0.9.13，go1.25.0

## 1. 修改文件清单

**新增**：
- `internal/worker/document/source.go`（Source：SKIP LOCKED 领取 + 生命周期回写）
- `internal/worker/document/handler.go`（Handler：只依赖编排接口）
- `internal/service/document_process_service.go`（DocumentProcessService 真实实现）
- `internal/api/v1/importtask/{controller,routes}.go`（任务列表/详情/重试 + 文档处理状态/重试/重索引）

**修改**：
- `internal/repository/document_interface.go`、`document_repository.go`（ImportTask 状态机方法 + Document 处理状态更新）
- `internal/service/document_process_interface.go`（接口扩展：ProcessImportTask/ListImportTasks/GetImportTask/RecoverStaleTasks）
- `internal/app/worker.go`（RegisterJob 显式注册 document.import）
- `internal/app/server.go`、`internal/api/router.go`（DI 注入 DocumentProcess）

## 2. 已实现行为

### 状态机（对齐计划 §7.3）
```text
pending → running → succeeded
                  ↘ failed
pending/running → skipped（去重命中）
failed → pending（显式重试 API）
running + 超租约 → pending（启动恢复）
```

### 关键实现
- **Reserve 行锁**：`FOR UPDATE SKIP LOCKED` 领取 pending 任务并原子置 running + 写 started_at；冲突时返回无任务，两个 Worker 不会处理同一任务。
- **幂等键**：`document.import:{task_id}:{started_at_ms}`，Runner 的 Redis 幂等存储防止重复执行；`MaxAttempts=1` 禁止自动重试（failed→pending 仅走显式 API）。
- **ProcessImportTask**（任务包 03 阶段）：创建文档行（source_type=file，引用 MinIO 对象）→ 回填 import_tasks.document_id → 标记任务 succeeded；已关联文档时直接复用（幂等，不生成重复 Document）；`duplicate_policy=skip` 且源哈希命中时标记 skipped。
- **恢复策略**：Worker 启动时 `RecoverStale` 将卡在 running 且超过 10min 租约的任务重置为 pending。
- **Handler 约束**：只依赖 `DocumentProcessService`（Processor 接口），无 SQL/Gin/Eino；响应 context 取消。
- **失败原因**：Runner 的 Fail 路径经 Source.Fail → 内部错误链安全落库 `failure_reason`。

### API
- `GET /knowledge-bases/{kb_id}/import-tasks`（分页）
- `GET /import-tasks/{task_id}`、`POST /import-tasks/{task_id}/retry`
- `GET /documents/{document_id}/processing`、`POST .../retry-processing`、`POST .../reindex`

## 3. 验证

- `go build ./...`、`go vet ./...`、`go test ./...`、`go build ./cmd/...`、`git diff --check`：全部通过。
- **运行态验收：未执行**。需要真实 PostgreSQL + Redis + MinIO 验证：两个 Worker 并发不重领、失败重试、恢复、`/health/workers` 不破坏。

## 4. 决策与偏差记录

1. **文档行在任务包 03 创建**：计划验收要求"重复执行不生成重复 Document"，故 Handler 阶段创建文档行（processing_status=pending），解析/分段留待任务包 04。
2. **`Source.Retry` 映射 FailTask**：Runner 自动重试被 `MaxAttempts=1` 禁用，状态机严格"failed→pending 仅显式重试"。
3. **Reindex 为占位**：任务包 03 仅校验归属与状态，新索引版本生成在任务包 06 接入。
4. **租约 10min**：保守默认值，可配置化留待后续。
5. **`GetProcessingStatus` 签名**：按 API 8.7 无 kb_id 路径，改为 userID+documentID 查询（内部附带 KnowledgeBaseID 供 Reindex 使用）。
6. **`CreateImportTask` 接口方法**：本包上传流程仍走 DocumentService.UploadFiles；此方法为 URL 导入（任务包 09）预留。

## 5. 复查修正记录（2026-08-05 第二轮审查）

| # | 问题 | 严重度 | 修复 |
|---|---|---|---|
| 1 | **双重 Complete**：ProcessImportTask 内部已标 succeeded，Runner 成功后 Source.Complete 再调 CompleteSucceeded 因 `WHERE status='running'` 不命中报错日志 | 严重 | `CompleteSucceeded` 改为 `WHERE status IN ('running','succeeded')` 幂等 |
| 2 | `ProcessImportTask` 中 `FindByImportTask` 非 NotFound 错误（DB 故障）被吞掉继续创建文档 → 可能重复创建 | 严重 | 区分 `ErrDocumentNotFound`，其他错误向上返回 |
| 3 | 去重检查 `FindBySourceHash` 的 DB 错误被当作"无重复"继续创建 | 重要 | 区分 `ErrDocumentNotFound`，其他错误返回 |
| 4 | **运行中恢复缺失**：RecoverStale 仅 Worker 启动时执行一次，运行中 Worker 崩溃的任务永久卡死 | 重要 | `ReservePending` 每次领取前执行 `recoverStaleLocked`（低成本 UPDATE，走部分索引） |
| 5 | `RetryProcessing` 未校验 failed 状态即返回成功（占位但语义错误） | 重要 | 校验 `processing_status=failed`，否则 `INVALID_STATE` |
| 6 | 租约常量在 service 与 repository 重复定义 | 低 | 统一到 repository 并导出 `ImportTaskLease()` |
| 7 | `Source.Retry` 映射 FailTask 但 `MaxAttempts=1` 下永不触发 | 低 | 保留，注释说明（防未来改 MaxAttempts 改变状态机） |

确认无问题项：
- `Source.Reserve` 的 StartedAt 由 ReservePending 保证非 nil，幂等键无 panic 风险；
- 创建文档与 CompleteSucceeded 非原子：失败后任务停留 running，靠租约恢复 → 幂等分支复用文档自愈；
- 路由 `/documents/:document_id/processing` 与 document controller 的 `/documents/:document_id` 不冲突（不同子路径）；
- `Retry` 重试后走幂等分支复用文档，任务包 04 需在幂等分支后重新触发解析。

遗留待协作：
- **`current_index_version` 无落库字段**：documents 表只有 active_index_version，API 8.7 的 current_index_version 当前返回 0 占位，任务包 06 重建索引时需补充设计（新 Migration 或临时版本概念）。
- **`CreateImportTask` 服务方法未接入**：上传流程走 DocumentService.UploadFiles；该方法为 URL 导入（任务包 09）预留。
- **每 2s 轮询时 recoverStaleLocked 全表 UPDATE**：无匹配行时成本低，可优化为节流（记入待办）。

## 6. 风险 / 待协作

- 任务包 04 将 `ProcessImportTask` 内部替换为 Eino 文档加工 Graph 调用；当前文档行停留 pending。
- MinIO 对象清理（删除文档/知识库后）仍由后续任务包处理。
- 恢复任务的 `started_at` 置 nil，重新领取时新幂等键，避免旧键冲突。

## 7. 下一步

任务包 04：基于 Eino 的 Markdown/TXT 最小文档加工链路。
