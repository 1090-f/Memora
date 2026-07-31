# Memora

Memora is a personal intelligent knowledge-base agent built around RAG, ReAct, Plan-Execute, MCP, and long-term memory.

The repository currently contains the **Foundation milestone only**: configuration, platform adapters, health endpoints, identity primitives, worker lifecycle, and local deployment. Product ingestion, retrieval, orchestration, and persistent job processing are not implemented yet.

## Requirements

- Go 1.26.5
- Docker with Docker Compose v2 (recommended for local development)

Copy `.env.example` to `.env` when you want to customize local values. The application reads these variables directly:

| Variable | Purpose |
| --- | --- |
| `MEMORA_ENVIRONMENT` | Runtime environment name |
| `MEMORA_HTTP_ADDRESS` | API listen address |
| `MEMORA_HTTP_READ_TIMEOUT`, `MEMORA_HTTP_WRITE_TIMEOUT`, `MEMORA_HTTP_SHUTDOWN_TIMEOUT` | HTTP lifecycle timeouts |
| `MEMORA_DATABASE_URL`, `MEMORA_DATABASE_MAX_OPEN`, `MEMORA_DATABASE_MAX_IDLE` | PostgreSQL connection and pool settings |
| `MEMORA_REDIS_ADDRESS`, `MEMORA_REDIS_PASSWORD`, `MEMORA_REDIS_DB` | Redis connection settings |
| `MEMORA_MINIO_ENDPOINT`, `MEMORA_MINIO_ACCESS_KEY`, `MEMORA_MINIO_SECRET_KEY`, `MEMORA_MINIO_BUCKET`, `MEMORA_MINIO_USE_SSL` | MinIO object-store settings |
| `MEMORA_JWT_SECRET`, `MEMORA_ACCESS_TTL` | Access-token signing and lifetime |

## Run locally

Start PostgreSQL, Redis, MinIO, initialize the bucket, apply migrations, and then start the API and worker:

```sh
docker compose --env-file .env.example -f deploy/docker-compose.yml up --build
```

The API listens on `http://localhost:8080`; liveness and readiness are available at `/live` and `/ready`. MinIO's console listens on `http://localhost:9001`.

Run migrations outside Compose with an exported `MEMORA_DATABASE_URL`:

```sh
go run ./cmd/memora-migrate up
go run ./cmd/memora-migrate down
```

## Verify

```sh
gofmt -w .
go vet ./...
go test -race ./...
go build ./cmd/...
docker compose --env-file .env.example -f deploy/docker-compose.yml config
```
