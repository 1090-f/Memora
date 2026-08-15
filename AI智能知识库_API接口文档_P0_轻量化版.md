# AI 智能知识库与知识服务 Agent 系统 API 接口文档

> 版本：MVP P0 轻量化数据版  
> 对应需求：统一问答 + Router Agent + ReAct-RAG + Plan-Execute + 跨会话长期记忆  
> 后端建议：Go + Gin + GORM + PostgreSQL + pgvector + Redis + MinIO  
> 接口风格：REST + JSON；Agent 执行事件使用 SSE

---

## 1. 文档说明

### 1.1 设计目标

本 API 文档对应 P0 需求，覆盖：

- 用户与认证；
- 多知识库；
- 文档目录；
- 文档创建、查看、删除；
- PDF / DOCX / Markdown / TXT 文件导入；
- 静态网页 URL 导入；
- 文档处理、重试与重新索引；
- 关键词、向量、混合检索；
- 搜索配置；
- AI 模型配置；
- 会话与消息；
- 统一智能问答；
- Router Agent；
- ReAct-RAG；
- Plan-Execute；
- AgentRun、Plan、Step、Round、ToolCall、Citation；
- SSE 执行事件；
- 跨会话长期记忆；
- MCP Server、工具发现、只读工具启停；
- Agent 与 MCP 工具授权关系；
- Agent 任务停止与整体重试。

P0 不提供：

- 用户手工选择 ReAct / Plan-Execute；
- 独立“普通 RAG 问答”入口；
- 在线文档编辑；
- MCP stdio；
- MCP 写工具；
- 工具人工确认；
- Checkpoint 恢复；
- SubAgent / 多 Agent；
- 可视化 Workflow；
- Memory 手工编辑；
- Plan 版本回滚。

### 1.2 Base URL

```text
/api/v1
```

示例：

```text
POST /api/v1/auth/login
GET  /api/v1/knowledge-bases
POST /api/v1/knowledge-bases/{knowledge_base_id}/questions
```

### 1.3 认证

除登录接口外，默认要求：

```http
Authorization: Bearer <access_token>
```

身份相关字段必须由后端根据登录态注入，前端和模型不得传入或覆盖：

- `user_id`
- `agent_run_id`
- 工具运行时的 `knowledge_base_id`
- `allowed_tool_names`

### 1.4 ID、时间、枚举

- ID：UUID 字符串；
- 时间：RFC3339 / ISO-8601，例如 `2026-07-31T14:30:00+09:00`；
- 数据库存储：`timestamptz`；
- JSON 字段使用 snake_case；
- 枚举值使用小写英文。

### 1.5 通用成功响应

```json
{
  "code": "OK",
  "message": "success",
  "data": {},
  "request_id": "8c36f645-5fb3-4f89-98d3-f61943ccb878"
}
```

### 1.6 通用分页响应

请求参数：

| 参数 | 类型 | 默认值 | 说明 |
|---|---:|---:|---|
| page | int | 1 | 页码，从 1 开始 |
| page_size | int | 20 | 每页数量，最大 100 |
| keyword | string | - | 名称等模糊搜索 |
| sort | string | updated_at_desc | 排序方式 |

