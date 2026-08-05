# Agent Core 模块开发方案

> 负责人：成员三  
> 适用版本：Memora Agent Core P0   
> 文档状态：开发设计方案  
> 关联方案：[MCP 一键导入工具服务技术方案](./mcp-one-click-import-design.md)

## 1. 功能概述

### 1.1 建设目标

Agent Core 负责将一次用户问题编排为可控、可恢复、可审计的 Agent Run，统一承接以下能力：

- 构建 Agent 执行上下文；
- 路由 ReAct 与 Plan-Execute 执行模式；
- 规划、执行、评审与有限重规划；
- 统一调用内置工具和 MCP 只读工具；
- 发布 Agent 生命周期事件，供 SSE 和后台任务消费；
- 持久化运行状态、计划、步骤、工具调用、最终结果和错误摘要。

P0 不保存完整 Chain-of-Thought，只保存必要的执行摘要、状态、工具调用元数据和可追溯引用。

### 1.2 核心执行链路

```text
用户问题
  → 创建 Conversation / AgentRun
  → Worker 领取 queued AgentRun
  → ContextBuilder 构建上下文
  → Router 选择 react 或 plan_execute
  → RetrievalService 检索知识与记忆
  → ToolRegistry 计算可用工具
  → ReAct 循环或 Plan-Execute 执行
  → Reviewer 评审并决定完成/有限重规划
  → 写入 Assistant Message 与 AgentRun
  → 发布完成/失败/取消事件
```

### 1.3 设计约束

- 遵循 `Controller → Service → Repository` 分层，跨模块结构统一复用 `internal/contracts`。
- Agent Run、文档检索、模型调用和 MCP 调用不得阻塞 HTTP 请求，统一由 Worker 执行。
- 所有模型、检索、MCP、工具调用必须支持 `context.Context`、超时、取消和结果大小限制。
- 工具调用必须经过 `ToolExecutor`，不得由 Planner、Router 或执行器绕过注册表直接调用工具。
- MCP P0 仅允许已授权、已启用且 `read_only=true` 的工具，沿用现有 MCP 安全策略。
- 用户身份、知识库归属和资源权限由服务端校验，不能信任请求体中的身份字段。
- 事件序号按 Run 单调递增，SSE 断线通过 `RunID + Sequence` 恢复。

---

## 2. 现有基础与边界

### 2.1 已有 contracts

现有 [agent.go](../../internal/contracts/agent.go) 已定义 `AgentConfig`、`AgentRunRequest`、`AgentRunResult` 和 `AgentRunService`；[context.go](../../internal/contracts/context.go) 已定义 `AgentContext` 与 `ContextBuilder`；[router.go](../../internal/contracts/router.go) 已定义路由决策；[plan.go](../../internal/contracts/plan.go) 已定义计划、步骤、Planner、PlanExecutor 和 PlanReviewer。

工具层已经具备：

- [tool.go](../../internal/agent/tools/tool.go)：统一 `Tool` 接口；
- [registry.go](../../internal/agent/tools/registry.go)：工具注册和查询；
- [executor.go](../../internal/agent/tools/executor.go)：启用、授权、只读、联网、超时、结果清洗与截断；
- [mcp.go](../../internal/agent/tools/mcp.go)：将 MCP 工具包装为普通 Agent 工具；
- [client.go](../../internal/mcp/client.go)：Streamable HTTP/stdio MCP 客户端。

### 2.2 已有数据模型

[000005_conversation_agent.up.sql](../../scripts/migrations/000005_conversation_agent.up.sql) 已提供：

- `conversations`、`messages`；
- `agent_runs`；
- `agent_plans`、`agent_plan_steps`；
- `tool_calls`。

[MCP 相关迁移](../../scripts/migrations/000006_memory_mcp.up.sql) 已提供 `mcp_servers`、`mcp_tools` 和 `agent_mcp_tools`。MCP 导入、发现和安全约束以 [mcp-one-click-import-design.md](./mcp-one-click-import-design.md) 为准，Agent Core 只消费授权后的工具规格，不重复实现 MCP 导入逻辑。

### 2.3 本模块负责与不负责的内容

