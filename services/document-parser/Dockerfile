# Memora document-parser：PDF/DOCX → ParsedDocument
# 构建阶段使用固定 uv 版本与 uv.lock 实现可重复构建。

FROM python:3.11-slim AS base

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUTF8=1 \
    TORCH_COMPILE_DISABLE=1 \
    HF_HOME=/models/huggingface \
    UV_LINK_MODE=copy

COPY --from=ghcr.io/astral-sh/uv:0.8 /uv /uvx /bin/

WORKDIR /app

# 先复制依赖清单，最大化层缓存
COPY pyproject.toml uv.lock ./

RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev --no-install-project

COPY app.py schemas.py docling_adapter.py ./

EXPOSE 5001

HEALTHCHECK --interval=30s --timeout=5s --start-period=300s --retries=5 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:5001/health/ready', timeout=4)"

CMD ["uv", "run", "--no-dev", "uvicorn", "app:app", "--host", "0.0.0.0", "--port", "5001", "--workers", "1"]