响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "items": [],
    "page": 1,
    "page_size": 20,
    "total": 0
  },
  "request_id": "..."
}
```

### 1.7 通用错误响应

```json
{
  "code": "KNOWLEDGE_BASE_NOT_FOUND",
  "message": "knowledge base not found",
  "details": null,
  "request_id": "..."
}
```

建议错误码：

| HTTP | code | 说明 |
|---:|---|---|
| 400 | INVALID_ARGUMENT | 参数错误 |
| 400 | INVALID_STATE | 当前状态不允许该操作 |
| 400 | UNSUPPORTED_FILE_TYPE | 文件类型不支持 |
| 400 | WRITE_MCP_TOOL_FORBIDDEN | P0 禁止启用写工具 |
| 401 | UNAUTHORIZED | 未登录或凭证失效 |
| 403 | FORBIDDEN | 无访问权限 |
| 403 | NETWORK_DISABLED | 当前知识库禁止联网 |
| 404 | RESOURCE_NOT_FOUND | 资源不存在 |
| 409 | DUPLICATE_RESOURCE | 重复资源 |
| 409 | INDEX_VERSION_CONFLICT | 索引版本冲突 |
| 413 | PAYLOAD_TOO_LARGE | 文件/结果过大 |
| 422 | KNOWLEDGE_INSUFFICIENT | 仅用于需要显式失败的内部场景；正常问答应返回答案中的不足提示 |
| 429 | RATE_LIMITED | 并发或调用预算超限 |
| 500 | INTERNAL_ERROR | 服务内部错误 |
| 502 | MODEL_CALL_FAILED | 模型调用失败 |
| 502 | MCP_CALL_FAILED | MCP 调用失败 |
| 504 | UPSTREAM_TIMEOUT | 模型/MCP/工具超时 |

---

# 2. 核心枚举

## 2.1 文档来源

```text
manual
file
url
```

## 2.2 文档知识处理状态

```text
pending
parsing
cleaning
chunking
embedding
keyword_indexing
succeeded
failed
```

## 2.3 导入任务状态

```text
pending
running
succeeded
failed
skipped
```

## 2.4 索引版本状态

```text
processing
active
inactive
failed
```

## 2.5 知识充分状态

```text
sufficient
insufficient
ambiguous
```

## 2.6 Agent 执行模式

```text
react
plan_execute
```

用户不能通过问答接口指定该字段，由 Router Agent 决定。

## 2.7 AgentRun 状态

```text
queued
running
completed
failed
cancelled
```

## 2.8 Plan 状态

```text
pending
executing
replanning
reviewing
completed
failed
cancelled
```

## 2.9 PlanStep 状态

```text
pending
running
completed
failed
skipped
cancelled
```

## 2.10 Memory 类型

```text
preference
project
decision
goal
fact
progress
```

## 2.11 Memory 作用域

```text
user
knowledge_base
```

## 2.12 Memory 状态

```text
active
inactive
deleted
```

## 2.13 MCP / Tool 调用状态

```text
running
succeeded
failed
timeout
cancelled
```

---

# 3. API 总览

## 3.1 用户与认证

| Method | Path | 说明 |
|---|---|---|
| POST | `/auth/login` | 用户登录 |
| POST | `/auth/logout` | 退出当前登录状态 |
| GET | `/users/me` | 获取当前用户 |
| PATCH | `/users/me` | 修改基础信息 |
| PATCH | `/users/me/password` | 修改密码 |

## 3.2 知识库

| Method | Path | 说明 |
|---|---|---|
| POST | `/knowledge-bases` | 创建知识库 |
| GET | `/knowledge-bases` | 知识库列表 |
| GET | `/knowledge-bases/{kb_id}` | 知识库详情 |
| PATCH | `/knowledge-bases/{kb_id}` | 修改知识库 |
| DELETE | `/knowledge-bases/{kb_id}` | 删除知识库 |
| GET | `/knowledge-bases/{kb_id}/search-config` | 获取搜索配置 |
| PUT | `/knowledge-bases/{kb_id}/search-config` | 更新搜索配置 |
| GET | `/knowledge-bases/{kb_id}/agent-config` | 获取 Agent 配置 |
| PUT | `/knowledge-bases/{kb_id}/agent-config` | 更新 Agent 配置 |

## 3.3 文档目录

| Method | Path | 说明 |
|---|---|---|
| GET | `/knowledge-bases/{kb_id}/directories/tree` | 目录树 |
| POST | `/knowledge-bases/{kb_id}/directories` | 创建目录 |
| PATCH | `/directories/{directory_id}` | 修改名称/父目录/排序 |
| DELETE | `/directories/{directory_id}` | 删除目录 |

## 3.4 文档与导入

| Method | Path | 说明 |
|---|---|---|
| POST | `/knowledge-bases/{kb_id}/documents` | 手工创建只读知识文档 |
| GET | `/knowledge-bases/{kb_id}/documents` | 文档列表 |
| GET | `/documents/{document_id}` | 文档详情 |
| DELETE | `/documents/{document_id}` | 删除文档 |
| GET | `/documents/{document_id}/processing` | 处理状态 |
| POST | `/documents/{document_id}/retry-processing` | 处理失败整体重试 |
| POST | `/documents/{document_id}/reindex` | 主动重新索引 |
| GET | `/documents/{document_id}/index-versions` | 索引版本 |
| POST | `/knowledge-bases/{kb_id}/imports/files` | 文件导入 |
| POST | `/knowledge-bases/{kb_id}/imports/url` | URL 导入 |
| GET | `/knowledge-bases/{kb_id}/import-tasks` | 导入任务列表 |
| GET | `/import-tasks/{task_id}` | 导入任务详情 |
| POST | `/import-tasks/{task_id}/retry` | 重试导入任务 |

## 3.5 搜索

| Method | Path | 说明 |
|---|---|---|
| POST | `/knowledge-bases/{kb_id}/search` | 用户搜索 |
| POST | `/knowledge-bases/{kb_id}/search/test` | 检索测试 |

## 3.6 模型配置

| Method | Path | 说明 |
|---|---|---|
| POST | `/model-configs` | 新建模型配置 |
| GET | `/model-configs` | 模型配置列表 |
| GET | `/model-configs/{model_config_id}` | 模型配置详情 |
| PATCH | `/model-configs/{model_config_id}` | 修改模型配置 |
| DELETE | `/model-configs/{model_config_id}` | 删除模型配置 |

P0 不提供模型连通性测试接口。

## 3.7 会话与统一智能问答

| Method | Path | 说明 |
|---|---|---|
| POST | `/knowledge-bases/{kb_id}/conversations` | 创建会话 |
| GET | `/knowledge-bases/{kb_id}/conversations` | 会话列表 |
| GET | `/conversations/{conversation_id}` | 会话详情 |
| GET | `/conversations/{conversation_id}/messages` | 消息列表 |
| DELETE | `/conversations/{conversation_id}` | 删除/归档会话 |
| POST | `/conversations/{conversation_id}/questions` | 提交问题并创建 AgentRun |
| GET | `/agent-runs/{run_id}/events` | SSE 订阅 AgentRun |
| POST | `/agent-runs/{run_id}/cancel` | 停止任务 |
| POST | `/agent-runs/{run_id}/retry` | 失败后整体重新执行 |

## 3.8 Agent 运行记录

| Method | Path | 说明 |
|---|---|---|
| GET | `/agent-runs` | 运行记录列表 |
| GET | `/agent-runs/{run_id}` | 运行详情 |
| GET | `/agent-runs/{run_id}/router-decision` | Router 结果（从 AgentRun 结构化字段读取） |
| GET | `/agent-runs/{run_id}/plans` | Plan 版本列表 |
| GET | `/agent-runs/{run_id}/rounds` | ReAct 轮次（从 AgentRun execution_trace 读取） |
| GET | `/agent-runs/{run_id}/tool-calls` | 工具调用 |
| GET | `/agent-runs/{run_id}/citations` | 引用（从 Assistant Message citations 读取） |
| GET | `/agent-runs/{run_id}/memories` | 本次运行召回的 Memory 摘要 |

## 3.9 长期记忆

| Method | Path | 说明 |
|---|---|---|
| GET | `/memories` | Memory 列表 |
| GET | `/memories/{memory_id}` | Memory 详情 |
| PATCH | `/memories/{memory_id}/status` | 启用/停用 |
| DELETE | `/memories/{memory_id}` | 删除 Memory |

P0 不提供 Memory 手工新增和手工修改内容接口，Memory 默认由 MemoryExtractor / MemoryManager 产生。

## 3.10 MCP

| Method | Path | 说明 |
|---|---|---|
| POST | `/mcp/servers` | 新增 MCP Server |
| GET | `/mcp/servers` | Server 列表 |
| GET | `/mcp/servers/{server_id}` | Server 详情 |
| PATCH | `/mcp/servers/{server_id}` | 修改 Server |
| DELETE | `/mcp/servers/{server_id}` | 删除 Server |
| POST | `/mcp/servers/{server_id}/test` | 连接/认证测试 |
| POST | `/mcp/servers/{server_id}/discover` | 工具发现 |
| GET | `/mcp/servers/{server_id}/tools` | 工具列表 |
| GET | `/mcp/tools/{tool_id}` | 工具详情与 Schema |
| PATCH | `/mcp/tools/{tool_id}/enabled` | Server 级启停 |
| PUT | `/knowledge-bases/{kb_id}/agent-config/mcp-tools/{tool_id}` | 知识库 Agent 工具授权 |
| DELETE | `/knowledge-bases/{kb_id}/agent-config/mcp-tools/{tool_id}` | 取消知识库 Agent 工具授权 |
| GET | `/tool-calls?tool_type=mcp` | MCP 调用记录（统一 ToolCall） |

---

# 4. 用户与认证接口

## 4.1 登录

### `POST /auth/login`

请求：

```json
{
  "account": "admin@example.com",
  "password": "******"
}
```

响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "access_token": "xxx",
    "token_type": "Bearer",
    "expires_in": 7200,
    "user": {
      "id": "uuid",
      "username": "admin",
      "nickname": "admin",
      "email": "admin@example.com",
      "avatar_url": null
    }
  },
  "request_id": "..."
}
```

