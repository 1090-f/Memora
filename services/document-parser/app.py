"""Memora document-parser 服务入口。

职责边界：
  - 只提供 GET /health/live、GET /health/ready、POST /v1/parse；
  - 常驻 FastAPI 进程，Docling/OCR 模型跨请求复用；
  - 不接受 chunk size/overlap/tokenizer/Embedding 参数；
  - 拒绝未知格式、空文件、超限文件和伪造格式；
  - 禁止请求携带任意模型路径、远程 URL、shell 参数或插件名称；
  - 不访问 MinIO/PostgreSQL/Redis，不建立任务队列。
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
import os
import threading
from collections.abc import AsyncIterator

from docling import __version__ as DOCLING_VERSION
from fastapi import FastAPI, File, Form, HTTPException, Request, UploadFile
from fastapi.responses import JSONResponse

import schemas
from docling_adapter import DoclingAdapter, DocumentParserError

logging.basicConfig(
    level=os.environ.get("DOCUMENT_PARSER_LOG_LEVEL", "info").upper(),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
_log = logging.getLogger("document-parser")

# ---------------------------------------------------------------- 配置

MAX_FILE_BYTES = int(os.environ.get("DOCUMENT_PARSER_MAX_FILE_BYTES", str(64 * 1024 * 1024)))
MAX_ASSET_BYTES = int(os.environ.get("DOCUMENT_PARSER_MAX_ASSET_BYTES", str(32 * 1024 * 1024)))
MAX_ASSET_COUNT = int(os.environ.get("DOCUMENT_PARSER_MAX_ASSET_COUNT", "100"))
MAX_TOTAL_ASSET_BYTES = int(
    os.environ.get("DOCUMENT_PARSER_MAX_TOTAL_ASSET_BYTES", str(64 * 1024 * 1024))
)
MAX_PAGES = int(os.environ.get("DOCUMENT_PARSER_MAX_PAGES", "500"))
MAX_CONCURRENCY = int(os.environ.get("DOCUMENT_PARSER_MAX_CONCURRENCY", "2"))
MAX_REQUEST_TIMEOUT_S = int(os.environ.get("DOCUMENT_PARSER_MAX_REQUEST_TIMEOUT_S", "480"))

# ---------------------------------------------------------------- 适配器与并发控制

adapter = DoclingAdapter(
    max_asset_bytes=MAX_ASSET_BYTES,
    max_asset_count=MAX_ASSET_COUNT,
    max_total_asset_bytes=MAX_TOTAL_ASSET_BYTES,
    max_pages=MAX_PAGES,
)
_semaphore = threading.BoundedSemaphore(MAX_CONCURRENCY)
_ready = threading.Event()
_init_error: str | None = None


def _initialize_models() -> None:
    """初始化常驻模型（幂等）；失败时记录错误并使 /health/ready 返回 503。"""
    global _init_error
    try:
        adapter.initialize()
        _init_error = None
    except Exception as exc:  # 模型下载/加载失败
        _init_error = f"{type(exc).__name__}: {exc}"
        _log.error("Docling 模型初始化失败: %s", _init_error)
        raise
    finally:
        _ready.set()


@contextlib.asynccontextmanager
async def lifespan(_app: FastAPI) -> AsyncIterator[None]:
    thread = threading.Thread(target=_initialize_models, name="docling-init", daemon=True)
    thread.start()
    yield


app = FastAPI(
    title="Memora document-parser",
    version="0.1.0",
    lifespan=lifespan,
    docs_url=None,
    redoc_url=None,
)


@app.get("/health/live")
async def health_live() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/health/ready")
async def health_ready() -> JSONResponse:
    if not _ready.is_set():
        return JSONResponse({"status": "starting"}, status_code=503)
    if _init_error is not None:
        return JSONResponse({"status": "error", "detail": _init_error}, status_code=503)
    return JSONResponse({"status": "ok"})


@app.post("/v1/parse", response_model=schemas.ParsedDocument)
async def parse_document(
    request: Request,
    file: UploadFile = File(...),  # noqa: B008
    options: str = Form("{}"),  # noqa: B008
) -> schemas.ParsedDocument:
    """解析 PDF/DOCX 为 ParsedDocument。

    multipart 字段：
      - file: PDF 或 DOCX 原始字节；
      - options: 解析选项 JSON（见 schemas.ParseOptions）。
    响应为 ParsedDocument JSON（不含任何 Chunk/RAG 字段）。
    """
    if not _ready.is_set():
        raise HTTPException(status_code=503, detail="document-parser 正在初始化模型")
    if _init_error is not None:
        raise HTTPException(status_code=503, detail="document-parser 模型初始化失败")

    try:
        parse_options = schemas.ParseOptions.model_validate_json(options)
    except Exception as exc:
        raise HTTPException(status_code=422, detail=f"options 无效: {exc}") from exc

    file_name = file.filename or ""
    if file_name == "" or not _is_supported_document(file_name):
        raise HTTPException(status_code=422, detail=f"不支持的格式: {file_name!r}")

    content = await _read_limited(request, file, MAX_FILE_BYTES)
    if len(content) == 0:
        raise HTTPException(status_code=422, detail="空文件")

    if not _semaphore.acquire(timeout=MAX_REQUEST_TIMEOUT_S):
        raise HTTPException(status_code=429, detail="并发解析超限，请稍后重试")
    try:
        return await asyncio.to_thread(
            adapter.parse,
            file_name=file_name,
            content=content,
            options=parse_options,
            docling_version=DOCLING_VERSION,
        )
    except DocumentParserError as exc:
        _log.warning("解析失败 %s: %s", file_name, exc.message)
        raise HTTPException(status_code=422, detail=f"{exc.code}: {exc.message}") from exc
    finally:
        _semaphore.release()


def _is_supported_document(file_name: str) -> bool:
    lower = file_name.lower()
    return lower.endswith((".pdf", ".docx", ".xlsx", ".pptx", ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".tif", ".gif", ".webp"))


_IMAGE_SIGNATURES: list[tuple[bytes, str]] = [
    (b"\x89PNG\r\n\x1a\n", "png"),
    (b"\xff\xd8\xff", "jpeg"),
    (b"RIFF", "webp"),
    (b"GIF8", "gif"),
    (b"BM", "bmp"),
]


def _looks_like_image(content: bytes) -> bool:
    """按魔数判断常见图片格式，禁止伪造格式。"""
    for signature, _name in _IMAGE_SIGNATURES:
        if content.startswith(signature):
            return True
    return False


@app.post("/v1/ocr", response_model=schemas.OcrResult)
async def ocr_image(
    request: Request,
    file: UploadFile = File(...),  # noqa: B008
    languages: str = Form('["zh", "en"]'),  # noqa: B008
) -> schemas.OcrResult:
    """识别单张图片中的文字（RapidOCR）。

    multipart 字段：
      - file: PNG/JPEG/WebP/GIF/BMP 图片字节；
      - languages: 语言代码 JSON 数组（仅元信息，模型固定为中英混合）。
    响应为 OcrResult JSON。图片无法解码或 OCR 失败时返回空 lines（不报错）。
    """
    if not _ready.is_set():
        raise HTTPException(status_code=503, detail="document-parser 正在初始化模型")
    if _init_error is not None:
        raise HTTPException(status_code=503, detail="document-parser 模型初始化失败")

    content = await _read_limited(request, file, MAX_ASSET_BYTES)
    if len(content) == 0:
        raise HTTPException(status_code=422, detail="空图片")
    if not _looks_like_image(content):
        raise HTTPException(status_code=422, detail="不支持的图片格式")

    try:
        langs = _parse_languages(languages)
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=f"languages 无效: {exc}") from exc

    if not _semaphore.acquire(timeout=MAX_REQUEST_TIMEOUT_S):
        raise HTTPException(status_code=429, detail="并发解析超限，请稍后重试")
    try:
        lines = await asyncio.to_thread(adapter.ocr_image, content)
    finally:
        _semaphore.release()
    return schemas.OcrResult(lines=lines, languages=langs, engine="rapidocr")


def _parse_languages(raw: str) -> list[str]:
    import json

    try:
        value = json.loads(raw)
    except Exception as exc:
        raise ValueError(f"JSON 解析失败: {exc}") from exc
    if not isinstance(value, list) or not value:
        raise ValueError("必须是非空数组")
    langs: list[str] = []
    for item in value:
        if not isinstance(item, str) or not item.strip():
            raise ValueError("语言代码必须是非空字符串")
        if item not in langs:
            langs.append(item)
    return langs


async def _read_limited(request: Request, file: UploadFile, limit: int) -> bytes:
    """读取上传文件并强制大小限制。"""
    try:
        data = await file.read(limit + 1)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"读取上传文件失败: {exc}") from exc
    if len(data) > limit:
        raise HTTPException(status_code=413, detail=f"文件超过大小限制 {limit} 字节")
    return data
