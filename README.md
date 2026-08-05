# Memora Backend

Memora 是个人 AI 智能知识库与知识服务 Agent 系统。本仓库包含基于 Gin 的后端 Foundation，以及位于 `web/admin` 的 React 管理端。

## 框架

```text
Controller → Service → Repository → PostgreSQL / Redis / MinIO
```

- Controller：HTTP Binding、协议校验、统一响应。
- Service：业务逻辑、权限和事务编排。
- Repository：数据库与缓存访问。
- Model：Entity 与 Request/Response DTO 分离。
- App Launcher：统一初始化配置、日志、依赖、路由与优雅关闭。

详细说明：

- [架构说明](docs/ARCHITECTURE.md)
- [开发规范](docs/DEVELOPMENT.md)
- [Foundation API](docs/API.md)
- [React 管理端](docs/FRONTEND.md)

## 技术栈

- Go 1.25.0
- Gin 1.12.0
- GORM 1.31.1
- PostgreSQL 17 + pgvector
- Redis 7
- MinIO
- Viper、Zap、JWT v5、Argon2id、golang-migrate

## 本地启动

```sh
cp .env.example .env
# 修改 .env 中的 JWT Secret、管理员密码和外部服务凭证
docker compose --env-file .env -f deploy/docker-compose.yml up --build
```

服务地址：

- API：`http://localhost:8080`
- Liveness：`GET /health/live`
- Readiness：`GET /health/ready`
- Worker 状态：`GET /health/workers`
- Prometheus 指标：`GET /metrics`
- MinIO Console：`http://localhost:9001`
- React 管理端：`http://localhost:3000`

开发管理员示例账号为 `admin@example.com`。首次启动前必须在本地 `.env` 中修改示例密钥和密码。`bootstrap-admin` 只在用户不存在时创建账号，不会覆盖已有密码；需要重置时显式运行 `go run ./cmd/migrate reset-admin-password`。

## 单独运行

复制 YAML 配置并按本地环境修改：

```sh
cp configs/config.yaml.example configs/config.yaml
go run ./cmd/migrate up
go run ./cmd/migrate bootstrap-admin
go run ./cmd/server
go run ./cmd/worker
```

环境变量使用 `MEMORA_` 前缀，可覆盖配置文件中的敏感配置。

## 前端开发

```sh
cd web
corepack enable
pnpm install --frozen-lockfile
pnpm dev
```

前端默认把 `/api/v1` 代理到 `http://localhost:8080`。完整命令、路由、能力状态和部署说明见 [docs/FRONTEND.md](docs/FRONTEND.md)。

## 当前范围

Foundation 包含配置、日志、PostgreSQL/Redis/MinIO、P0 20 表 Migration、健康检查、指标、Trace ID、审计日志、完整用户接口、`contracts v0.1` 和通用 Worker 执行底座。Knowledge、RAG、Memory、Agent 与 MCP 的业务实现将按需求文档分阶段实现。

当前项目按负责人要求不保留测试文件；不得将静态代码落地描述为已经通过运行态验收。