## 4.2 退出登录

### `POST /auth/logout`

当前凭证立即失效。若使用 JWT，可通过 Redis 黑名单或 token version 实现。

## 4.3 当前用户

### `GET /users/me`

响应字段：

```json
{
  "id": "uuid",
  "username": "admin",
  "nickname": "东风",
  "email": "admin@example.com",
  "avatar_url": "https://...",
  "bio": "..."
}
```

## 4.4 修改当前用户

### `PATCH /users/me`

允许修改：

```json
{
  "nickname": "新昵称",
  "avatar_url": "https://...",
  "bio": "简介",
  "email": "new@example.com"
}
```

## 4.5 修改密码

### `PATCH /users/me/password`

```json
{
  "old_password": "******",
  "new_password": "******"
}
```

---

# 5. 知识库接口

## 5.1 创建知识库

### `POST /knowledge-bases`

```json
{
  "name": "Go 学习知识库",
  "description": "Go、Eino、Agent、RAG 学习资料",
  "icon": "book",
  "default_language": "zh-CN",
  "qa_enabled": true,
  "agent_enabled": true,
  "network_enabled": false,
  "default_chat_model_id": "uuid",
  "default_embedding_model_id": "uuid",
  "default_reranker_model_id": "uuid"
}
```

响应：

```json
{
  "id": "uuid",
  "name": "Go 学习知识库",
  "network_enabled": false,
  "created_at": "2026-07-31T14:30:00+09:00"
}
```

创建成功后后端必须在同一业务事务中初始化：

1. 默认根目录；
2. 默认搜索配置；
3. 默认 Agent 配置。

## 5.2 知识库列表

### `GET /knowledge-bases?page=1&page_size=20&keyword=go&sort=updated_at_desc`

单项：

```json
{
  "id": "uuid",
  "name": "Go 学习知识库",
  "icon": "book",
  "description": "...",
  "document_count": 25,
  "agent_enabled": true,
  "network_enabled": false,
  "updated_at": "...",
  "created_at": "..."
}
```

## 5.3 修改知识库

### `PATCH /knowledge-bases/{kb_id}`

允许修改基础配置：

```json
{
  "name": "Go + Agent 知识库",
  "description": "...",
  "icon": "robot",
  "default_language": "zh-CN",
  "qa_enabled": true,
  "agent_enabled": true,
  "network_enabled": true,
  "default_chat_model_id": "uuid",
  "default_embedding_model_id": "uuid",
  "default_reranker_model_id": "uuid"
}
```

## 5.4 删除知识库

### `DELETE /knowledge-bases/{kb_id}`

前端负责二次确认。后端执行逻辑删除，并立即使：

- 文档；
- chunk；
- vector；
- Agent 工具访问；

全部失效。MinIO 原文件由后台任务清理。

---

# 6. 搜索配置与 Agent 配置

## 6.1 搜索配置

### `GET /knowledge-bases/{kb_id}/search-config`

```json
{
  "keyword_top_k": 30,
  "vector_top_k": 30,
  "rrf_k": 60,
  "rrf_top_k": 20,
  "reranker_top_k": 8,
  "reranker_threshold": 0.35,
  "minimum_effective_results": 1,
  "reranker_model_id": "uuid"
}
```

