FROM node:24-alpine AS build

WORKDIR /workspace
RUN corepack enable

# npm 源：默认官方源；国内在 .env 里覆盖成 https://registry.npmmirror.com
ARG NPM_REGISTRY=https://registry.npmjs.org
# store-dir 固定到 /pnpm-store，配合下面的 cache mount 让已下载的包跨构建复用
RUN pnpm config set registry "${NPM_REGISTRY}" && pnpm config set store-dir /pnpm-store

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY admin/package.json admin/package.json
# cache id 写死，避免 build args 变化时缓存被换掉（同根目录 Dockerfile 里的道理）
RUN --mount=type=cache,id=pnpm-store,target=/pnpm-store pnpm install --frozen-lockfile

COPY admin admin
RUN pnpm --filter memora-admin build

FROM nginxinc/nginx-unprivileged:1.29-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /workspace/admin/dist /usr/share/nginx/html

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/ || exit 1
