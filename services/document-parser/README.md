# document-parser：PDF/DOCX → ParsedDocument（基于 Docling）

Memora 内部解析服务，只负责把 PDF/DOCX 转换为统一的 `ParsedDocument` 协议。
**不包含**任何 RAG 分块策略、Embedding tokenizer、chunk size/overlap 概念；
不访问 MinIO/PostgreSQL/Redis；不建立任务队列。

## 职责边界

| 负责 | 不负责 |
| --- | --- |
| PDF/DOCX → DoclingDocument | Chunk 拆分 / 合并 / overlap |
| OCR、版面、阅读顺序、标题层级 | Embedding tokenizer 与 token 上限 |
| 表格行列、单元格、合并关系 | 表格按行拆分、重复表头 |
| 图片提取（原图/裁剪图、caption） | 图片与正文的 Chunk 关联策略 |
| 页码、bbox、Docling self-ref | 任何持久化与任务队列 |
| ParsedDocument 稳定协议（schema 1.0） | 主动访问外部网络 |

## 目录

```text
services/document-parser/
├── app.py                 # FastAPI、health、parse endpoint
├── schemas.py             # 请求、ParsedDocument 协议（Pydantic）
├── docling_adapter.py     # Docling 初始化、转换和结构适配
├── tests/
├── pyproject.toml         # uv 管理，固定依赖版本（uv.lock）
├── Dockerfile
└── README.md
```

协议字段与 Go 侧 `internal/service/rag/parser/contract.go` 一一对应，契约测试
见 `tests/test_schemas.py` 与 Go `parser/contract_test.go`。

## 本地开发（Windows）

前置：安装 [uv](https://docs.astral.sh/uv/)，Python >= 3.10。

```bash
cd services/document-parser
uv sync
uv run uvicorn app:app --host 0.0.0.0 --port 5001
```

> **Windows 中文路径限制**：docling-parse 的 C++ 组件用窄字符路径打开字体资源，
> 若仓库所在路径含非 ASCII 字符（如 `C:\Users\冬冬\...`），会报
> `filename does not exists: .../glyphs//standard/additional.dat`。
> 解决：把 venv 建在纯 ASCII 路径并同步：
>
> ```bash
> uv venv C:\venv\document-parser --python 3.11
> $env:VIRTUAL_ENV = "C:\venv\document-parser"; uv sync --active
> & C:\venv\document-parser\Scripts\python.exe -m uvicorn app:app --port 5001
> ```

必须设置的环境变量（避免 GBK 编码与 torch 编译问题）：

```bash
$env:PYTHONUTF8 = "1"           # 模型/配置读取使用 UTF-8
$env:TORCH_COMPILE_DISABLE = "1" # 禁用 torch.compile（无需 MSVC）
```

模型首次初始化会下载到 HuggingFace 缓存（`HF_HOME` 可重定向），随后常驻内存。

## 配置（环境变量）

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `DOCUMENT_PARSER_MAX_FILE_BYTES` | 67108864 | 单文件大小上限（字节） |
| `DOCUMENT_PARSER_MAX_ASSET_BYTES` | 33554432 | 单张图片上限 |
| `DOCUMENT_PARSER_MAX_ASSET_COUNT` | 100 | 图片数量上限 |
| `DOCUMENT_PARSER_MAX_TOTAL_ASSET_BYTES` | 67108864 | 图片总量上限 |
| `DOCUMENT_PARSER_MAX_PAGES` | 500 | 最大页数 |
| `DOCUMENT_PARSER_MAX_CONCURRENCY` | 2 | 并发解析上限（模型常驻） |
| `DOCUMENT_PARSER_MAX_REQUEST_TIMEOUT_S` | 480 | 获取并发槽位超时 |
| `DOCUMENT_PARSER_LOG_LEVEL` | info | 日志级别 |

## API

```text
GET  /health/live   存活探针
GET  /health/ready  就绪探针（模型初始化完成后返回 200）
POST /v1/parse      multipart：file（PDF/DOCX）+ options（JSON）
```

`options` 示例：

```json
{
  "schema_version": "1.0",
  "ocr_languages": ["zh", "en"],
  "do_ocr": true,
  "table_structure": true,
  "extract_pictures": true,
  "include_bboxes": true
}
```

`options` 中**不包含** chunk size、overlap、tokenizer 或 Embedding model；
拒绝未知格式、空文件、超限文件与伪造格式（扩展名 + 文件签名双重校验）。

## 测试

```bash
uv run ruff check .
uv run mypy app.py schemas.py docling_adapter.py
uv run pytest                 # 模型用例默认跳过
$env:DOCLING_MODELS_READY="1"; uv run pytest   # 启用真实模型用例
```

## Docker

```bash
docker build -t memora/document-parser .
docker run -p 5001:5001 -v docling-models:/root/.cache/huggingface memora/document-parser
```

镜像使用固定 Python/依赖版本（uv.lock），默认 CPU；模型缓存挂载独立 volume。