### `PUT /knowledge-bases/{kb_id}/search-config`

可修改同一组字段。

## 6.2 Agent 配置

### `GET /knowledge-bases/{kb_id}/agent-config`

```json
{
  "id": "uuid",
  "name": "Default Agent",
  "system_prompt": "...",
  "chat_model_id": "uuid",
  "max_react_rounds": 8,
  "max_plan_steps": 5,
  "max_replans": 1,
  "reviewer_runs": 1,
  "max_tool_calls": 10,
  "max_document_read_tokens": 6000,
  "max_tool_result_bytes": 1048576,
  "max_run_seconds": 300,
  "network_enabled": false,
  "memory_enabled": true,
  "memory_top_k": 8,
  "show_execution_status": true,
  "status": "active"
}
```

### `PUT /knowledge-bases/{kb_id}/agent-config`

约束：

- `max_plan_steps <= 5`；
- `max_replans <= 1`；
- `reviewer_runs <= 1`；
- P0 建议 `max_react_rounds <= 8`；
- P0 建议 `max_tool_calls <= 10`；
- 不存在 `default_execution_mode` 字段。

---

# 7. 文档目录接口

## 7.1 获取目录树

### `GET /knowledge-bases/{kb_id}/directories/tree`

```json
[
  {
    "id": "uuid",
    "name": "默认目录",
    "parent_id": null,
    "depth": 1,
    "sort_order": 0,
    "children": []
  }
]
```

## 7.2 创建目录

### `POST /knowledge-bases/{kb_id}/directories`

```json
{
  "name": "Eino",
  "parent_id": "uuid-or-null",
  "sort_order": 10
}
```

后端校验：

- 目录属于当前用户和当前知识库；
- 最大深度 5。

## 7.3 修改/移动/排序目录

### `PATCH /directories/{directory_id}`

```json
{
  "name": "Agent",
  "parent_id": "uuid-or-null",
  "sort_order": 20
}
```

禁止将目录移动到自身或自己的子孙节点下。

## 7.4 删除目录

### `DELETE /directories/{directory_id}`

建议 P0 规则：目录非空时返回 `409 DIRECTORY_NOT_EMPTY`，避免隐式批量删除文档。

---

# 8. 文档与导入接口

## 8.1 手工创建只读知识文档

### `POST /knowledge-bases/{kb_id}/documents`

```json
{
  "title": "Eino ReAct 笔记",
  "content": "# ReAct\n...",
  "directory_id": "uuid",
  "source_type": "manual",
  "source_url": null
}
```

响应：

```json
{
  "id": "uuid",
  "processing_status": "pending",
  "content_version": 1,
  "created_at": "..."
}
```

创建后异步进入：

```text
pending
→ parsing
→ cleaning
→ chunking
→ embedding
→ keyword_indexing
→ succeeded / failed
```

## 8.2 文档列表

### `GET /knowledge-bases/{kb_id}/documents`

查询参数：

- `page`
- `page_size`
- `keyword`
- `directory_id`
- `processing_status`
- `source_type`

## 8.3 文档详情

### `GET /documents/{document_id}`

```json
{
  "id": "uuid",
  "knowledge_base_id": "uuid",
  "directory_id": "uuid",
  "title": "Eino ReAct 笔记",
  "content": "...",
  "source_type": "manual",
  "source_url": null,
  "processing_status": "succeeded",
  "content_version": 1,
  "active_index_version": 1,
  "created_at": "...",
  "updated_at": "..."
}
```

## 8.4 文件导入

### `POST /knowledge-bases/{kb_id}/imports/files`

`Content-Type: multipart/form-data`

字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| files | file[] | 是 | md/txt/pdf/docx |
| directory_id | UUID | 否 | 目标目录 |
| duplicate_policy | string | 否 | `create_new` / `skip`，默认 `skip` |

响应：

```json
{
  "tasks": [
    {
      "task_id": "uuid",
      "file_name": "eino.pdf",
      "status": "pending"
    }
  ]
}
```

校验：

- 文件扩展名；
- MIME；
- 文件大小；
- 单次数量；
- 文件哈希；
- 用户存储限制。

## 8.5 URL 导入

### `POST /knowledge-bases/{kb_id}/imports/url`

```json
{
  "url": "https://example.com/article",
  "directory_id": "uuid",
  "duplicate_policy": "skip"
}
```

服务端必须：

- 只允许 HTTP / HTTPS；
- 拒绝 localhost、回环、私网、云元数据地址、file://；
- 每次重定向重新校验；
- 限制重定向、响应体大小、超时；
- 保存最终来源 URL。

## 8.6 导入任务详情

### `GET /import-tasks/{task_id}`

```json
{
  "id": "uuid",
  "source_type": "file",
  "file_name": "eino.pdf",
  "file_size": 123456,
  "mime_type": "application/pdf",
  "source_url": null,
  "status": "running",
  "current_step": "embedding",
  "failure_reason": null,
  "document_id": "uuid",
  "created_at": "...",
  "completed_at": null
}
```

## 8.7 处理状态

### `GET /documents/{document_id}/processing`

```json
{
  "document_id": "uuid",
  "processing_status": "embedding",
  "current_index_version": 2,
  "active_index_version": 1,
  "failure_step": null,
  "failure_reason": null
}
```

新索引尚未完成时，旧 active 版本继续提供检索。

## 8.8 处理失败重试

### `POST /documents/{document_id}/retry-processing`

仅允许 `processing_status=failed`。

## 8.9 重新索引

### `POST /documents/{document_id}/reindex`