| 范围 | Agent Core 负责 | 其他模块负责 |
|---|---|---|
| Agent Run | 创建后的执行编排、状态机、取消、重试 | Conversation API 创建消息和 Run 的入口事务 |
| 上下文 | 组装会话、检索结果、记忆和工具白名单 | Conversation、Retrieval、Memory 服务提供事实数据 |
| 模型 | 调用统一 `ModelFactory`，解析结构化输出 | 供应商适配、模型配置和凭证管理 |
| 工具 | 调用 `ToolExecutor`、记录调用摘要 | 内置工具实现、MCP 导入和 MCP 连接实现 |
| 事件 | 发布 AgentEvent、保证序号 | SSE 订阅接口和前端展示 |
| 持久化 | 通过 Repository 更新 Agent 状态 | Repository 负责 SQL、所有权过滤和错误映射 |

---

## 3. 模块架构设计

### 3.1 目录结构

```text
internal/agent/
├── core/
│   ├── service.go             # Agent Core 用例接口与依赖
│   ├── run_service.go         # Run 生命周期编排
│   ├── react_runner.go        # ReAct 循环
│   ├── plan_runner.go         # Plan-Execute 执行
│   ├── state_machine.go       # 状态转换校验
│   ├── output.go              # 模型输出与最终结果归一化
│   └── errors.go              # Agent Core 内部错误
├── planner/                   # Planner、Reviewer 的模型适配实现
├── router/                    # Router 实现和结构化输出兜底
├── context/                   # ContextBuilder 实现
└── tools/                     # 已有 Tool、Registry、Executor、MCPTool
```

Repository、Entity、DTO 和 API 仍分别位于既有目录：

```text
internal/repository/
internal/model/entity/
internal/model/dto/{request,response}/
internal/api/v1/agent/
internal/worker/
```

### 3.2 依赖方向

```text
Agent API Controller
        ↓
AgentRunService / Agent Core
        ↓
Contracts + Repository interfaces + Retrieval + Memory + ModelFactory + ToolExecutor
        ↓
PostgreSQL / Redis / Model provider / MCP client
```

`internal/agent/core` 不依赖 Gin、GORM、具体模型供应商 SDK 或具体 MCP Repository 实现；所有外部依赖由 `internal/app` 通过构造函数注入。

### 3.3 核心接口

```go
type AgentCore interface {
    Run(ctx context.Context, req contracts.AgentRunRequest) (contracts.AgentRunResult, error)
    Cancel(ctx context.Context, runID, userID contracts.ID) error
    Retry(ctx context.Context, runID, userID contracts.ID) (contracts.ID, error)
}

type ReactRunner interface {
    Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error)
}

type PlanRunner interface {
    Run(ctx context.Context, agentCtx contracts.AgentContext, cfg contracts.AgentConfig) (RunOutput, error)
}

type RunOutput struct {
    FinalResult string
    Citations   []contracts.Citation
    Usage       contracts.TokenUsage
    Summary     string
}
```

实现接口时保持最小能力集合：Planner、PlanExecutor、PlanReviewer、Router、ContextBuilder、ToolExecutor、EventPublisher 均优先依赖已有 contracts，不新增同义结构。

---

## 4. Agent Run 状态机

### 4.1 状态

| 状态 | 含义 |
|---|---|
| `queued` | 已创建，等待 Worker 领取 |
| `running` | Agent 正在执行 |
| `completed` | 已生成最终结果并成功落库 |
| `failed` | 发生不可恢复错误 |
| `cancelled` | 收到用户或系统取消请求 |

### 4.2 合法转换

```text
queued → running
queued → cancelled
running → completed
running → failed
running → cancelled
failed → queued       # 仅 Retry 创建新的 Run，不直接复用旧 Run
```

每次状态变更必须满足：

1. 使用 `run_id + current_user_id` 查询；
2. 通过条件更新避免并发 Worker 重复处理；
3. 写入错误码、执行摘要、开始/结束时间和耗时；
4. 发布对应 `AgentEvent`；
5. 不覆盖已完成、失败或取消的终态。

### 4.3 运行上限

默认值复用 [DefaultAgentConfig](../../internal/contracts/agent.go)：

| 限制 | 默认值 | 说明 |
|---|---:|---|
| ReAct 最大轮数 | 8 | 防止模型循环 |
| Plan 最大步骤 | 5 | 与数据库约束一致 |
| 最大重规划次数 | 1 | 与 `agent_runs.replan_count` 一致 |
| 最大工具调用数 | 10 | 单次 Run 总上限 |
| 文档读取最大 token | 6000 | 防止上下文膨胀 |
| 工具结果最大字节 | 1 MB | 与 ToolResult 截断策略一致 |
| 单次 Run 最大时长 | 300 秒 | Worker 和 Service 双重控制 |
| 记忆 TopK | 8 | 由 ContextBuilder 传递 |

