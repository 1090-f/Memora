# 任务包 01 交付记录 · 知识库/目录/搜索配置/默认 Agent 配置

> 日期：2026-08-05
> 范围：成员一任务包 01。知识库、默认目录、搜索配置、默认 Agent 配置的原子创建闭环与知识库/目录/搜索配置 API。
> 基线：Eino v0.9.13（go.mod 已锁定），go1.25.0

## 1. 修改文件清单

**新增**：
- `internal/model/entity/knowledge_base.go`、`search_config.go`、`document_directory.go`、`agent_config.go`、`model_config.go`
- `internal/model/dto/request/knowledge_base.go`、`search_config.go`、`directory.go`
- `internal/model/dto/response/knowledge_base.go`、`directory.go`
- `internal/repository/knowledge_base_interface.go`、`knowledge_base_repository.go`
- `internal/repository/directory_interface.go`、`directory_repository.go`
- `internal/repository/config_interface.go`、`config_repository.go`（SearchConfig / AgentConfig / ModelConfig 最小只读）
- `internal/repository/transactor.go`、`tx_context.go`（可注入事务边界）
- `internal/service/knowledge_base_interface.go`、`knowledge_base_service.go`
- `internal/service/directory_interface.go`、`directory_service.go`
- `internal/api/v1/knowledgebase/{controller,routes}.go`
- `internal/api/v1/directory/{controller,routes}.go`

**修改**：
- `internal/app/server.go`（构造 Repository/Service 并注入）
- `internal/api/router.go`（`api.Dependencies` 增 `KnowledgeBases`/`Directories`，挂载路由）

## 2. 已实现行为

- **知识库 CRUD**：创建、分页列表、详情、修改、软删除。
- **原子创建闭环**：`POST /knowledge-bases` 在 `Transactor` 短事务内一次创建 `knowledge_bases + 默认目录(document_directories) + search_configs + agent_configs`，任一步失败整体回滚，不落库任何关联表。
- **默认 Chat 模型决策（按用户确认）**：请求 `default_chat_model_id` → 校验归属/类型/enabled 后使用；否则查当前用户 `is_default=true` 且启用的 Chat 模型；都没有则返回 `INVALID_ARGUMENT` 并拒绝创建，不创建任何关联表。
- **搜索配置**：读取与更新；TopK/RRFK/RRFTopK/RerankerTopK/Threshold 均做范围校验（上界对齐 DB CHECK 与 API 6.x），`reranker_model_id` 校验归属与启用。
- **目录**：目录树读取、目录创建；父目录必须属于当前用户与当前知识库；最大深度 5。
- **归属过滤**：所有 Repository 查询在同一 SQL 中组合 `id + user_id(+ knowledge_base_id) + deleted_at IS NULL`。
- **错误链**：Service 返回 `internal/apperror`，错误码复用 contracts，HTTP 映射走 `internal/api/httperror`，Controller 统一 `internal/api/response`。

## 3. 验证

- `go build ./...`：通过。
- `go vet ./...`：通过。
- `go test ./...`：通过（仅包编译探测，无测试用例）。
- `go build ./cmd/...`、`git diff --check`：通过。
- **运行态验收：未执行**。需要真实 PostgreSQL 才可验证知识库原子创建、越权隔离与目录树。

## 4. 决策与偏差记录

1. **Agent 默认配置依赖 Chat 模型**：`agent_configs.chat_model_id NOT NULL REFERENCES ai_model_configs`，而系统无模型种子数据。按用户决策实现"请求 → 用户默认 → 拒绝"，拒绝时不落库任何表。
2. **默认 Agent 配置 network_enabled 固定 false**：默认 Agent 配置联网权限保守关闭，业务规则由成员二负责。
3. **`GetSearchConfig/UpdateSearchConfig`**：`search_configs` 无 user_id 列，归属通过先校验 `knowledge_bases` 归属推导（仍单查询，不越权）。
4. **`RerankerModelID` 校验用 `FindEnabledByID`**：不强制 model_type=reranker，偏宽但避免成员二模型类型规则未冻结前误拒。
5. **列表 document_count N+1**：List 逐 KB 统计文档数，P0 可接受，后续可优化为 join 一次统计。

## 5. 风险 / 待协作

- API 文档 5.1 请求含 `default_embedding_model_id`/`default_reranker_model_id`，本包已实现读取与归属校验。
- 目录 PATCH/DELETE（API 7.3/7.4，含 `DIRECTORY_NOT_EMPTY` 错误码）不在任务包 01 范围，待后续任务包评估；若实现需新增错误码并同步 httperror + API 文档。
- 数据库迁移未改动（000003 已满足），未新增表，无回写。

## 6. 下一步

任务包 02：文档 CRUD 与 MinIO 对象能力。