```json
{
  "reason": "embedding_model_changed"
}
```

响应：

```json
{
  "document_id": "uuid",
  "new_index_version": 3,
  "active_index_version": 2,
  "status": "processing"
}
```

后端为新版本生成 Chunk/Vector；全部成功后在事务中更新 `documents.active_index_version=3`。构建失败时继续使用原 active 版本。

---

# 9. 搜索接口

## 9.1 用户搜索

### `POST /knowledge-bases/{kb_id}/search`

请求：

```json
{
  "query": "Eino ReAct 如何调用工具",
  "mode": "hybrid",
  "document_ids": [],
  "top_k": 8
}
```

`mode`：

```text
keyword
semantic
hybrid
```

响应：

```json
{
  "query": "Eino ReAct 如何调用工具",
  "mode": "hybrid",
  "knowledge_status": "sufficient",
  "results": [
    {
      "document_id": "uuid",
      "document_title": "Eino Agent",
      "chunk_id": "uuid",
      "content": "...",
      "directory_id": "uuid",
      "source_location": {
        "page": 10,
        "section": "ToolsNode"
      },
      "keyword_score": 0.71,
      "vector_score": 0.84,
      "rrf_rank": 1,
      "reranker_score": 0.91,
      "final_rank": 1,
      "document_updated_at": "..."
    }
  ],
  "elapsed_ms": 210
}
```

## 9.2 检索测试

### `POST /knowledge-bases/{kb_id}/search/test`

请求与 `/search` 基本相同，响应额外返回检索阶段信息：

```json
{
  "query": "...",
  "keyword_results": [],
  "vector_results": [],
  "rrf_results": [],
  "reranked_results": [],
  "final_results": [],
  "timing": {
    "keyword_ms": 12,
    "vector_ms": 35,
    "rrf_ms": 2,
    "reranker_ms": 120,
    "total_ms": 169
  }
}
```

---

# 10. 模型配置接口

## 10.1 新建模型配置

### `POST /model-configs`

```json
{
  "model_type": "chat",
  "provider": "openai_compatible",
  "name": "mimo-v2.5",
  "base_url": "https://api.example.com/v1",
  "api_key": "sk-...",
  "timeout_seconds": 60,
  "retry_times": 2,
  "max_tokens": 8192,
  "temperature": 0.2,
  "vector_dimension": null,
  "supports_tool_calling": true,
  "supports_streaming": true,
  "is_default": true,
  "enabled": true
}
```

`model_type`：

```text
chat
embedding
reranker
```

API Key 响应中只能脱敏：

```json
{
  "api_key_masked": "sk-****abcd"
}
```

## 10.2 修改模型配置

### `PATCH /model-configs/{model_config_id}`

当默认 Embedding 模型或向量维度变化时，不应直接修改已有 active vector 的语义；应触发文档重新索引和 Memory 向量重建流程。

---

# 11. 会话与统一智能问答

## 11.1 创建会话

### `POST /knowledge-bases/{kb_id}/conversations`

```json
{
  "title": "Eino Agent 讨论"
}
```

响应：

```json
{
  "id": "uuid",
  "knowledge_base_id": "uuid",
  "title": "Eino Agent 讨论",
  "created_at": "..."
}
```

## 11.2 消息列表

### `GET /conversations/{conversation_id}/messages?page=1&page_size=50`

单项：

```json
{
  "id": "uuid",
  "role": "user",
  "content": "ReAct 和 RAG 是什么关系？",
  "agent_run_id": null,
  "created_at": "..."
}
```

Assistant 消息可以带：

```json
{
  "id": "uuid",
  "role": "assistant",
  "content": "...",
  "agent_run_id": "uuid",
  "status": "completed",
  "citations": []
}
```

## 11.3 提交问题

### `POST /conversations/{conversation_id}/questions`

请求：

```json
{
  "query": "结合我之前的项目决策，说明 ReAct 和 RAG 应该怎么整合"
}
```

**禁止出现：**

```json
{
  "execution_mode": "react"
}
```

执行模式必须由 Router Agent 根据：

- 当前问题；
- 当前会话上下文；
- 召回长期记忆；
- 当前知识库；
- 联网权限；

自动决定。

响应：`202 Accepted`

```json
{
  "run_id": "uuid",
  "user_message_id": "uuid",
  "status": "queued",
  "events_url": "/api/v1/agent-runs/uuid/events"
}
```

内部链路：

```text
保存用户消息
→ ConversationContextService
→ MemoryRetriever
→ Context Builder
→ Router Agent
→ 创建/更新 AgentRun
→ ReAct 或 Plan-Execute
→ ToolExecutor
→ 最终回答
→ 保存 AssistantMessage / Citation / Usage
→ 异步 MemoryExtractor
```

## 11.4 SSE 订阅

### `GET /agent-runs/{run_id}/events`

响应：

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

统一格式：

```text
event: router.selected
data: {"run_id":"...","sequence":3,"timestamp":"...","payload":{"execution_mode":"react","reason_summary":"任务步骤不固定，适合动态检索"}}
```

SSE 公共字段：

```json
{
  "run_id": "uuid",
  "sequence": 1,
  "timestamp": "...",
  "payload": {}
}
```

必须支持的事件：

```text
run.started
run.completed
run.failed
run.cancelled
router.selected
memory.retrieved
plan.created
plan.replanned
step.started
step.completed
step.failed
agent.round.started
tool.call.started
tool.call.completed
tool.call.failed
answer.delta
citation.created
usage.updated
memory.updated
```

### 典型 ReAct 流

