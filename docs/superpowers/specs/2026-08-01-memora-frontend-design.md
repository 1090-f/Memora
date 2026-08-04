# Memora 前端设计方案

> 状态：待用户书面复核
>
> 日期：2026-08-01
>
> 适用范围：Memora MVP P0 桌面 Web 前端
> 前端技术：Vue 3 + Vite + TypeScript

## 1. 文档目的

本文定义 Memora MVP P0 的前端产品形态、信息架构、技术架构、页面边界、数据流、实时 Agent 交互、安全约束、验证方式和分阶段落地顺序。

方案依据以下项目文档：

- `AI智能知识库与知识服务Agent系统需求规格说明书_轻量化数据版.md`；
- `AI智能知识库_API接口文档_P0_轻量化版.md`；
- `AI智能知识库_数据库设计文档_P0_20表版.md`；
- 当前 Go/Gin MVC 后端目录、Foundation API 和部署结构。

## 2. 已确认的设计决策

1. 前端使用 Vue 3，不使用 React。
2. 产品采用“聊天优先 + Notion 式文档工作区”的 B1 双工作区形态。
3. 文档工作区只读，不提供在线富文本编辑。
4. P0 只保证桌面端，不实现手机端布局。
5. 覆盖完整 P0 前端架构，按当前后端实现状态分阶段联调。
6. 技术路线采用产品化自定义方案，不套用传统管理后台模板。
7. UI 基础为 Tailwind CSS + Reka UI，复杂业务组件由 Memora 自行封装。
8. 用户不能手动选择 ReAct 或 Plan-Execute，只展示 Router Agent 的选择结果。

## 3. 目标与非目标

### 3.1 P0 目标

- 提供登录、用户信息和会话级登录态；
- 管理多个相互隔离的个人知识库；
- 管理文档目录、导入文件与 URL，并查看处理状态；
- 以 Notion 式目录树和阅读区查看清洗后的文档内容；
- 提供统一智能问答入口、流式回答、停止与整体重试；
- 展示 Router、ReAct、Plan-Execute、工具调用、Token、耗时和引用；
- 管理长期记忆、模型配置、Agent 配置和 MCP 只读工具；
- 支持知识库引用与网络引用的明确区分和追溯；
- 在桌面端完成 API 文档定义的最小完整验收链路与联网能力验收链路。

### 3.2 P0 非目标

- 在线文档编辑、富文本协同编辑和文档改写；
- 手机端和平板端完整适配；
- 手动选择 Agent 执行模式；
- 可视化工作流、Checkpoint、SubAgent 或多 Agent；
- MCP stdio、写工具和人工确认流程；
- Memory 正文编辑和复杂记忆图谱；
- 模型完整思维链展示；
- 原始 PDF/DOCX 像素级排版还原。

## 4. 产品形态与信息架构

Memora 使用两个一级工作区，并通过全局窄侧栏切换。

```text
Memora
├── 智能问答工作区
│   ├── 会话列表
│   ├── 对话与流式回答
│   └── Agent 实时运行面板
├── 文档工作区
│   ├── 知识库与目录树
│   ├── 只读文档阅读区
│   └── 当前文档 AI 侧栏
├── 知识库管理与检索测试
├── Agent 运行记录
├── 长期记忆
├── MCP 工具
└── 用户、模型与系统设置
```

### 4.1 全局应用框架

`AppShell` 负责：

- 全局窄侧栏；
- 当前用户和知识库切换；
- 一级导航与路由容器；
- 全局搜索入口；
- 全局通知、确认弹窗和错误出口。

一级导航固定包含：智能问答、文档、运行记录、长期记忆、MCP 和设置。

### 4.2 智能问答工作区

采用三栏布局：

1. 左栏：知识库切换、会话搜索、新建会话和会话管理；
2. 中栏：消息流、流式答案、引用、输入框、停止与重试；
3. 右栏：Router 结果、Plan 或 ReAct 轨迹、工具调用、Token、耗时和最终状态。

