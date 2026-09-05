# Memora React 管理端

## 概述

Memora P0 管理端位于 `web/admin`，使用 React 19、TypeScript、Vite、MUI、TanStack Query 与 Redux Toolkit。它来自 PandaWiki 管理端的视觉基础，但运行时协议、路由、DTO 与品牌均为 Memora；不包含 Next.js 公共门户、Vue、在线协作编辑、发布和许可证功能。

调用链固定为：

```text
Route / Page → Feature API / Query → Memora API Client or authenticated SSE → /api/v1
```

服务端数据由 TanStack Query 管理。Redux 只保存认证会话镜像和聊天布局偏好，DTO 保持后端 `snake_case`。

## Workspace 与命令

```text
web/
├─ admin/                 React/Vite 管理端
├─ packages/icons/        @memora/icons
├─ packages/themes/       @memora/themes
├─ packages/ui/           @memora/ui
├─ Dockerfile
└─ nginx.conf
```

需要 Node.js 24 与 pnpm 10.12.1：

```sh
cd web
corepack enable
pnpm install --frozen-lockfile
pnpm dev
pnpm --filter memora-admin test
pnpm lint
pnpm typecheck
pnpm build
```

开发服务器把 `/api/v1` 转发到 `VITE_API_PROXY_TARGET`，默认 `http://localhost:8080`。环境变量样例见 `web/.env.example`。

## 路由

| 路由 | 页面 |
|---|---|
| `/login` | 登录 |
| `/knowledge-bases` | 知识库 |
| `/kb/:kbId/docs/:documentId?` | 目录树与只读文档 |
| `/chat/:kbId/:conversationId?` | 三栏智能问答 |
| `/runs/:runId?` | Agent 运行记录/详情 |
| `/memories` | 长期记忆 |
| `/mcp` | MCP 服务与只读工具 |
| `/kb/:kbId/search-test` | 检索测试 |
| `/kb/:kbId/settings` | 知识库设置 |
| `/settings/profile` | 当前用户资料与密码 |
| `/settings/models` | 模型配置 |

## 当前能力状态

| 能力 | 状态 | 行为 |
|---|---|---|
| Auth、Current User | `available` | 使用当前 Go API |
| Knowledge Base、Document | `available` | 知识库 CRUD、搜索配置、目录树、文档列表/只读正文、文件导入、导入任务轮询与重试、处理重试/重新索引 |
| Conversation、Agent Run | `backend_pending` | 三栏工作区与事件归约器已就绪，零请求/零 SSE |
| Memory、Search、Model | `backend_pending` | 强类型 API 边界，零请求，写操作禁用 |
| MCP | `available` | 使用当前 Go API |

> 注：Search 的 `/knowledge-bases/{kb_id}/search/test` 与 URL 导入 `/imports/url` 属于任务包 07+ 范围，后端接口就绪前检索测试页保持 `backend_pending`、URL 导入保持禁用。

生产能力开关集中在 `web/admin/src/app/capabilities.ts`。只有后端路由、DTO 和错误场景都完成契约验证后才能从 `backend_pending` 改为 `available`。

## API、认证与错误

统一响应：

```json
{
  "code": "OK",
  "message": "success",
  "data": {},
  "details": null,
  "request_id": "uuid"
}
```

前端只解包 HTTP 成功且 `code === "OK"` 的响应；其他响应转换为 `AppError`。可见错误保留 `request_id` 供复制排障，非 JSON 上游错误不会把响应正文暴露给用户。

Token 只存于 `sessionStorage` 的 `memora.auth`，结构为 `{ access_token, expires_at, user }`。启动时拒绝过期或畸形数据；401 会清除会话并携带当前地址跳到登录页；退出请求即使网络失败也会清除本地会话。

## Agent SSE

提交问题返回 `run_id` 与 `events_url` 后，前端使用带 Bearer Token 的 `fetch` 流读取 SSE，不使用原生 `EventSource`。传输层支持拆分 UTF-8、多行 data、`after_sequence`、终态和取消。

Agent 事件归约器：

- 按 `sequence` 忽略重复与乱序事件；
- 失败/取消后保留已生成答案；
- 保存 Router、Plan、ReAct 轮次和工具调用的用户可见摘要；
- 不保存模型完整隐藏推理；
- `completed`、`failed`、`cancelled` 进入确定终态。

## 生产部署

`web/Dockerfile` 用冻结 lockfile 构建 `admin/dist`，运行时使用监听 8080 的非特权 Nginx。`web/nginx.conf` 提供：

- SPA fallback；
- 仅 `/assets/` 使用 immutable 缓存，`index.html` 禁止缓存；
- `/api/v1` 与健康检查反向代理；
- Agent SSE 路由关闭缓冲并把读取超时延长到 1 小时。

仓库 Compose 将管理端暴露在 `http://localhost:3000`。

## 后端能力激活清单

1. 对照当前 Go handler 与 API 文档确认路径、字段和枚举。
2. 使用本地真实服务验证成功、校验、401、404、409、限流与上游失败响应。
3. 保持 `backend_pending`，通过浏览器网络面板确认页面零请求。
4. 联调 `available` 状态并核对 Memora 信封和 DTO。
5. 本地真实服务完成手工路径后再修改生产 capability。
6. 运行 `pnpm lint && pnpm typecheck && pnpm build`。

## 已知限制

- 当前 Go Foundation 实现认证、当前用户、MCP、知识库与文档域；会话、运行记录、记忆、检索测试与模型配置页面仍明确显示“后端待接入”。
- 管理端以 1280px 及以上桌面窗口为 P0 目标，不提供完整移动端布局。
- 生产包仍有单块体积优化空间，后续可按路由增加懒加载。