```text
run.started
memory.retrieved
router.selected
agent.round.started
tool.call.started
tool.call.completed
answer.delta
citation.created
usage.updated
run.completed
```

### 典型 Plan-Execute 流

```text
run.started
memory.retrieved
router.selected
plan.created
step.started
tool.call.started
tool.call.completed
step.completed
...
answer.delta
citation.created
run.completed
```

`answer.delta`：

```json
{
  "delta": "ReAct 与 RAG 在这里不是两个独立模式，"
}
```

`usage.updated`：

```json
{
  "input_tokens": 3200,
  "output_tokens": 880,
  "total_tokens": 4080
}
```

## 11.5 停止任务

### `POST /agent-runs/{run_id}/cancel`

响应：

```json
{
  "run_id": "uuid",
  "status": "cancelled"
}
```

后端需要：

- 发出 context cancel；
- 停止后续工具调用；
- 当前可中断上游应尽快取消；
- 记录 `run.cancelled`。

## 11.6 整体重试

### `POST /agent-runs/{run_id}/retry`

只支持失败任务整体重试，不支持 P0 任意节点恢复。

响应：

```json
{
  "new_run_id": "uuid",
  "retry_of_run_id": "old-uuid",
  "status": "queued"
}
```

---

# 12. Agent 运行详情

> **P0 轻量化存储说明**  
> 下列 API 展示能力保持不变，但不要求每类信息对应独立数据库表：
> - Router 结果来自 `agent_runs`；
> - ReAct Round 来自 `agent_runs.execution_trace`；
> - Reviewer 结果来自 `agent_runs`；
> - Replan 来自 `agent_plans` 的版本和 `replan_reason`；
> - Citation 来自 Assistant Message 的 `citations`；
> - MCP 与内部工具调用统一来自 `tool_calls`。


## 12.1 AgentRun 详情

### `GET /agent-runs/{run_id}`

```json
{
  "id": "uuid",
  "knowledge_base_id": "uuid",
  "conversation_id": "uuid",
  "query": "...",
  "execution_mode": "plan_execute",
  "knowledge_status": "sufficient",
  "status": "completed",
  "router_reason_summary": "任务包含多个依赖步骤",
  "memory_used_count": 4,
  "input_tokens": 5600,
  "output_tokens": 1450,
  "total_tokens": 7050,
  "duration_ms": 8200,
  "final_result": "...",
  "error_code": null,
  "error_message": null,
  "started_at": "...",
  "ended_at": "..."
}
```

## 12.2 Router 决策

### `GET /agent-runs/{run_id}/router-decision`

```json
{
  "execution_mode": "react",
  "reason_summary": "普通知识问答，可通过动态检索完成",
  "confidence": 0.91,
  "created_at": "..."
}
```

不得保存或返回完整隐藏思维链。

## 12.3 Plan 版本

### `GET /agent-runs/{run_id}/plans`

```json
[
  {
    "id": "uuid",
    "version": 1,
    "goal": "比较多个方案并给出结论",
    "status": "completed",
    "is_current": false,
    "steps": [
      {
        "id": "uuid",
        "step_no": 1,
        "title": "检索方案 A",
        "depends_on": [],
        "recommended_tool": "knowledge_search",
        "status": "completed",
        "output_summary": "..."
      }
    ]
  },
  {
    "id": "uuid",
    "version": 2,
    "is_current": true,
    "status": "completed",
    "replan_reason": "原资料不足"
  }
]
```

P0 最多 version 2。

## 12.4 ReAct 轮次

### `GET /agent-runs/{run_id}/rounds`

```json
[
  {
    "round_no": 1,
    "status": "completed",
    "action_summary": "检索当前知识库",
    "tool_name": "knowledge_search",
    "duration_ms": 320,
    "token_count": 780
  }
]
```

只返回可观察轨迹摘要，不返回模型完整内部推理。

## 12.5 工具调用

### `GET /agent-runs/{run_id}/tool-calls`

```json
[
  {
    "tool_call_id": "uuid",
    "tool_name": "knowledge_search",
    "tool_type": "internal",
    "mcp_server_id": null,
    "mcp_tool_id": null,
    "status": "succeeded",
    "input_summary": "搜索 Eino ReAct 工具调用",
    "output_summary": "返回 6 个片段",
    "duration_ms": 182,
    "is_truncated": false,
    "created_at": "..."
  }
]
```

## 12.6 引用

### `GET /agent-runs/{run_id}/citations`

知识库引用：

```json
{
  "source_type": "knowledge_base",
  "document_id": "uuid",
  "document_title": "Eino.md",
  "chunk_id": "uuid",
  "quoted_text": "...",
  "knowledge_base_id": "uuid",
  "source_location": {
    "section": "ToolsNode"
  },
  "document_updated_at": "..."
}
```

网络引用：

```json
{
  "source_type": "network",
  "title": "Eino Documentation",
  "url": "https://...",
  "site_name": "CloudWeGo",
  "published_at": null,
  "fetched_at": "..."
}
```

Memory 不作为知识事实 Citation。

---

# 13. 长期记忆接口

## 13.1 Memory 列表

### `GET /memories`

查询参数：

| 参数 | 说明 |
|---|---|
| memory_type | preference/project/decision/goal/fact/progress |
| scope_type | user/knowledge_base |
| scope_id | knowledge_base 作用域时可用 |
| status | active/inactive/deleted |
| page/page_size | 分页 |

单项：

