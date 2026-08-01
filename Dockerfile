FROM golang:1.25.0-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/memora-server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -o /out/memora-worker ./cmd/worker && \
    CGO_ENABLED=0 go build -trimpath -o /out/memora-migrate ./cmd/migrate

FROM alpine:3.22
RUN addgroup -S memora && adduser -S -G memora memora
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY --from=build /src/scripts/migrations ./scripts/migrations
USER memora
CMD ["memora-server"]
