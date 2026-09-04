# Memora API

正式接口契约以根目录 `AI智能知识库_API接口文档_P0_轻量化版.md` 为准。

当前框架挂载：

```text
GET  /health/live
GET  /health/ready
GET  /health/workers
GET  /metrics
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/users/me
PATCH /api/v1/users/me
PATCH /api/v1/users/me/password

POST   /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases/:kb_id
PATCH  /api/v1/knowledge-bases/:kb_id
DELETE /api/v1/knowledge-bases/:kb_id
GET    /api/v1/knowledge-bases/:kb_id/search-config
PUT    /api/v1/knowledge-bases/:kb_id/search-config

GET  /api/v1/knowledge-bases/:kb_id/directories/tree
POST /api/v1/knowledge-bases/:kb_id/directories

POST   /api/v1/knowledge-bases/:kb_id/documents
GET    /api/v1/knowledge-bases/:kb_id/documents
GET    /api/v1/knowledge-bases/:kb_id/documents/:document_id/content
GET    /api/v1/documents/:document_id
GET    /api/v1/documents/:document_id/preview
GET    /api/v1/documents/:document_id/preview/text
GET    /api/v1/documents/:document_id/preview/rendered
GET    /api/v1/documents/:document_id/preview/table?sheet_index=0&row_offset=0&row_limit=200
POST   /api/v1/documents/:document_id/preview/retry
GET    /api/v1/documents/:document_id/original
GET    /api/v1/documents/:document_id/rendered
DELETE /api/v1/documents/:document_id
GET    /api/v1/documents/:document_id/processing
POST   /api/v1/documents/:document_id/retry-processing
POST   /api/v1/documents/:document_id/reindex
GET    /api/v1/documents/:document_id/index-versions

POST /api/v1/knowledge-bases/:kb_id/imports/files
POST /api/v1/knowledge-bases/:kb_id/imports/url
GET  /api/v1/knowledge-bases/:kb_id/import-tasks
GET  /api/v1/import-tasks/:task_id
POST /api/v1/import-tasks/:task_id/retry

POST /api/v1/knowledge-bases/:kb_id/search
POST /api/v1/knowledge-bases/:kb_id/search/test
```

`GET /health/workers` 使用 Redis TTL 心跳判断 Worker 是否存活，并在 `workers` 中返回文档处理和
预览队列的 `pending/running/failed/retried`、最老待处理任务年龄、Redis Consumer Group
Pending 数以及数据库 Outbox backlog。没有有效心跳时接口返回 503，但仍在错误详情中保留已取得的
队列诊断数据。运行时间超过对应 `processing_timeout` 的任务计入 `stalled`；存在停滞任务但心跳仍
有效时返回 HTTP 200 和 `status=degraded`，并同时导出
`memora_worker_stalled_tasks{job_type=...}`，供 Prometheus 主动告警。

文档处理详情额外返回 `task_id`、`failure_code`、`recovery_advice` 与标准阶段数组
`stages[]`。阶段名固定为 `upload/parse/normalize/chunk/embed/index/preview`，状态固定为
`pending/running/succeeded/failed/skipped`。重试处理与重新索引响应均返回新建的 `task_id`
和 `status=processing`，前端应立即切换到处理中并开始轮询。运行任务超过 15 分钟没有结束时，
详情返回 `stalled=true` 与 `failure_code=TASK_STALLED`，用于提示检查 Worker 健康状态。

Agent 运行详情返回 `trace_id`、`request_id`、`router_confidence`、
`router_fallback_used`、`first_token_at`、`first_token_latency_ms`、
`model_generate_duration_ms`、`failure_stage`、`retryable`、`recovery_advice`
与脱敏的 `execution_trace`。其中 `first_token_latency_ms` 表示从运行开始到首个可见回答片段的延迟；
当前非 token 流式模型链路不将其表述为模型供应商的原生首 Token 延迟。SSE 事件保留既有 `event_type/data`
兼容字段，并增加顶层 `stage/status/trace_id/request_id`；不得在这些字段或摘要中写入
完整 Prompt、文档正文、认证信息或工具敏感参数。

文档页先请求 `GET /documents/:document_id/preview` 获取统一预览描述器。调用方只依据
`preview_type`、`status`、`content_url` 和 `fallbacks` 选择 Viewer，不再自行按扩展名或
MIME 路由。`pending`/`processing` 描述器会返回 `retry_after_ms`；预览失败后可调用 retry
接口重新排队。`rendered` 旧接口仅作为兼容别名读取已经生成的 Office PDF，不执行同步转换。

预览类型包括 `text`、`markdown`、`pdf`、`image`、`table`、`download` 和 `none`。XLSX
结构化表格接口按 Sheet 和行分页，`row_limit` 为 20～500，并返回 Sheet 尺寸、稀疏单元格、
合并单元格和下一页偏移量。

URL 导入的 HTTP 请求只创建异步任务。Worker 使用安全 Web Loader 抓取，只允许 HTTP/HTTPS，并限制 DNS 目标、重定向、响应类型、响应大小和超时。

搜索请求的 `mode` 支持 `keyword`、`vector`（兼容 `semantic`）和 `hybrid`；`top_k` 最大为 20。正常无有效知识时返回成功结果且 `knowledge_status=insufficient`。

统一响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {},
  "request_id": "uuid"
}
```

所有 HTTP 响应返回 `X-Request-ID` 与 `X-Trace-ID`。调用方可传入 W3C `traceparent`，服务端提取其中的 Trace ID；未提供时自动生成。

`observability.retention_days` 控制 `agent_events`、`document_processing_events` 与 `trace_spans`
的保留期，后台进程启动时及之后每 24 小时清理一次。HTTP、模型、Redis Stream 文档/预览任务与
Streamable HTTP MCP 请求会传播 W3C Trace Context；Go → Python Parser 调用由 Go 客户端 Span 记录，关键 Go Span 经过属性白名单
裁剪后批量写入现有 PostgreSQL。旧队列消息没有 `traceparent` 时仍使用任务表中的 `trace_id`
作为兼容回退，不需要 Collector、Tempo、Jaeger 或 Grafana。

`GET /api/v1/agent/runs/:id/trace` 返回当前用户指定运行的持久化 Span，包含父子 Span ID、名称、
类型、状态、起止时间、耗时、安全属性和事件。接口先校验运行归属，不允许通过 Trace ID 越权查询。