执行面板默认折叠详细轨迹，只突出当前阶段和关键状态，不挤占回答正文的阅读注意力。

### 4.3 文档工作区

采用 Notion 式三栏布局：

1. 左栏：知识库、目录树、文档搜索、文件导入和 URL 导入；
2. 中栏：面包屑、标题、元信息、处理状态和只读正文；
3. 右栏：明确带有“基于当前文档”作用域的 AI 侧栏。

从聊天引用打开文档时，前端进入对应文档路由，定位 `source_location.section`，并高亮 `quoted_text`。页面提供返回原会话的入口。

## 5. 技术架构

### 5.1 技术栈

- Vue 3：视图层与 Composition API；
- Vite：开发服务和生产构建；
- TypeScript：API DTO、领域模型、组件属性和事件类型；
- Vue Router：SPA 路由、鉴权守卫和懒加载；
- Pinia：登录用户、当前知识库、布局偏好等客户端状态；
- TanStack Vue Query：服务端数据缓存、分页、轮询、失效和请求状态；
- Tailwind CSS：布局、Design Token 映射和业务样式；
- Reka UI：无样式、可访问的弹窗、抽屉、菜单、标签和提示基础；
- DOMPurify：文档与 Markdown HTML 清洗；
- Markdown 渲染器：回答与统一文档正文渲染；
- Fetch API：REST、上传和带 Bearer Token 的 SSE 流。

P0 不引入 SSR 或 Nuxt。系统为登录后的桌面应用，不依赖搜索引擎收录，独立 SPA 更符合当前 Go API 部署形态。

### 5.2 分层

```text
Route Page
→ Feature Component / Composable
→ Vue Query / Pinia / Runtime Store
→ API Client / SSE Client
→ Gin REST API / SSE
```

- 页面只负责编排，不直接拼接 HTTP 请求；
- Feature Composable 封装用例和数据组合；
- API Client 处理协议、认证和错误映射；
- DTO 与后端 JSON 保持 `snake_case`，不做全局字段转换；
- 业务组件依赖 Feature Composable，不依赖具体传输实现。

### 5.3 建议目录

```text
web/
├── src/
│   ├── app/                 # 入口、Provider、Router、AppShell
│   ├── api/                 # API Client、DTO、Envelope、错误映射
│   ├── assets/
│   ├── components/
│   │   ├── base/            # Memora 基础组件
│   │   └── shared/          # 跨业务共享组件
│   ├── features/
│   │   ├── auth/
│   │   ├── knowledge-base/
│   │   ├── document/
│   │   ├── search/
│   │   ├── conversation/
│   │   ├── agent-run/
│   │   ├── memory/
│   │   ├── mcp/
│   │   ├── model-config/
│   │   └── user/
│   ├── layouts/
│   ├── router/
│   ├── stores/
│   ├── styles/
│   ├── types/
│   └── utils/
├── public/
├── index.html
├── package.json
├── tsconfig.json
└── vite.config.ts
```

每个 Feature 只暴露稳定入口，内部可包含 `api.ts`、`queries.ts`、`composables/`、`components/`、`pages/` 和 `types.ts`。禁止把所有接口、Store 或组件集中到单个大文件。

## 6. 路由与页面

| 路由 | 页面 | 主要职责 |
|---|---|---|
| `/login` | 登录 | 登录与错误提示 |
| `/knowledge-bases` | 知识库列表 | 搜索、创建、修改和删除知识库 |
| `/chat/:kbId/:conversationId?` | 智能问答 | 会话、流式回答和实时 Agent 轨迹 |
| `/kb/:kbId/docs/:documentId?` | 文档工作区 | 目录、导入、状态和只读正文 |
| `/kb/:kbId/search-test` | 检索测试 | 关键词、向量、RRF、Reranker 和引用结果 |
| `/kb/:kbId/settings` | 知识库设置 | 基础信息、搜索与 Agent 配置、MCP 授权 |
| `/runs` | Agent 运行列表 | 筛选、分页和运行摘要 |
| `/runs/:runId` | Agent 运行详情 | Router、Plan/ReAct、工具、引用和错误 |
| `/memories` | 长期记忆 | 查看、启停和删除 Memory |
| `/mcp` | MCP 配置 | Server、连接测试、工具发现和启停 |
| `/settings/models` | 模型配置 | Chat、Embedding 和 Reranker 配置 |
| `/settings/profile` | 用户设置 | 基础信息和修改密码 |