请求方可以降低限制，不得突破系统最大值；缺省配置由 Service 统一补齐，不能由模型输出决定。

---

## 5. 执行流程设计

### 5.1 ContextBuilder

ContextBuilder 按以下顺序构建上下文：

1. 校验用户、知识库、会话和 Run 的归属关系；
2. 读取会话历史，按 token 上限裁剪；
3. 调用 RetrievalService 获取知识结果和 Citation；
4. 调用 MemoryRetriever 获取用户/知识库范围内的有效记忆；
5. 从 ToolRegistry 获取工具规格，并根据授权关系生成 `AllowedTools`；
6. 初始化 `AgentContext`，禁止把 Secret、完整 Prompt 或内部错误放入上下文持久化字段。

### 5.2 Router

Router 输出必须能解析为 `contracts.RouterDecision`：

- 复杂多步骤任务优先 `plan_execute`；
- 简单问答、单次检索或单次工具任务优先 `react`；
- 非法输出、超时或模型失败统一兜底为 `react`；
- 只记录 `ReasonSummary`、`Confidence` 和 `FallbackUsed`，不保存完整思维过程；
- 发布 `agent.router.completed` 事件。

### 5.3 ReAct Runner

```text
初始化上下文
  → 请求模型生成 answer 或 tool_call
  → 若为 tool_call：校验轮次/调用次数 → ToolExecutor.Execute
  → 写入工具调用摘要和结果 → 发布 tool.started/tool.completed
  → 将受限结果追加到上下文
  → 继续下一轮
  → answer 或达到上限 → 归一化最终结果
```

要求：

- 模型输出必须使用结构化协议，无法解析时返回稳定的模型错误；
- 工具名必须存在于 ToolRegistry 且在 `AllowedTools` 中；
- 工具参数在执行前按 `ToolSpec.InputSchema` 校验；
- 达到轮数、调用次数或运行时长上限时，优先返回已有结果，否则失败；
- 不把原始完整工具参数和结果写入日志，数据库只保存脱敏摘要与必要元数据。

### 5.4 Plan-Execute Runner

```text
Planner.Plan
  → 校验步骤数量、依赖关系、ToolHint 和完成判据
  → 保存 agent_plans / agent_plan_steps
  → 按依赖顺序执行 PlanStep
  → 每一步调用 RetrievalService / ToolExecutor / ModelFactory
  → 更新步骤状态和摘要
  → PlanReviewer.Review
  → pass：生成最终结果
  → needs_attention：最多重规划 1 次
  → failed：结束 Run
```

校验规则：

- 步骤数量为 1–5；
- `step_no` 连续且唯一；
- `depends_on` 只能引用已存在步骤，禁止循环依赖；
- `ToolHint` 为空或必须属于 ToolRegistry；
- 每步结束必须写入状态、耗时和输出摘要；
- 重规划时旧计划保留为历史版本，新计划递增 `version`，同一 Run 仅一个 `is_current=true`。

### 5.5 输出归一化

最终结果统一生成：

- `FinalResult`：面向用户的回答文本；
- `Citations`：来自 Retrieval 或工具真实返回的可追溯引用；
- `Usage`：输入、输出和总 token；
- `StartedAt`、`EndedAt`；
- `KnowledgeStatus`、`ExecutionMode`；
- 执行摘要和稳定错误码。

禁止生成无法追溯的 Citation，禁止把工具密钥、内部堆栈和完整模型 Prompt 输出给客户端。

---

## 6. Worker 与异步执行

### 6.1 Job 设计

Agent Run 使用已有 [worker.Job](../../internal/worker/job.go) 模型：

| 字段 | 设计 |
|---|---|
| `Type` | `agent.run.execute` |
| `Payload` | `run_id`，不携带完整 Prompt 或 Secret |
| `IdempotencyKey` | `agent-run:{run_id}` |
| `Timeout` | 使用 `AgentConfig.MaxRunSeconds`，不得超过系统上限 |
| `MaxAttempts` | P0 默认 2，取消、参数错误不重试 |

### 6.2 Source 与 Handler

- Source 复用 `agent_runs` 表，领取时使用事务、行锁或原子状态更新；
- Handler 只根据 `run_id` 重新加载必要数据，避免依赖进程内状态；
- Handler 必须响应 Context 取消，在 `cancelled` 状态落库并发布事件；
- 成功、失败、超时和取消都更新 `agent_runs`；
- 幂等 Claim 成功后才执行，重复任务不得重复生成 Assistant Message；
- 外部调用结束后再提交短事务，事务中禁止等待模型、MCP 或网络。

