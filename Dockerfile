# 全局构建源开关：默认值与官方一致，海外 / CI 构建行为完全不变。
# 国内构建通过 docker compose 的 build.args 覆盖（见 deploy/docker-compose.yml 与 .env）。
ARG UV_IMAGE=ghcr.io/astral-sh/uv:0.8

# uv 二进制单独做成一个 stage，ghcr.io 不可达时可整体换源
FROM ${UV_IMAGE} AS uvbin

FROM golang:1.25.0-alpine AS build
# proxy.golang.org 在国内多数机器上超时；国内改成 https://goproxy.cn,direct
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
# cache id 必须写死：不写 id 时缓存键会绑到本步骤的 build args，
# 换源时调一下 GOPROXY 就等于清空缓存，go build 又要从零编译（实测 14 分钟）。
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /out/memora-server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -o /out/memora-migrate ./cmd/migrate

# Build the Python parser environment once. The final image contains both the
# Go application and this environment; memora-server owns the parser process.
FROM python:3.11-slim AS parser
ENV UV_LINK_MODE=copy
# PyPI 源与超时：国内指向镜像可显著提速（例 https://pypi.tuna.tsinghua.edu.cn/simple）
# UV_INDEX_URL 是已废弃的旧名，一并设置以兼容旧版 uv。
ARG UV_DEFAULT_INDEX=https://pypi.org/simple
ARG UV_HTTP_TIMEOUT=120
ARG UV_HTTP_RETRIES=5
# 使用 http（非 https）索引源时必须显式放行主机，例如阿里云 ECS 内网源
# http://mirrors.cloud.aliyuncs.com/pypi/simple/ 就要求 UV_INSECURE_HOST=mirrors.cloud.aliyuncs.com
ARG UV_INSECURE_HOST=
ENV UV_DEFAULT_INDEX=${UV_DEFAULT_INDEX} \
    UV_INDEX_URL=${UV_DEFAULT_INDEX} \
    UV_HTTP_TIMEOUT=${UV_HTTP_TIMEOUT} \
    UV_HTTP_RETRIES=${UV_HTTP_RETRIES} \
    UV_INSECURE_HOST=${UV_INSECURE_HOST}
COPY --from=uvbin /uv /uvx /bin/
WORKDIR /opt/memora/document-parser
COPY services/document-parser/pyproject.toml services/document-parser/uv.lock ./
# ⚠️ 必须写死 cache id：不写 id 时 BuildKit 会把缓存键绑到本步骤的
# build args 上，只要 GOPROXY / UV_* 这些值一变（换源、调超时），
# 之前下好的 wheel 全部作废、从头重下。写死 id 后跨构建累积复用。
#
# ⚠️⚠️ 关键：uv.lock 里每个包都记录了 registry（source = { registry = "https://pypi.org/simple" }），
# `uv sync --frozen` **严格按 lock 里记的源下载，UV_DEFAULT_INDEX 对它完全无效**。
# 所以只配环境变量没用 —— 国内部署必须在构建时把 lock 里的 registry 换掉。
# 这里用 build arg 自动完成（该 URL 在 lock 中只出现在 registry 行，替换等价、不动版本）；
# UV_LOCK_REGISTRY_TO 留空则完全不改动，海外/CI 行为与以前一模一样。
ARG UV_LOCK_REGISTRY_FROM=https://pypi.org/simple
ARG UV_LOCK_REGISTRY_TO=
RUN if [ -n "${UV_LOCK_REGISTRY_TO}" ] && [ "${UV_LOCK_REGISTRY_TO}" != "${UV_LOCK_REGISTRY_FROM}" ]; then \
        sed -i "s#${UV_LOCK_REGISTRY_FROM}#${UV_LOCK_REGISTRY_TO}#g" uv.lock && \
        echo "[build] uv.lock registry -> ${UV_LOCK_REGISTRY_TO}（已替换 $(grep -c "${UV_LOCK_REGISTRY_TO}" uv.lock) 处）"; \
    else \
        echo "[build] uv.lock registry 保持不变（未设置 UV_LOCK_REGISTRY_TO）"; \
    fi
#
# 注意：不要试图用容器跑 `uv lock --default-index` 来重生成 lock ——
# 官方 uv 镜像（ghcr.io/astral-sh/uv）是 distroless，没有 /bin/sh，
# uv 探不到 libc 会直接报 Failed to discover managed Python installations。
RUN --mount=type=cache,id=uv-cache,target=/root/.cache/uv \
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
# Debian 官方源在国内极慢（实测 apt-get install gosu 要 9~15 分钟）。
# 同时覆盖 sources.list 与 deb822 格式的 debian.sources，兼容不同基础镜像。
ARG APT_MIRROR=
RUN if [ -n "${APT_MIRROR}" ]; then \
        for f in /etc/apt/sources.list /etc/apt/sources.list.d/debian.sources; do \
            [ -f "$f" ] && sed -i "s|deb.debian.org|${APT_MIRROR}|g" "$f"; \
        done; \
        echo "[build] apt 源 -> ${APT_MIRROR}"; \
    fi
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