路由级页面按 Feature 懒加载。鉴权守卫只负责登录态和基本重定向，资源归属与权限必须由后端判断。

## 7. 页面与组件设计

### 7.1 基础组件

```text
BaseButton       BaseDialog       BaseDrawer
BaseDropdown     BaseTooltip      BaseTabs
BaseTree         BaseVirtualList  BaseInput
StatusBadge      EmptyState       ErrorState
ConfirmDialog    LoadingSkeleton  SecretInput
```

业务页面只能使用 Design Token 或封装后的组件变体，禁止散落品牌色、层级和间距常量。

### 7.2 文档组件

- `KnowledgeTree`：目录展开、排序展示和文档选择；
- `DocumentToolbar`：面包屑、元信息、重新索引、删除；
- `DocumentViewer`：清洗后的统一正文；
- `DocumentProcessingState`：处理阶段、失败原因、重试；
- `CitationHighlight`：章节定位与引文高亮；
- `DocumentAiPanel`：当前文档作用域提示和会话入口；
- `ImportDrawer`：文件与 URL 导入及重复内容处理。

P0 使用 `GET /documents/{document_id}` 返回的 `content` 展示正文。Markdown 按格式渲染；PDF、DOCX 和 TXT 展示后端解析出的统一内容，不还原原始文件排版。原文件预览必须等待后端提供安全的文件读取或签名下载契约。

### 7.3 聊天与 Agent 组件

- `ConversationSidebar`：会话列表、搜索和管理；
- `MessageList`：历史消息和当前流式消息；
- `ChatComposer`：提交、停止和输入状态；
- `AssistantMessage`：Markdown、引用和知识不足提示；
- `CitationPopover`：知识库引用与网络引用；
- `AgentRunPanel`：实时运行总览；
- `RouterSummary`：执行模式和原因摘要；
- `PlanTimeline`：Plan 版本与步骤；
- `ReactRounds`：可观察轮次摘要；
- `ToolCallList`：工具输入输出摘要、耗时和截断状态；
- `UsageSummary`：Token、耗时和 Memory 使用量。

模型完整内部推理文本不进入组件属性、Store 或日志。

### 7.4 配置与管理组件

- 知识库、模型和 MCP 使用卡片列表加抽屉表单；
- Agent 配置按模型、预算、联网、Memory 和工具授权分组；
- API Key 与 MCP Token 使用 `SecretInput`，只支持输入或替换，不回显完整值；
- 危险操作使用二次确认并明确资源影响范围。

## 8. 状态与数据管理

### 8.1 Pinia

Pinia 只保存客户端或跨页面会话状态：

- `authStore`：用户、Access Token、过期时间；
- `workspaceStore`：当前知识库和最近访问路由；
- `layoutStore`：栏宽、折叠状态和密度偏好；
- `agentRuntimeStore`：当前运行中的 SSE 事件投影。

知识库列表、文档、会话、消息、Memory、MCP 和模型配置不复制到 Pinia，统一由 Vue Query 管理。

### 8.2 Vue Query

查询键按资源组织：

```text
knowledgeBases
directories/{kbId}
documents/{kbId}/{filters}
document/{documentId}
conversations/{kbId}
messages/{conversationId}
agentRun/{runId}
memories/{filters}
mcpServers
modelConfigs
```

