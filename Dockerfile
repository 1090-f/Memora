FROM golang:1.26.5-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/memora-api ./cmd/memora-api && \
    CGO_ENABLED=0 go build -trimpath -o /out/memora-worker ./cmd/memora-worker && \
    CGO_ENABLED=0 go build -trimpath -o /out/memora-migrate ./cmd/memora-migrate

FROM alpine:3.22

RUN addgroup -S memora && adduser -S -G memora memora
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY --from=build /src/migrations ./migrations
USER memora

CMD ["memora-api"]
