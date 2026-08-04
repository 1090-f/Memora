# Memora MVC 后端框架重建设计

> 状态：待用户书面复核  
> 日期：2026-08-01  
> 参考框架：`C:\Users\冬冬\Desktop\cluadque-releasev1.0`

## 1. 目标

删除当前 Foundation 的模块化六边形目录与旧实施计划，改用参考项目的标准 MVC 三层目录和 App Launcher 模式重新搭建 Memora 后端框架。

采用参考项目的代码组织和依赖注入方式，但不复制与 Memora 需求冲突的 MySQL、AutoMigrate、bcrypt、单进程设计和 CloudQue 业务代码。

## 2. 保留范围

重建过程中必须保留：

- `.git`、`.gitignore`、`LICENSE`；
- `AI智能知识库与知识服务Agent系统需求规格说明书_轻量化数据版.md`；
- `AI智能知识库_API接口文档_P0_轻量化版.md`；
- `AI智能知识库_数据库设计文档_P0_20表版.md`；
- 用户的 `.idea` 目录，不主动修改或提交。

## 3. 删除范围

删除当前框架及其未提交修改：

- `cmd/`、`internal/`、`deploy/`、`migrations/`；
- `docs/superpowers/plans/` 中的旧 Foundation 计划；
- 旧后端架构设计规格；
- `Dockerfile`、`Makefile`、`.env.example`、`README.md`；
- 当前 `go.mod`、`go.sum`；
- 根目录 `后端代码开发规范.md`；
- 已删除测试文件在 Git 工作区中的残留状态。

本设计规格和后续新实施计划不属于“旧计划”，重建后继续保留。

## 4. 参考框架采用项

- `cmd/server/main.go` 的轻量入口；
- `internal/app` 的 App Launcher 初始化流程；
- `internal/api/v1/<resource>` 的 Controller 与 Routes 组织；
- `internal/service` 业务逻辑层；
- `internal/repository` 数据访问层；
- `internal/model/entity` 与 `internal/model/dto/{request,response}`；
- `internal/middleware` 全局与认证中间件；
- `pkg/config`、`pkg/database`、`pkg/errors`、`pkg/jwt`、`pkg/logger`、`pkg/response`；
- `configs`、`scripts`、`docs`、Makefile 和 README 的入口结构；
- 构造函数依赖注入和接口隔离。

## 5. 明确不采用项

- MySQL 和 `gorm.io/driver/mysql`；
- GORM AutoMigrate；
- uint 自增主键；
- bcrypt；
- 可选 Redis；
- 单一 `cmd/server` 进程覆盖全部后台工作；
- 参考项目的默认 Redis 密码、JWT Secret 和宽松 CORS；
- `.claude/settings.local.json`；
- `cloudque` 模块名和业务文案。

## 6. Memora 技术基线

- Module：`github.com/1090-f/Memora`；
- Go 1.25.0；
- Gin 1.12.0；
- GORM 1.31.1 + PostgreSQL Driver 1.6.1；
- PostgreSQL 17 + pgvector；
- Redis 7；
- MinIO；
- golang-migrate；
- JWT v5；
- Argon2id；
- Zap + Lumberjack；
- Viper + YAML + 环境变量覆盖；
- 显式 SQL Migration，禁止 AutoMigrate。

## 7. 目标目录

```text
Memora/
├── cmd/
│   ├── server/main.go
│   ├── worker/main.go
│   └── migrate/main.go
├── configs/
│   └── config.yaml.example
├── docs/
│   ├── ARCHITECTURE.md
│   ├── DEVELOPMENT.md
│   ├── API.md
│   └── superpowers/
│       ├── specs/
│       └── plans/
├── internal/
│   ├── app/
│   │   ├── server.go
│   │   └── worker.go
│   ├── api/
│   │   ├── router.go
│   │   └── v1/
│   │       ├── auth/{controller.go,routes.go}
│   │       └── user/{controller.go,routes.go}
│   ├── middleware/
│   ├── model/
│   │   ├── dto/{request,response}/
│   │   └── entity/
│   ├── repository/
│   └── service/
├── pkg/
│   ├── config/
│   ├── database/
│   ├── errors/
│   ├── jwt/
│   ├── logger/
│   ├── objectstore/
│   ├── response/
│   └── utils/
├── scripts/
│   └── migrations/
├── deploy/
│   ├── docker-compose.yml
│   └── postgres/init.sql
├── Dockerfile
├── Makefile
├── .env.example
├── go.mod
├── go.sum
└── README.md
```

`pkg/utils` 只允许放无业务语义、可独立复用的纯函数，不得成为杂物包。

## 8. 分层职责

### API / Controller

负责 Gin Binding、参数格式校验、身份读取、调用 Service、错误映射和统一响应。禁止直接访问 GORM、Redis、MinIO。

### Service

负责业务规则、权限、事务边界和 Repository 编排。Service 接口与实现分文件组织，所有方法传递 `context.Context`。

### Repository

负责 PostgreSQL/Redis 数据访问、查询过滤和 Entity 映射。查询必须包含用户归属、状态与软删除条件。复杂 pgvector、全文检索和锁使用参数化原生 SQL。

### Model

`entity` 对应数据库表结构；`dto/request` 和 `dto/response` 对应 HTTP 协议。三者禁止混用。

### pkg

保存与具体业务无关的框架能力。业务流程不得下沉到 `pkg`。

## 9. 进程与启动器

- `cmd/server` 创建 Server App，初始化配置、日志、PostgreSQL、Redis、MinIO、Repository、Service、Router 和 HTTP Server。
- `cmd/worker` 创建 Worker App，共用配置与基础设施，但独立管理 Runner 和优雅关闭。
- `cmd/migrate` 只执行 SQL Migration 与开发管理员初始化。
- main 文件保持轻量，初始化细节集中在 `internal/app`。

## 10. Foundation 功能范围

重建后仍只实现：

- 强类型配置；
- PostgreSQL、Redis、MinIO 连接；
- 显式 Migration；
- request_id、结构化日志、Recovery、CORS；
- `/health/live`、`/health/ready`；
- `/api/v1/auth/login`、`/api/v1/auth/logout`；
- `/api/v1/users/me`；
- JWT 黑名单撤销；
- App/Worker 优雅关闭；
- Docker Compose 本地依赖与管理员初始化。

Knowledge、RAG、Memory、Agent 和 MCP 不在本次重建中实现。

## 11. 数据与安全约束

- 数据库表严格遵守当前 20 表文档；本次只创建扩展与完整 `users` 表。
- UUID 使用 `gen_random_uuid()`。
- JWT 使用官方 v5 库并限制 HS256。
- 密码使用 Argon2id。
- Redis、MinIO、PostgreSQL 均是 Server/Worker 必需依赖；启动失败即退出。
- 用户查询过滤 `status='active' AND deleted_at IS NULL`。
- 日志和响应禁止泄露密码、Token、Secret、SQL 和内部错误。

## 12. 文档策略

- `docs/ARCHITECTURE.md`：框架、分层、依赖和进程说明；
- `docs/DEVELOPMENT.md`：按 MVC 结构新增功能的开发步骤；
- `docs/API.md`：Foundation 已实现接口入口，并链接根目录正式 API 文档；
- README：快速启动、配置、管理员和目录入口。

## 13. 验收边界

本次以目录和框架代码落地为目标。根据项目负责人此前指令，不恢复旧测试文件、不执行测试和运行态验证；只能报告静态格式与文件一致性检查结果，不得宣称服务已经构建或运行通过。