Mutation 成功后只失效相关查询。文档处理状态仅在 `queued` 或 `processing` 时以 2–3 秒间隔轮询，进入终态后立即停止。

### 8.3 API Client

统一 API Client 负责：

- 添加 `Authorization: Bearer <access_token>`；
- 解包统一响应 Envelope；
- 保留 `request_id`；
- 将错误映射为 `AppError`；
- 支持 JSON、`multipart/form-data`、取消和超时；
- 避免在日志中输出请求正文中的凭证或隐私内容。

## 9. 认证设计

当前后端登录接口只返回 Access Token，P0 将 Token 放入 `sessionStorage`：

- 刷新当前标签页后登录态仍存在；
- 关闭浏览器会话后清除；
- 不保存密码；
- 收到 `401 UNAUTHORIZED` 时取消请求和 SSE、清理状态、跳转登录，并记录原地址；
- 重新登录后返回原地址。

HttpOnly Cookie 与 Refresh Token 需要后端增加契约，列为后续安全增强。前端必须通过严格 CSP、HTML 清洗和最小化第三方脚本降低 Token 被 XSS 读取的风险。

## 10. SSE 与 Agent 实时数据流

### 10.1 请求链

```text
提交问题
→ POST /conversations/{id}/questions
→ 获得 run_id 与 events_url
→ 建立带 Bearer Token 的 SSE Fetch
→ 按 sequence 更新 agentRuntimeStore
→ answer.delta 拼接流式回答
→ run.completed / failed / cancelled 关闭连接
→ 刷新消息、AgentRun、引用和 Memory 查询
```

原生 `EventSource` 无法添加当前 API 要求的 Bearer 请求头，因此 P0 使用 `fetch + ReadableStream` 封装 `agentSseClient`。该客户端负责：

- 解析 `event:` 与 `data:`；
- 校验公共字段；
- 按 `sequence` 去重；
- 使用 `AbortController` 取消；
- 页面离开、退出登录或运行进入终态时关闭连接；
- 将事件投影为 UI 所需摘要，不保存隐藏思维链。

### 10.2 事件投影

| 事件 | 前端行为 |
|---|---|
| `run.started` | 建立运行卡片，状态改为执行中 |
| `memory.retrieved` | 更新 Memory 使用摘要 |
| `router.selected` | 展示模式和原因摘要 |
| `plan.created/replanned` | 创建或切换 Plan 版本 |
| `step.*` | 更新步骤时间线 |
| `agent.round.started` | 增加 ReAct 轮次摘要 |
| `tool.call.*` | 更新工具调用状态 |
| `answer.delta` | 追加回答文本 |
| `citation.created` | 增加引用 |
| `usage.updated` | 更新 Token 使用量 |
| `run.completed` | 完成回答并刷新持久数据 |
| `run.failed/cancelled` | 保留已有内容并显示失败或取消状态 |

### 10.3 断线恢复

前端以 `sequence` 作为去重依据。后端需要补充 `Last-Event-ID` 或 `after_sequence` 契约，使客户端可以从已确认序号继续订阅。

如果 P0 后端尚未支持续传，SSE 断开后前端先查询 AgentRun：

- 已进入终态：加载最终结果；
- 仍在执行：提示连接中断，并允许用户重新连接；
- 重连事件重复：按 `sequence` 丢弃已处理事件。

前端不承诺在后端没有事件续传能力时恢复所有中间动画。

## 11. 当前文档 AI 契约

当前 API 的提问请求只有 `query`。为了让文档右栏安全地限定当前文档上下文，后端需要正式增加可选字段：

```json
{
  "query": "总结当前文档的主要结论",
  "document_id": "uuid"
}
```

后端必须验证 `document_id` 属于当前用户和当前会话知识库，并在 AgentContext 中注入文档范围。前端不得把文档 ID、标题或内容拼进隐藏提示词来模拟授权。

该字段缺失时，文档 AI 侧栏仍可退化为当前知识库问答，但必须明确显示“基于当前知识库”，不得误导为只使用当前文档。

