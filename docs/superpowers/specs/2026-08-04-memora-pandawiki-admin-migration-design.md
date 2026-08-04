# Memora PandaWiki 管理端迁移设计

**状态：** 已由用户在对话中逐节确认  
**日期：** 2026-08-04  
**范围：** 以 `PandaWiki-main/PandaWiki-main/web/admin` 为基础，重建 Memora 根目录 `web/`

## 目标

用 PandaWiki 的 React 19 + Vite 管理端替换已移走的 Vue 前端，复用其布局、主题、基础组件以及知识库、文档和会话交互，同时彻底改用 Memora 的产品信息架构、DTO 和 `/api/v1` 协议。

首期以可运行优先：真实接通当前后端已经提供的登录、退出、当前用户、更新资料和修改密码接口；其他业务域先形成可导航、可测试、不会伪造成功数据的页面与强类型接口边界。

## 架构

根目录 `web/` 保持精简的 pnpm workspace：

```text
web/
├─ admin/                 # React 19 + Vite 管理端
├─ packages/
│  ├─ icons/              # 共享图标
│  ├─ themes/             # MUI 主题与设计变量
│  └─ ui/                 # 共享基础组件
├─ package.json
├─ pnpm-workspace.yaml
└─ pnpm-lock.yaml
```

不迁移 PandaWiki 的 Next.js `app/`、后端、SDK 和公开门户。`admin` 对三个共享包存在大量引用，所以首期保留 workspace 边界并将包名渐进改为 `@memora/*`，不把共享代码摊平。

运行时调用链固定为：

```text
Route/Page → Feature Hook → TanStack Query → Memora API Client / SSE Client → /api/v1
```

Redux 只保存认证会话、当前知识库和布局偏好；服务端数据由 TanStack Query 管理。DTO 原样使用 Memora 的 `snake_case`。

## 路由与功能

| 路由 | 功能 | 首期能力 |
|---|---|---|
| `/login` | 登录 | 真实联调 |
| `/knowledge-bases` | 知识库列表 | 明确的后端待接入状态 |
| `/kb/:kbId/docs/:documentId?` | 目录树与只读文档 | 页面及接口边界 |
| `/chat/:kbId/:conversationId?` | 三栏问答工作台 | 页面及 SSE 边界 |
| `/runs/:runId?` | Agent 运行记录 | 页面及接口边界 |
| `/memories` | 长期记忆 | 页面及接口边界 |
| `/mcp` | MCP 服务与工具 | 页面及接口边界 |
| `/kb/:kbId/search-test` | 检索测试 | 页面及接口边界 |
| `/kb/:kbId/settings` | 知识库配置 | 页面及接口边界 |
| `/settings/profile` | 资料与密码 | 真实联调 |
| `/settings/models` | 模型配置 | 页面及接口边界 |

PandaWiki 的发布、贡献者、反馈、公开站点配置和在线文档编辑器不进入 Memora。

## 已实现后端契约

- `POST /api/v1/auth/login`：`{ account, password }`，返回访问令牌和用户。
- `POST /api/v1/auth/logout`：需要 Bearer Token。
- `GET /api/v1/users/me`：返回当前用户。
- `PATCH /api/v1/users/me`：允许修改 `nickname`、`avatar_url`、`bio`、`email`。
- `PATCH /api/v1/users/me/password`：`old_password`、`new_password`，新密码最少 12 个字符。

统一响应为 `{ code, message, data?, details?, request_id }`。前端统一转换成 `AppError`，401 清理会话并携带当前地址跳转登录。

## 安全与实时通信

- Token 仅存 `sessionStorage`，不迁移 PandaWiki 的 `localStorage` Token 行为。
- SSE 通过带 Bearer Token 的 `fetch` 流实现，按 `sequence` 去重，并为 `after_sequence` 续传保留接口。
- Markdown/HTML 渲染必须净化；外链包含 `noopener noreferrer`。
- Secret、Token、完整模型推理不得写入日志、通知或持久化 Store。
- 未实现后端能力显示明确的不可用状态，不使用生产 Mock 伪造成功。

## 阶段与验收

1. 迁入精简 workspace 并完成可重复安装。
2. 去除 PandaWiki 品牌和专属构建假设。
3. 建立 Memora 路由、应用外壳、错误模型和能力状态。
4. 接通认证、当前用户、资料和密码。
5. 改造知识库、文档、聊天、Agent、Memory、MCP 和设置页面。
6. 完成 Nginx、Docker、SSE、安全、可访问性和桌面视觉验收。

自动验证包含 Vitest、React Testing Library、MSW、ESLint、TypeScript 和生产构建；视觉检查覆盖 1280、1440、1920 像素宽度。

## 非目标

- 不修改 Go 后端业务实现。
- 不迁移 Next.js 门户，不保留 Vue/React 双栈。
- 不实现在线文档编辑和移动端完整适配。
- 不兼容 PandaWiki 后端协议。
- 不在迁移计划中删除 `PandaWiki-main/` 参考目录。
