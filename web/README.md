# Memora Frontend

Vue 3 + Vite + TypeScript 单页应用，提供 Memora MVP P0 的完整前端界面。

## 开发环境

```bash
# 安装依赖
pnpm install

# 启动开发服务器
pnpm dev

# 类型检查
pnpm typecheck

# 代码检查
pnpm lint

# 生产构建
pnpm build
```

## 环境变量

在 `web/` 目录下创建 `.env` 文件：

```env
VITE_API_BASE_URL=/api/v1
VITE_MAX_UPLOAD_MB=50
VITE_SSE_RESUME_ENABLED=false
VITE_DOCUMENT_SCOPE_ENABLED=false
```

| 变量 | 说明 | 默认值 |
|---|---|---|
| `VITE_API_BASE_URL` | API 基础地址 | `/api/v1` |
| `VITE_MAX_UPLOAD_MB` | 文件上传大小限制（MB） | `50` |
| `VITE_SSE_RESUME_ENABLED` | SSE 断线续传（需后端支持） | `false` |
| `VITE_DOCUMENT_SCOPE_ENABLED` | 文档作用域问答（需后端支持） | `false` |

## 技术栈

- Vue 3 + Composition API
- Vite 8
- TypeScript
- Vue Router 5
- Pinia
- TanStack Vue Query
- Tailwind CSS 4
- Reka UI
- DOMPurify + markdown-it
- Zod

## 项目结构

```text
web/src/
├── api/              # REST 客户端、SSE、错误处理
├── app/              # 入口、Provider
├── components/       # 基础和共享组件
├── features/         # 业务功能模块
│   ├── auth/         # 登录认证
│   ├── knowledge-base/ # 知识库管理
│   ├── document/     # 文档工作区
│   ├── search/       # 检索测试
│   ├── conversation/ # 会话与聊天
│   ├── agent-run/    # Agent 运行记录
│   ├── memory/       # 长期记忆
│   ├── mcp/          # MCP 配置
│   ├── model-config/ # 模型配置
│   └── user/         # 用户设置
├── layouts/          # 布局组件
├── router/           # 路由配置
├── stores/           # Pinia 状态管理
├── styles/           # 样式和 Design Token
├── types/            # 全局类型定义
└── utils/            # 工具函数
```

## 构建部署

```bash
# 生产构建
pnpm build

# 输出到 web/dist/
```

Docker 部署：

```bash
docker build -t memora-web .
```

## 注意事项

- `VITE_SSE_RESUME_ENABLED` 和 `VITE_DOCUMENT_SCOPE_ENABLED` 默认关闭，需后端确认对应契约后方可启用
- 文档内容只读，不提供在线编辑
- 仅支持桌面端（≥1280px 宽度）
- 不保存模型完整思维链
