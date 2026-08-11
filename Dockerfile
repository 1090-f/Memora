FROM golang:1.25.0-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/memora-server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -o /out/memora-migrate ./cmd/migrate

# Build the Python parser environment once. The final image contains both the
# Go application and this environment; memora-server owns the parser process.
FROM python:3.11-slim AS parser
ENV UV_LINK_MODE=copy
COPY --from=ghcr.io/astral-sh/uv:0.8 /uv /uvx /bin/
WORKDIR /opt/memora/document-parser
COPY services/document-parser/pyproject.toml services/document-parser/uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev --no-install-project
COPY services/document-parser/app.py \
     services/document-parser/schemas.py \
     services/document-parser/docling_adapter.py ./

FROM python:3.11-slim
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUTF8=1 \
    TORCH_COMPILE_DISABLE=1 \
    HF_HOME=/models/huggingface \
    UV_CACHE_DIR=/tmp/uv-cache \
    UV_LINK_MODE=copy
RUN apt-get update && \
    apt-get install -y --no-install-recommends gosu && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --system memora && \
    useradd --system --gid memora --create-home memora && \
    mkdir -p /app /models/huggingface /tmp/uv-cache && \
    chown -R memora:memora /app /models/huggingface /tmp/uv-cache
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY --from=build /src/scripts/migrations ./scripts/migrations
COPY --from=parser /bin/uv /bin/uvx /usr/local/bin/
COPY --from=parser --chown=memora:memora /opt/memora/document-parser /app/services/document-parser
COPY --chmod=755 scripts/docker-entrypoint.sh /usr/local/bin/memora-entrypoint
ENTRYPOINT ["memora-entrypoint"]
CMD ["memora-server"]