## 12. 视觉设计

### 12.1 视觉语言

- P0 只提供亮色主题，整体采用接近 Notion 的低噪声工作台风格；禁止纯黑大面积背景、低对比度图标和无边界的空白画布；
- 品牌主色固定为 `#7C3AED`，悬停色为 `#6D28D9`，浅品牌背景为 `#F5F3FF`；品牌色只用于主要操作、AI、焦点和选中态；
- 全局导航背景固定为 `#18181B`，禁止使用 `#000000`；导航默认图标为 `#A1A1AA`，悬停图标为 `#FFFFFF`，选中项使用 `#7C3AED` 背景和 `#FFFFFF` 图标；
- 页面背景为 `#F8FAFC`，卡片和正文表面为 `#FFFFFF`，次级表面为 `#FAFAFA`，边框为 `#E4E4E7`；
- 一级文字为 `#18181B`，正文为 `#27272A`，次级文字为 `#71717A`；正文与背景对比度至少 `4.5:1`，图标和交互边界至少 `3:1`；
- 成功使用 `#16A34A`，运行中使用 `#2563EB`，警告使用 `#D97706`，失败使用 `#DC2626`；状态必须同时包含文字或图标，不得只依赖颜色；
- 使用边框、色块和 1 级轻阴影建立层次，禁止用大面积重阴影或渐变充当页面结构；
- 所有业务图标统一使用 Lucide Vue Next，默认 `20px`、`stroke-width=1.75`；主导航图标可使用 `22px`，禁止继续使用散落的占位内联 SVG。

### 12.2 字体、间距与组件密度

- 字体栈固定为 `Inter, "Noto Sans SC", "Microsoft YaHei", system-ui, sans-serif`；无 Inter 时自动使用后续系统字体；
- 页面标题为 `20px/28px`、`600`；区块标题为 `16px/24px`、`600`；正文为 `14px/22px`；辅助文字为 `12px/18px`；
- 以 `4px` 为基础间距单位，常用间距只允许 `4/8/12/16/20/24/32px`；
- 普通按钮高度 `36px`，紧凑按钮 `32px`，输入框高度 `36px`，主导航点击区域至少 `40×40px`；
- 控件圆角使用 `6px`，卡片、抽屉和弹窗使用 `10px`；不要在同一页面混用更多圆角层级；
- 所有可交互元素必须有可见的 hover、active、disabled 和 `2px #8B5CF6` focus ring。

### 12.3 桌面布局

- 支持宽度：1280px 及以上；
- 全局导航栏固定 `60px`，深色背景与主内容之间使用 `1px` 边界；
- 页面顶部栏固定 `64px`，必须显示页面标题、上下文和主要操作，禁止页面进入后只出现空白画布；
- 文档或会话侧栏默认 `260px`，允许在 `240–320px` 之间调整；
- AI 或运行侧栏默认 `360px`，允许在 `320–440px` 之间调整和折叠；
- 文档正文最大阅读宽度约 800px；
- 聊天正文最大宽度约 900px；
- 列表和设置页内容区使用 `24px` 内边距，主要内容最大宽度 `1440px` 并水平居中；
- 用户调整后的栏宽和折叠状态保存在本地。

低于 1280px 时显示明确的窗口宽度提示，不为复杂页面压缩出不可用布局。

### 12.4 页面可见状态

每个路由进入后必须立即呈现页面框架，不能出现只有导航栏、其余区域完全空白的状态：

- 加载中：保留页面标题和工具栏，在内容区显示与最终结构相符的 Skeleton；
- 空数据：显示图标、明确标题、说明文字和一个可执行的主要操作；
- 请求失败：显示错误摘要、重试操作和可复制的 `request_id`；
- 无权限或不存在：使用完整页面状态，不保留不可操作的空白区域；
- 就绪：标题、筛选/操作区和内容区形成清楚的三级视觉层级。

