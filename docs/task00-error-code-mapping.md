# 任务包 00 交付记录 · 错误码映射清单（成员一）

> 日期：2026-08-05
> 规则：稳定错误码只在 `internal/contracts/error_code.go`；HTTP 状态与公开消息只在 `internal/api/httperror`；Controller 统一使用 `internal/api/response`。成员一不新建错误码枚举、不复制错误消息表。

## 1. 成员一预期错误 → contracts → HTTP 映射

| # | 触发场景 | contracts.ErrorCode | HTTP | httperror 状态 | 已存在 |
|---|---|---|---|---|---|
| 1 | 请求参数非法（query 空、TopK 超限、配置越界） | `INVALID_ARGUMENT` | 400 | `StatusBadRequest` | 是 |
| 2 | 未认证 | `UNAUTHORIZED` | 401 | `StatusUnauthorized` | 是 |
| 3 | 越权（跨用户/跨知识库） | `FORBIDDEN` | 403 | `StatusForbidden` | 是 |
| 4 | 资源不存在 | `RESOURCE_NOT_FOUND` | 404 | `StatusNotFound` | 是 |
| 5 | 状态非法（任务已结束、文档状态不符） | `INVALID_STATE` | 409 | `StatusConflict` | 是 |
| 6 | 重复资源（KB 重名、重复任务） | `DUPLICATE_RESOURCE` | 409 | `StatusConflict` | 是 |
| 7 | 索引版本冲突 | `INDEX_VERSION_CONFLICT` | 409 | `StatusConflict` | 是 |
| 8 | 请求/文件过大 | `PAYLOAD_TOO_LARGE` | 413 | `StatusRequestEntityTooLarge` | 是 |
| 9 | 不支持的文件类型 | `UNSUPPORTED_FILE_TYPE` | 415 | `StatusUnsupportedMediaType` | 是 |
| 10 | 知识不足（契约显式失败场景） | `KNOWLEDGE_INSUFFICIENT` | 422 | `StatusUnprocessableEntity` | 是 |
| 11 | 模型调用失败（Embedding/Reranker） | `MODEL_CALL_FAILED` | 502 | `StatusBadGateway` | 是 |
| 12 | 上游超时（MinIO/模型/网络） | `UPSTREAM_TIMEOUT` | 504 | `StatusGatewayTimeout` | 是 |
| 13 | 依赖未就绪（检索图编译失败等） | `SERVICE_UNAVAILABLE` | 503 | `StatusServiceUnavailable` | 是 |
| 14 | 未知内部错误 | `INTERNAL_ERROR` | 500 | `StatusInternalServerError` | 是 |

## 2. 结论

- 成员一全部预期错误均可复用现有 `error_code.go` 常量，**本包无新增错误码**。
- `internal/api/httperror/mapper.go` 与 `docs/API.md` 均无需改动。
- `KNOWLEDGE_INSUFFICIENT` 只用于契约/API 显式要求失败返回的少数场景；常规无有效结果检索返回 `knowledge_status = "insufficient"`（HTTP 200），不默认 422。

## 3. 审计发现的文档差异（需协调，不在本包改码）

| 差异 | 位置 | 建议 |
|---|---|---|
| API 1.7 错误码清单未列 `SERVICE_UNAVAILABLE`，但代码与健康检查已使用 | API 文档 | API 文档补充该码 |
| API 7.4 删除非空目录返回 `DIRECTORY_NOT_EMPTY`，contracts 无此码 | API 文档 ↔ contracts | 任务包 01 目录删除实现时评估是否新增；新增须同步 httperror + API 文档 |