```json
{
  "id": "uuid",
  "memory_type": "project",
  "scope_type": "knowledge_base",
  "scope_id": "uuid",
  "content": "该项目统一使用 Router 自动选择 ReAct 或 Plan-Execute",
  "summary": "问答执行模式由 Router 自动路由",
  "importance": 0.86,
  "source_conversation_id": "uuid",
  "source_message_id": "uuid",
  "status": "active",
  "created_at": "...",
  "updated_at": "...",
  "last_accessed_at": "..."
}
```

## 13.2 修改 Memory 状态

### `PATCH /memories/{memory_id}/status`

```json
{
  "status": "inactive"
}
```

允许：

```text
active
inactive
```

不通过该接口恢复 `deleted`。

## 13.3 删除 Memory

### `DELETE /memories/{memory_id}`

删除后必须立即从 MemoryRetriever 结果中排除。

## 13.4 Memory 操作历史
详细的 Memory create / merge / update / deactivate / delete 操作历史属于 P1，P0 不提供独立 MemoryOperation 查询接口。

P0 的 Memory 页面直接展示当前 Memory 的：
- 内容；
- 类型；
- 作用域；
- 来源会话和消息；
- 创建时间；
- 更新时间；
- 最近使用时间；
- 当前状态。

MemoryExtractor / MemoryManager 的必要执行结果可以作为 AgentRun 运行事件摘要记录，但不单独建设 MemoryOperation 核心实体。

# 14. MCP 接口

## 14.1 新增 MCP Server

### `POST /mcp/servers`

```json
{
  "name": "Web Search MCP",
  "description": "第三方联网搜索",
  "transport": "streamable_http",
  "url": "https://mcp.example.com/mcp",
  "headers": {
    "X-Client": "knowledge-agent"
  },
  "auth": {
    "type": "bearer",
    "token": "secret"
  },
  "connect_timeout_ms": 5000,
  "call_timeout_ms": 15000,
  "max_response_bytes": 1048576,
  "enabled": true
}
```

P0 `transport` 只允许：

```text
streamable_http
```

敏感 header / auth 必须加密保存，响应只返回脱敏内容。

## 14.2 连接测试

### `POST /mcp/servers/{server_id}/test`

```json
{
  "status": "available",
  "auth_ok": true,
  "latency_ms": 216,
  "error": null
}
```

## 14.3 工具发现

### `POST /mcp/servers/{server_id}/discover`

后端执行：

1. 连接；
2. 获取工具列表；
3. 获取 description；
4. 获取 input schema；
5. 计算 schema hash；
6. upsert 工具元数据；
7. 若 Schema 改变，重新校验启用状态。

响应：

```json
{
  "server_id": "uuid",
  "discovered_at": "...",
  "tools": [
    {
      "id": "uuid",
      "name": "web_search",
      "description": "Search the web",
      "schema_hash": "sha256:...",
      "read_only": true,
      "enabled": false
    }
  ]
}
```

## 14.4 工具详情

### `GET /mcp/tools/{tool_id}`

```json
{
  "id": "uuid",
  "server_id": "uuid",
  "name": "web_search",
  "description": "...",
  "input_schema": {
    "type": "object",
    "properties": {
      "query": {"type": "string"}
    },
    "required": ["query"]
  },
  "schema_hash": "...",
  "read_only": true,
  "enabled": true,
  "discovered_at": "...",
  "last_checked_at": "..."
}
```

## 14.5 Server 级启停工具

### `PATCH /mcp/tools/{tool_id}/enabled`

```json
{
  "enabled": true
}
```

规则：

- `read_only=false`：返回 `WRITE_MCP_TOOL_FORBIDDEN`；
- Server disabled：不得启用；
- Schema 变化且未重新校验：不得直接继续启用。

## 14.6 知识库 Agent 授权工具

### `PUT /knowledge-bases/{kb_id}/agent-config/mcp-tools/{tool_id}`

```json
{
  "enabled": true
}
```

后端校验：

- 工具只读；
- MCP Server 有效；
- 工具 server 级 enabled；
- Agent 配置有效。

即使已授权，如果 `knowledge_base.network_enabled=false` 或 `agent_config.network_enabled=false`，运行时仍禁止调用联网工具。

---

# 15. Agent 内部工具契约

这些不是直接暴露给浏览器的开放 HTTP API，而是 Agent Core 内部统一 Tool 接口契约。

## 15.1 ToolContext

由后端注入：

```json
{
  "user_id": "uuid",
  "knowledge_base_id": "uuid",
  "agent_run_id": "uuid",
  "allowed_tool_names": [
    "knowledge_search",
    "document_read",
    "web_search"
  ],
  "network_enabled": false
}
```

模型不可见、不可覆盖 `user_id / knowledge_base_id / agent_run_id`。

## 15.2 ToolCall

```json
{
  "tool_call_id": "uuid",
  "tool_name": "knowledge_search",
  "arguments": {}
}
```

## 15.3 ToolResult

```json
{
  "tool_call_id": "uuid",
  "content": "文本形式结果",
  "data": {},
  "citations": [],
  "truncated": false,
  "failed": false,
  "error_code": null
}
```

## 15.4 KnowledgeSearchTool

模型可传：

```json
{
  "query": "Eino ReAct",
  "mode": "hybrid",
  "top_k": 8,
  "document_ids": []
}
```

模型不得传：

```text
user_id
knowledge_base_id
agent_run_id
```

输出：

```json
{
  "knowledge_status": "sufficient",
  "results": [
    {
      "document_id": "uuid",
      "document_title": "...",
      "chunk_id": "uuid",
      "content": "...",
      "keyword_rank": 3,
      "keyword_score": 0.61,
      "vector_rank": 1,
      "vector_score": 0.86,
      "rrf_rank": 1,
      "reranker_score": 0.93,
      "source_location": {}
    }
  ]
}
```