Vue Router 布局组件必须使用 `<RouterView />` 渲染子路由。聊天、知识库、文档、运行记录、Memory、MCP 和设置页全部位于同一个 AppShell 中；禁止某些页面绕过全局导航。

### 12.5 交互规则

- `Ctrl/Cmd + K` 打开全局搜索；
- 流式回答期间保持输入框可见；
- 停止生成后保留已生成内容；
- Router、Plan、ReAct 和工具轨迹默认折叠；
- 知识库引用与网络引用使用不同图标和标签；
- AI 侧栏始终显示当前作用域；
- 所有弹窗、菜单和抽屉支持键盘操作和焦点恢复；
- 状态不能只依赖颜色表达。

## 13. 异常与降级

统一错误模型：

```text
AppError
├── code
├── message
├── httpStatus
├── details
└── requestId
```

- 表单错误显示在字段旁；
- 普通操作失败使用通知；
- 资源不存在或无权限使用完整错误页；
- 文档处理失败展示阶段、原因和整体重试；
- Agent 失败保留已有回答和轨迹，并提供整体重试；
- `429` 提示并发或预算限制，不自动无限重试；
- `502/504` 区分模型、MCP 和工具失败；
- Memory 异常只提示降级，不阻塞已经完成的回答；
- MCP 不可用时保留知识库问答能力；
- 空列表、加载、处理中、失败和无权限必须使用不同页面状态。

面向用户的错误信息附带 `request_id` 复制入口，内部错误、SQL、Token 和 Secret 不进入 UI。

## 14. 前端安全

- Markdown 和文档 HTML 必须经过 DOMPurify 清洗；
- 禁止直接渲染未经处理的 `v-html`；
- 外部链接使用 `noopener noreferrer`；
- API Key 和 MCP Token 只显示脱敏状态；
- 文件类型与大小在前端预检，后端仍为最终安全边界；
- 前端不得提交 `user_id`、运行时 `agent_run_id` 或工具授权名单；
- 日志、通知和错误上报不得包含密码、Token、Secret 或完整私密文档；
- 网络引用 URL 必须由后端完成 SSRF 校验，前端不把“可点击”视为“安全”；
- CSP 至少限制脚本、对象、框架和连接来源；
- 模型完整内部推理文本不得保存到 Store、日志或浏览器持久存储。

## 15. 验证策略

当前仓库要求不保留测试文件，因此 P0 保留以下验证门槛：

- TypeScript 类型检查；
- ESLint；
- Vue SFC 检查；
- 生产构建；
- 1280、1440、1920 三种桌面宽度的视觉检查；
- 按 API 文档第 18 节顺序进行人工联调；
- 按 API 文档第 19 节执行最小完整验收和联网能力验收；
- 验证 SSE 正常完成、失败、取消、断线、重复事件和 Token 失效；
- 验证 Markdown XSS、超长文档、空数据、无权限和上游失败；
- 验证文件导入、文档失败重试、索引重建、引用定位和 Agent 整体重试。

静态检查和人工验收不得被描述为自动化测试通过。若后续允许保留测试文件，推荐增加 Vitest、Vue Testing Library 和 Playwright。

## 16. 构建与部署

前端生产构建产物由 Nginx 提供，并采用同域代理：

```text
/          → Vue SPA
/api/v1    → memora-server
/health    → memora-server
```

部署要求：

- SPA 未命中静态文件的路由回退到 `index.html`；
- SSE 路由关闭代理缓冲；
- SSE 路由延长读取超时并保持连接；
- 不缓存包含用户数据的 API 响应；
- 静态哈希资源使用长期缓存，入口 HTML 使用短缓存；
- 构建时 API 基址可配置，生产默认使用同域 `/api/v1`；
- 前端构建可加入现有 Docker 多阶段流程，但保持与 Go Server 进程解耦。

## 17. 分阶段落地

### 阶段 1：前端基础与认证

