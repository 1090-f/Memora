# Memora

Memora 是一个面向个人与团队的 AI 知识库系统。项目采用 Go 模块化单体后端、React
管理端和 Python Docling 文档解析器，提供知识库管理、文档导入与加工、关键词/向量
检索、引用定位、模型配置和 MCP 工具管理能力。

## 当前能力

- 用户登录、资料与密码管理，JWT 鉴权、审计日志和请求追踪。
- Chat、Embedding、Reranker 三类 OpenAI 兼容模型配置，API Key 加密存储。
- 知识库、默认目录、搜索配置和 Agent 配置的原子创建与管理。
- 手工文档、文件上传和安全 URL 导入。
- 支持 `.md`、`.txt`、`.pdf`、`.docx`，单文件最大 50 MB，单次最多 20 个文件。
- PDF/DOCX 使用 Docling 解析，可启用 OCR、表格结构识别、图片提取和页面定位。
- 结构化分段、关键词索引、pgvector 向量索引、RRF 融合和可降级 Reranker。
- 完整正文预览、原文件查看、分页文档读取和可验证引用。
- React 管理端包含知识库、文档工作区、检索测试、模型设置、MCP、个人资料等页面。
- PostgreSQL、Redis、MinIO、Python 解析服务和后台消费者统一健康检查。

## 架构

```text
React/Vite 管理端
        │ /api/v1
        ▼
Gin API（memora-server）
├─ Service / Repository ─────────── ParadeDB PG17 + pg_search + pgvector
├─ Outbox + Redis Stream 消费者 ─── Redis 7
├─ 原文件与解析 Artifact ────────── MinIO
└─ 托管 Python 子进程 ───────────── Docling / OCR
```

文档消费者运行在 `memora-server` 进程内，不需要单独启动 Worker。Python 解析服务同样由
主程序管理：启动时检查并拉起，模型就绪后才开放 API，主程序退出时回收子进程。

文档加工流程：

```text
导入 → 解析 → 清洗 → 结构化分段 → 可选向量化 → 关键词索引 → 完成
```

## 模型要求

三类模型的要求不同，并非创建知识库时全部必填：

| 模型 | 创建知识库 | 文档导入 | 检索用途 |
| --- | --- | --- | --- |
| Chat | 必须指定启用模型，或已有默认 Chat 模型 | 不参与解析和分段 | Agent/问答配置 |
| Embedding | 可选 | 未配置时仍会完成关键词索引 | Vector/Hybrid 检索必须配置 |
| Reranker | 可选 | 不参与文档加工 | 未配置或调用失败时保留 RRF 结果 |

因此，没有 Embedding 模型时文档状态会显示为“已完成（仅关键词）”；成功生成向量后显示
“已完成（混合索引）”。解析和分段本身不依赖 Embedding 或 Reranker。

## 技术栈

- Go 1.25、Gin 1.12、GORM、Eino
- React 19、TypeScript 5.9、Vite 6、MUI、TanStack Query、Redux Toolkit
- Python 3.11、FastAPI、Docling、RapidOCR、ONNX Runtime
- ParadeDB PostgreSQL 17、pg_search、pgvector、Redis 7、MinIO
- Viper、Zap、JWT v5、Argon2id、golang-migrate
- Docker Compose、uv、pnpm 10.12.1

## 使用 Docker Compose 启动

这是最简单的完整启动方式。需要 Docker 和 Docker Compose。

```powershell
Copy-Item .env.example .env
# 编辑 .env，至少修改 JWT Secret、管理员密码和加密密钥
docker compose --env-file .env -f deploy/docker-compose.yml up --build
```

Bash 环境可使用 `cp .env.example .env`。

Compose 会启动 PostgreSQL、Redis、MinIO、API 和管理端，并自动执行数据库迁移与管理员
初始化。Python 3.11、uv、Docling 依赖和解析源码已经打包进 API 镜像，不再创建独立的
`document-parser` 容器；模型缓存在 `docling-models` 数据卷中。

首次构建和首次启动需要下载 Python 依赖及 Docling 模型，耗时会明显长于后续启动。
API 容器默认限制为 4 GB 内存。

服务地址：

- 管理端：`http://localhost:3000`
- API：`http://localhost:8080`
- MinIO Console：`http://localhost:19001`
- Liveness：`GET http://localhost:8080/health/live`
- Readiness：`GET http://localhost:8080/health/ready`
- 后台消费者状态：`GET http://localhost:8080/health/workers`
- Prometheus 指标：`GET http://localhost:8080/metrics`

`.env.example` 的管理员邮箱是 `admin@example.com`。管理员密码至少需要 12 个字符；
发布模式下不得继续使用 `change-me` 开头的示例密钥或密码。

停止服务：

```powershell
docker compose --env-file .env -f deploy/docker-compose.yml down
```

如需同时删除数据库、对象和模型缓存卷，请明确执行 `down -v`；该操作不可恢复。