### 6.3 取消与重试

- Cancel 只允许当前用户取消自己的 queued/running Run；
- Retry 不修改原 Run，创建新的 Run，并设置 `retry_of_run_id`；
- 已完成、已取消的 Run 不允许再次取消；
- 失败原因按错误码分类，参数错误、权限错误和资源冲突不自动重试；
- 重试采用有限次数和退避，禁止无限重试。

---

## 7. API 设计

> Agent API 只负责创建、查询、取消和重试；实际执行由 Worker 完成。所有响应使用统一 Envelope：`code`、`message`、`data/details`、`request_id`。

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/v1/agent/runs` | 创建会话消息并排队 Agent Run |
| `GET` | `/api/v1/agent/runs/:id` | 查询 Run 摘要、状态和最终结果 |
| `GET` | `/api/v1/agent/runs/:id/events` | SSE 订阅事件，支持 `after_sequence` |
| `POST` | `/api/v1/agent/runs/:id/cancel` | 取消 queued/running Run |
| `POST` | `/api/v1/agent/runs/:id/retry` | 基于失败 Run 创建新 Run |
| `GET` | `/api/v1/agent/runs` | 按用户、知识库、会话和状态分页查询 |

创建请求建议：

```json
{
  "knowledge_base_id": "uuid",
  "conversation_id": "uuid",
  "query": "请总结本知识库中关于项目部署的内容",
  "config": {
    "max_react_rounds": 8,
    "max_tool_calls": 10
  }
}
```

约束：

- `user_id` 从 Auth Middleware 获取，不接受请求体传入；
- `query`、请求体和配置均设置最大长度；
- 创建消息与 queued Run 在同一短事务中完成；
- HTTP 接口只返回排队成功和 RunID，不同步等待 Agent 完成；
- SSE 不输出完整内部思维链，只输出状态、步骤、工具摘要、回答增量和最终结果。

事件示例：

```json
{
  "event_id": "uuid",
  "run_id": "uuid",
  "event_type": "agent.tool.completed",
  "sequence": 6,
  "timestamp": "2026-08-05T10:00:00Z",
  "data": {
    "tool_name": "mcp.server.search",
    "success": true,
    "truncated": false,
    "summary": "检索到 3 条结果"
  }
}
```

---

## 8. 数据访问设计

### 8.1 Repository 接口

建议新增以下接口，接口由 Service 使用方定义，具体实现放置在 `internal/repository`：

```go
type AgentRunRepository interface {
    CreateQueued(ctx context.Context, run *entity.AgentRun) error
    FindByID(ctx context.Context, userID, runID string) (*entity.AgentRun, error)
    ReserveQueued(ctx context.Context, runID string) (*entity.AgentRun, error)
    MarkRunning(ctx context.Context, runID string, startedAt time.Time) error
    MarkCompleted(ctx context.Context, runID string, result AgentCompletion) error
    MarkFailed(ctx context.Context, runID string, failure AgentFailure) error
    MarkCancelled(ctx context.Context, userID, runID string) error
    CreateRetry(ctx context.Context, userID, runID string) (string, error)
}
```

实际命名以仓库现有约定为准，禁止将上面的伪代码直接作为第二套 contracts。所有资源查询必须同时过滤 `run_id + user_id`，涉及知识库和会话时增加对应归属条件。

### 8.2 事务边界

| 事务 | 内容 | 禁止内容 |
|---|---|---|
| 创建 Run | User Message + queued AgentRun | 模型、检索、MCP、MinIO |
| 领取 Run | queued → running | 长时间外部 I/O |
| 保存计划 | 计划版本、步骤初始化 | 模型调用 |
| 完成 Run | Assistant Message + AgentRun 完成状态 | 等待 SSE 或外部服务 |
| 保存工具调用 | 工具调用状态和脱敏摘要 | 保存 Secret、完整结果 |

---

## 9. 安全、可靠性与可观测性

### 9.1 安全

- 工具调用统一走 `ToolExecutor` 的授权、启用、只读和联网校验；
- MCP 工具同时校验 User、Knowledge Base、Agent、Server、Tool 和授权关系；
- 不接受模型生成的用户身份、知识库身份或权限字段；
- 日志禁止记录 Token、Authorization、MCP Credential、完整 Prompt 和完整工具参数；
- 结果和事件使用脱敏摘要，敏感字段不进入 `execution_trace`；
- Agent 输出中的引用必须来自真实 Retrieval、Document、Chunk 或 URL。

### 9.2 超时与资源限制

- Run 总超时由 `MaxRunSeconds` 控制；
- Router、Planner、Reviewer、Model、Retrieval、Tool、MCP 分别设置独立超时；
- 限制会话历史、模型输出、工具调用次数、工具结果字节数和事件数据大小；
- Goroutine 必须由启动方负责退出和等待，禁止无限并发；
- MCP 连接沿用 [MCP 方案](./mcp-one-click-import-design.md) 的 SSRF、stdio 白名单、进程回收和响应大小限制。

### 9.3 日志、审计和指标

结构化日志至少包含 `request_id`、`trace_id`、`run_id`、`user_id`、`event_type`、`duration`、`status` 和 `error_code`，不包含敏感内容。

需要审计的操作：

- 创建、取消、重试 Agent Run；
- Agent 工具授权变更；
- MCP 工具调用和失败；
- Agent 最终完成或失败。

建议指标：

- `agent_run_total{mode,status}`；
- `agent_run_duration_seconds`；
- `agent_tool_call_total{type,status}`；
- `agent_tool_call_duration_seconds`；
- `agent_router_fallback_total`；
- `agent_replan_total`；
- `agent_run_queue_depth`。

---

## 10. 开发任务分解

每个 Task 都应独立编译、静态检查并完成对应验收；不要在 Controller 中临时实现业务逻辑。

### Task 0：确认 contracts 与数据约束

| 项目 | 内容 |
|---|---|
| 文件变更 | 必要时修改 `internal/contracts`；同步 API/架构说明 |
| 依赖 | 无 |
| 产出 | 明确 AgentCore、RunOutput、事件和错误码边界；确认现有表约束与配置默认值 |
| 验收 | 不新增同义结构；破坏性 contracts 变更完成影响说明 |

### Task 1：Agent Entity、DTO 与 Repository

| 项目 | 内容 |
|---|---|
| 文件变更 | `internal/model/entity`、`internal/model/dto`、`internal/repository` |
| 依赖 | Task 0 |
| 产出 | AgentRun、Plan、PlanStep、ToolCall 的持久化映射；用户/知识库所有权过滤 |
| 验收 | 查询均包含归属条件；状态更新具有并发保护；不使用 AutoMigrate |

### Task 2：ContextBuilder 与 Router

| 项目 | 内容 |
|---|---|
| 文件变更 | `internal/agent/context`、`internal/agent/router` |
| 依赖 | Task 0、Task 1；Retrieval/Memory/Model contracts |
| 产出 | 上下文裁剪、知识/记忆检索编排、工具白名单、Router 结构化输出和 react 兜底 |
| 验收 | 未授权工具不会进入 `AllowedTools`；Router 非法输出安全兜底 |

### Task 3：Tool 调用与 MCP 适配

| 项目 | 内容 |
|---|---|
| 文件变更 | 扩展 `internal/agent/tools`，必要时补充调用 Repository |
| 依赖 | Task 1、现有 MCP 方案 |
| 产出 | Agent Core 通过 ToolExecutor 调用内置/MCP 工具，并保存脱敏调用摘要 |
| 验收 | 禁止绕过 Executor；MCP 写工具拒绝；结果超限可截断；调用可取消 |

### Task 4：ReAct Runner

| 项目 | 内容 |
|---|---|
| 文件变更 | `internal/agent/core/react_runner.go`、模型适配调用 |
| 依赖 | Task 2、Task 3 |
| 产出 | 结构化模型输出解析、有限 ReAct 循环、工具调用和最终结果归一化 |
| 验收 | 达到轮数/调用/时长上限可安全结束；所有状态和事件正确发布 |

### Task 5：Planner、Plan-Execute 与 Reviewer

| 项目 | 内容 |
|---|---|
| 文件变更 | `internal/agent/planner`、`internal/agent/core/plan_runner.go` |
| 依赖 | Task 2、Task 3、Task 4 |
| 产出 | 计划校验、步骤执行、评审、最多一次重规划和版本持久化 |
| 验收 | 依赖无环；工具提示合法；步骤状态可恢复；重规划不超过配置上限 |

### Task 6：Agent Core Run Service 与状态机

| 项目 | 内容 |
|---|---|
| 文件变更 | `internal/agent/core/run_service.go`、状态更新 Repository |
| 依赖 | Task 1、Task 4、Task 5 |
| 产出 | Run 生命周期、取消、重试、完成/失败落库和幂等控制 |
| 验收 | 合法状态转换生效；终态不被覆盖；重复消费不会重复生成结果 |

### Task 7：Worker 注册与事件发布

| 项目 | 内容 |
|---|---|
| 文件变更 | `internal/worker`、事件 Repository/Publisher |
| 依赖 | Task 6 |
| 产出 | `agent.run.execute` Source/Handler、幂等键、重试、取消、AgentEvent 发布 |
| 验收 | HTTP 不同步执行 Agent；Worker 重启后可恢复 queued Run；Sequence 单调递增 |

### Task 8：API、依赖注入与文档同步

| 项目 | 内容 |
|---|---|
| 文件变更 | `internal/api/v1/agent`、`internal/api/router.go`、`internal/app`、`docs/API.md` |
| 依赖 | Task 6、Task 7 |
| 产出 | Run 创建、查询、SSE、取消、重试接口和依赖组装 |
| 验收 | 统一响应格式、认证和错误映射正确；所有依赖由 `internal/app` 注入 |

---

## 11. 验收标准

1. 创建 Agent Run 只产生 queued 任务，HTTP 请求不等待模型执行。
2. Worker 能领取任务并完成 `queued → running → completed/failed/cancelled` 生命周期。
3. ContextBuilder 正确合并会话、知识检索、记忆和授权工具白名单。
4. Router 能在 `react` 与 `plan_execute` 间选择，非法输出安全兜底到 `react`。
5. ReAct 能在最大轮数、工具调用次数和总时长限制内运行。
6. Plan-Execute 能保存计划和步骤状态，处理依赖，最多重规划一次。
7. 内置工具和 MCP 工具都必须经过 ToolExecutor；未授权、停用、非只读 MCP 工具调用失败。
8. 工具结果超过限制时正确截断并标记 `truncated=true`。
9. AgentEvent 的 Sequence 按 Run 单调递增，SSE 可从指定序号恢复。
10. 取消能中止后续模型、检索、工具和 MCP 调用，并落库为 `cancelled`。
11. Retry 创建新的 Run，原 Run 保留失败事实和 `retry_of_run_id` 关系。
12. Agent 最终结果包含真实 Citation、Token 用量和执行模式，不包含完整思维链。
13. Repository 查询均包含用户/知识库归属过滤，状态更新具有并发保护。
14. 日志、审计和响应不泄露 Secret、完整 Prompt、内部堆栈或敏感工具参数。
15. 交付说明如实列出执行过的格式化、编译、静态检查和未执行的运行态验证；不得将未执行的测试描述为通过。

---

## 12. 风险与后续扩展

| 风险 | P0 处理 |
|---|---|
| 模型输出格式不稳定 | 使用结构化输出；解析失败统一错误；Router 兜底 react |
| Agent 无限循环 | ReAct 轮数、工具调用次数、总时长三重上限 |
| Worker 重复消费 | Source 原子领取 + IdempotencyStore + 终态保护 |
| 工具结果污染上下文 | 结果清洗、字节上限、截断标记和摘要化 |
| MCP 外部服务不稳定 | 独立超时、取消、有限重试和连接状态落库 |
| SSE 断线丢事件 | 事件持久化，使用 RunID + Sequence 恢复 |

P1 可扩展能力：

- 人工确认、暂停和恢复工具调用；
- Agent 运行的人工接管与可视化调试；
- 计划步骤并行执行和资源配额；
- 长期 MCP 会话池与健康检查；
- 模型路由、成本预算和质量评估；
- Agent 运行回放，但仍不保存完整 Chain-of-Thought。

---

## 13. 交付检查清单

- [ ] 已对齐 `internal/contracts`、Agent 数据库表、MCP 方案和 API 文档。
- [ ] Controller、Service、Repository 依赖方向符合架构规范。
- [ ] Agent 长任务已注册 Worker，具备幂等、超时、重试和取消。
- [ ] 所有外部调用继承 Context，并设置超时和结果大小限制。
- [ ] ToolExecutor 是唯一工具执行入口，MCP P0 只允许只读工具。
- [ ] Agent Run、Plan、Step、ToolCall 状态变更可审计、可恢复。
- [ ] Repository 查询包含用户、知识库和状态过滤。
- [ ] 没有持久化完整 Chain-of-Thought、Secret 或完整 Prompt。
- [ ] 新增 Migration 使用连续编号，未修改已共享 Migration。
- [ ] API、配置、架构和数据库文档已同步。
- [ ] 已执行 `gofmt`、编译和约定的静态检查；未执行的验证已明确说明。