- 创建 `web/` 工程；
- AppShell、路由、Design Token、API Client；
- 登录、退出、当前用户和修改密码；
- 统一错误与空状态。

### 阶段 2：知识库与文档工作区

- 知识库列表和设置；
- 目录树和文档列表；
- 文件与 URL 导入；
- 处理状态、失败重试和重新索引；
- 只读 DocumentViewer。

### 阶段 3：搜索与模型配置

- 搜索配置；
- 模型配置；
- 用户搜索与检索测试；
- 关键词、向量、RRF、Reranker 和引用结果展示。

### 阶段 4：会话、问答与 SSE

- 会话和消息；
- 提问与 AgentRun 创建；
- SSE Client、流式回答和运行面板；
- 引用展示、文档定位、停止与整体重试。

### 阶段 5：Agent 详情与 Memory

- Agent 运行列表和详情；
- Router、Plan、ReAct、ToolCall 和 Usage；
- Memory 列表、详情、启停和删除。

### 阶段 6：MCP 与完整联调

- MCP Server 配置和连接测试；
- 工具发现、Schema、启停和知识库授权；
- 联网问答链路；
- 完整桌面视觉、安全和异常验收。

## 18. 前后端契约补充项

在实现对应 UI 前，需要冻结以下契约：

1. `POST /conversations/{id}/questions` 增加可选 `document_id`；
2. SSE 支持 `Last-Event-ID` 或 `after_sequence`；
3. 明确 SSE 首次连接和重连时是否回放历史事件；
4. 若需要原文件预览，增加安全的文件读取或短期签名下载接口；
5. 明确文档 `content` 对 PDF、DOCX、Markdown 和 TXT 的统一格式；
6. 明确 Access Token 到期后的产品行为；P0 重新登录，后续再增加 Refresh Token；
7. Nginx 和其他代理必须对 Agent SSE 路径关闭响应缓冲。

上述补充项不改变 P0 的数据库 20 表设计。

## 19. P0 前端验收标准

1. 用户能登录、退出、查看和修改个人信息；
2. 用户能创建多个知识库并在工作区切换；
3. 用户能管理目录、导入支持格式和 URL，并查看处理状态；
4. 用户能以 Notion 式布局查看统一只读文档内容；
5. 用户能创建会话并接收流式回答；
6. 用户能看到 Router 模式、Plan 或 ReAct 摘要、工具调用和运行状态；
7. 用户能停止任务并整体重试失败运行；
8. 用户能区分、打开和追溯知识库引用与网络引用；
9. 用户能查看和管理长期记忆；
10. 用户能配置模型、MCP Server 和只读工具授权；
11. 认证失效、文档失败、模型失败、MCP 失败、SSE 中断和限流均有明确状态；
12. 页面在 1280、1440 和 1920 宽度下无关键内容遮挡或不可达操作；
13. 前端不提交身份字段、不泄露凭证、不展示模型完整思维链；
14. API 文档定义的最小完整链路与联网能力链路能够通过人工联调验收；
15. 所有主路由都显示标题、加载/空/错误/就绪状态，不出现仅有导航栏的大面积空白页；
16. 文字、图标、边界和选中态满足第 12 节的固定色板、尺寸与对比度要求。

## 20. 开源项目参考边界

- Tencent WeKnora：参考知识库、文档、ReAct、MCP 和引用的信息架构；MIT 许可；
- LobeHub：参考聊天、工具调用、Memory 和 MCP 的交互体验；
- RAGFlow：参考文档处理、检索测试和引用展示；Apache-2.0 许可；
- AnythingLLM：参考个人知识库、会话和文档管理；MIT 许可；
- MaxKB：只参考页面与交互，不直接复制 GPLv3 代码。

Memora 不直接接入上述项目的后端协议，也不整体复制其前端。所有实现必须围绕 Memora 的 `/api/v1`、统一响应 Envelope、AgentRun SSE 和个人用户数据隔离重新建立。