## 15.5 DocumentReadTool

模型可传：

```json
{
  "document_id": "uuid",
  "section": "ToolsNode",
  "cursor": null,
  "max_tokens": 5000
}
```

输出：

```json
{
  "document_id": "uuid",
  "document_title": "...",
  "content": "...",
  "source": "...",
  "updated_at": "...",
  "next_cursor": "opaque-cursor",
  "truncated": true,
  "citation": {
    "source_location": {}
  }
}
```

必须由 ToolExecutor 校验文档属于当前 User + KnowledgeBase。

---

# 16. 关键运行规则

## 16.1 Router

Router 输入统一 `AgentContext`：

```json
{
  "user_query": "...",
  "conversation_context": [],
  "retrieved_memories": [],
  "knowledge_base_id": "uuid",
  "network_enabled": false
}
```

结构化输出：

```json
{
  "execution_mode": "react",
  "reason_summary": "执行步骤无法预先固定",
  "confidence": 0.88
}
```

非法输出兜底：

```text
react
```

Router 不调用 KnowledgeSearchTool、DocumentReadTool、MCP。

## 16.2 ReAct-RAG

- 最大轮数由 AgentConfig 控制；
- 每次 ToolCall 都经过 ToolExecutor；
- RAG 是 KnowledgeSearchTool 的底层能力，不存在独立普通 RAG 用户模式；
- 检索不足时可再次搜索、读文档或在允许时联网；
- 达预算、超时、取消后停止。

## 16.3 Plan-Execute

- Plan 最多 5 步；
- 步骤显式依赖；
- 最多重新规划 1 次；
- 只修改未执行的剩余步骤；
- Reviewer 最多 1 次；
- 不进行无限反思循环。

## 16.4 Memory

问答前：

```text
ConversationContext
→ MemoryRetriever
→ Context Builder
→ Router
```

问答后：

```text
MemoryExtractor（异步）
→ MemoryManager
→ create / merge / update / deactivate / ignore
```

Memory 服务异常时，核心问答降级为仅使用当前会话上下文。

---

# 17. 安全约束

所有接口实现必须遵守：

1. 所有用户资源按 `user_id` 强隔离；
2. KnowledgeBase、Document、Memory、AgentRun 均不能仅凭资源 ID 查询，必须同时校验 owner；
3. 模型不能控制身份字段；
4. P0 MCP 只允许 ReadOnly Tool；
5. API Key / MCP 凭证加密保存；
6. API 响应和日志只显示脱敏值；
7. URL 导入、网页读取、MCP 网络请求执行 SSRF 防护；
8. 禁止本机、回环、内网、云元数据地址；
9. 重定向后重新验证目标；
10. 工具调用有次数、时长、响应体大小限制；
11. 文档删除/知识库删除后相关 chunk/vector 立即不可检索；
12. inactive/deleted Memory 立即不可召回；
13. 不保存或返回模型完整内部思维链；
14. Citation 必须来自真实文档片段或真实网络 URL；
15. 长期记忆不能替代事实引用。

---

# 18. 前后端联调建议顺序

1. Auth；
2. KnowledgeBase；
3. Directory；
4. Document + ImportTask；
5. SearchConfig + ModelConfig；
6. 文档处理状态；
7. Search / SearchTest；
8. Conversation / Message；
9. Question → AgentRun；
10. SSE；
11. RouterDecision；
12. ReAct ToolCall；
13. Plan / PlanStep；
14. Citation；
15. Memory；
16. MCP Server / Discover / Tool Grant；
17. Cancel / Retry；
18. Agent 运行详情页。

---

# 19. P0 验收接口链路

最小完整验收：

```text
POST /auth/login
→ POST /knowledge-bases
→ POST /knowledge-bases/{kb}/imports/files
→ GET /import-tasks/{id}
→ POST /knowledge-bases/{kb}/search/test
→ POST /knowledge-bases/{kb}/conversations
→ POST /conversations/{id}/questions
→ GET /agent-runs/{run}/events
→ GET /agent-runs/{run}
→ GET /agent-runs/{run}/citations
→ GET /memories
```

联网能力验收：

```text
POST /mcp/servers
→ POST /mcp/servers/{id}/test
→ POST /mcp/servers/{id}/discover
→ PATCH /mcp/tools/{id}/enabled
→ PUT /knowledge-bases/{kb}/agent-config/mcp-tools/{tool}
→ 开启 knowledge_base.network_enabled
→ 发起需要最新信息的问题
→ GET /agent-runs/{run}/tool-calls
→ GET /agent-runs/{run}/citations
```


---

# 20. 与 20 表数据库设计的对应关系

API 资源与数据库并非一一对应。P0 保留业务 API，但通过合并存储降低表数量：

| API / 功能 | P0 数据来源 |
|---|---|
| Router Decision | `agent_runs` |
| ReAct Rounds | `agent_runs.execution_trace` |
| Reviewer Result | `agent_runs` |
| Replan | `agent_plans` |
| Citation | `messages.citations` |
| KnowledgeSearch / DocumentRead ToolCall | `tool_calls` |
| MCP ToolCall | `tool_calls` |
| Memory 当前状态 | `memories` |
| Memory 详细操作历史 | P1 |
| 文档原始文件信息 | `documents` / `import_tasks` |
| Active Index Version | `documents.active_index_version` |

因此“API 有一个独立查询入口”不代表必须为它单独创建数据库表。
