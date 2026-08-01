# Memora 前端文档

## 概述

Memora P0 前端是一个独立的 Vue 3 SPA，通过 REST API 和 SSE 与 Go 后端通信。

## 架构

```text
Route Page
→ Feature Component / Composable
→ Vue Query / Pinia / Runtime Store
→ API Client / SSE Client
→ Gin REST API / SSE
```

- 页面只负责编排
- Feature Composable 封装用例和数据组合
- API Client 处理协议、认证和错误映射
- DTO 与后端 JSON 保持 `snake_case`

## 认证

- Token 存储在 `sessionStorage`
- 关闭标签页后清除
- 401 响应触发自动登出并跳转登录页
- 登录后返回原页面

## SSE 流式回答

1. 提交问题获得 `run_id` 和 `events_url`
2. 建立带 Bearer Token 的 SSE Fetch
3. 按 `sequence` 去重处理事件
4. `answer.delta` 拼接流式回答
5. 终态事件关闭连接并刷新持久数据

## 功能开关

| 环境变量 | 说明 | 启用条件 |
|---|---|---|
| `VITE_SSE_RESUME_ENABLED` | SSE 断线续传 | 后端支持 `after_sequence` |
| `VITE_DOCUMENT_SCOPE_ENABLED` | 文档作用域问答 | 后端支持 `document_id` 字段 |

## 安全约束

- 所有 Markdown/HTML 经 DOMPurify 清洗
- 外链添加 `target="_blank" rel="noopener noreferrer"`
- API Key/MCP Token 只显示脱敏状态
- 前端不发送 `user_id`、`agent_run_id`、`allowed_tool_names`
- 模型完整思维链不存储到前端

## 部署

生产构建由 Nginx 提供，同域代理：

```text
/          → Vue SPA
/api/v1    → memora-server
/health    → memora-server
```

SSE 路由关闭代理缓冲，延长读取超时。

## API 端点参考

详见 `AI智能知识库_API接口文档_P0_轻量化版.md`。

### 联调顺序

1. Auth
2. KnowledgeBase
3. Directory
4. Document + ImportTask
5. SearchConfig + ModelConfig
6. 文档处理状态
7. Search / SearchTest
8. Conversation / Message
9. Question → AgentRun
10. SSE
11. RouterDecision
12. ReAct ToolCall
13. Plan / PlanStep
14. Citation
15. Memory
16. MCP Server / Discover / Tool Grant
17. Cancel / Retry
18. Agent 运行详情页

### 验收链路

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
