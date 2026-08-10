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

`document_parser.auto_start` 默认启用。执行 `go run ./cmd/server` 时，主程序会先复用
已就绪的 5001 解析服务；服务不存在时，通过 `uv` 自动创建/同步 Python 3.11 环境并
启动解析服务，主程序退出时一并关闭。首次启动需要下载 Python 依赖和 Docling 模型，
可能耗时数分钟。

部署镜像同样由 `memora-server` 托管解析进程。主镜像已经包含 Python 3.11、`uv`、
锁定的解析依赖和解析服务源码；Compose 不再创建独立的 `document-parser` 容器，
Docling 模型缓存在 API 容器挂载的 `docling-models` 卷中。

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

当前后端包含配置、日志、PostgreSQL/Redis/MinIO、P0 Schema、健康检查、指标、Trace ID、审计日志、用户/知识库/目录/文档/导入 API，以及 Eino 文档加工和混合检索内核。文件、PDF、DOCX 与安全 URL 来源统一进入 Worker；检索支持 keyword/vector/hybrid、RRF、可降级 Reranker、知识充分性和可信 Citation。

当前项目按负责人要求不保留测试文件；不得将静态代码落地描述为已经通过运行态验收。
