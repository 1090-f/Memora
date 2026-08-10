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
