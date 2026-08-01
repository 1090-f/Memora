# Memora Foundation API

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
```

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