## 源码开发

### 环境要求

- Go 1.25
- Node.js 24、Corepack、pnpm 10.12.1
- Python 3.11 和 [uv](https://docs.astral.sh/uv/)
- ParadeDB PostgreSQL 17（pg_search + pgvector）、Redis 7、MinIO

复制并修改配置：

```powershell
Copy-Item configs/config.yaml.example configs/config.yaml
Copy-Item .env.example .env
```

环境变量优先于 `configs/config.yaml`，统一使用 `MEMORA_` 前缀。请根据本机环境修改
PostgreSQL、Redis 和 MinIO 地址及凭据。

### 启动后端

在仓库根目录执行：

```powershell
go run ./cmd/server
```

服务启动时会自动执行数据库 Migration，并在配置了引导管理员环境变量时创建管理员。
如需显式执行维护命令：

```powershell
go run ./cmd/migrate up
go run ./cmd/migrate bootstrap-admin
go run ./cmd/migrate reset-admin-password
```

`document_parser.auto_start` 默认开启。主程序通过 `uv` 在
`services/document-parser` 中同步锁定依赖并启动 `127.0.0.1:5001`。如果该端口已有健康
解析服务，主程序会直接复用且不会在退出时关闭外部进程。

Windows 仓库路径包含中文时，建议在 `configs/config.yaml` 中把 Python 环境放到纯 ASCII
路径，避免 Docling C++ 组件读取字体资源失败：

```yaml
document_parser:
  auto_start_environment:
    PYTHONUTF8: "1"
    TORCH_COMPILE_DISABLE: "1"
    UV_PYTHON: "3.11"
    UV_PROJECT_ENVIRONMENT: "C:/venv/document-parser"
```

### 启动管理端

```powershell
Set-Location web
corepack enable
pnpm install --frozen-lockfile
pnpm dev
```

开发服务器默认将 `/api/v1` 代理到 `http://localhost:8080`。前端环境变量示例位于
`web/.env.example`。

## 文档导入与阅读规则

- Markdown：按 Markdown 渲染标题、列表、加粗、代码块和表格。
- TXT：按纯文本显示。
- PDF：默认直接预览 MinIO 中的原始 PDF，可切换到解析产生的完整 Markdown 阅读模式。
- DOCX/XLSX/PPTX：默认在浏览器中直接解析原文件，不要求服务器安装 LibreOffice；可切换到解析正文。
- 如需更接近 Office 的 PDF 版式后备，可显式启用 `preview.office.enabled` 并在 API 运行环境安装 LibreOffice。
- URL：展示安全抓取和清洗后的文章正文。
- 所有文件导入文档均提供独立的原文件下载入口，版式预览不会替代源文件。

正文预览读取完整 Parsed Artifact，不使用检索 Chunk 反向拼接，因此不会因为分段策略而
丢失标题、列表或段落结构。引用接口仍以 Chunk 和 `block_ids` 提供可追溯定位。

处理状态包括：待处理、解析中、清洗中、分段中、向量化中、关键词索引中、已完成和
失败。导入任务的“成功”表示当前可用索引已经建立；具体是关键词索引还是混合索引，
以文档的 `index_mode` 为准。

## 测试与质量检查

后端：

```powershell
go test ./...
go vet ./...
```

管理端：

```powershell
Set-Location web
pnpm lint
pnpm typecheck
pnpm build
```

Python 解析器：

```powershell
Set-Location services/document-parser
uv sync --frozen
uv run ruff check .
uv run mypy app.py schemas.py docling_adapter.py
uv run pytest
```

## 项目目录

```text
cmd/server/                  API、后台消费者与解析器生命周期入口
cmd/migrate/                 Migration 和管理员维护命令
configs/                     YAML 配置示例
deploy/                      Docker Compose 与 PostgreSQL 初始化
internal/api/                HTTP 路由、Controller、响应协议
internal/app/                应用组装和生命周期
internal/background/         Outbox 与 Redis Stream 消费者
internal/service/            业务服务
internal/service/rag/        解析、分段、索引、检索和引用内核
internal/repository/         PostgreSQL 数据访问
pkg/                         配置、日志、数据库、JWT、MinIO 等基础设施
scripts/migrations/          SQL Migration
services/document-parser/    FastAPI + Docling 解析服务
web/admin/                   React 管理端
```

## 相关文档

以下文档包含接口细节与设计背景；当前启动和部署方式以本 README 为准。

- [API 说明](docs/API.md)
- [前端说明](docs/FRONTEND.md)
- [后端架构](docs/ARCHITECTURE.md)
- [开发规范](docs/DEVELOPMENT.md)
- [Docling 解析服务](services/document-parser/README.md)
- [Docling 文档解析执行方案](docs/2026-08-08-docling-document-parsing-execution-plan.md)

## License

[MIT](LICENSE)
