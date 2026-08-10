.PHONY: build run-server migrate bootstrap-admin reset-admin-password test fmt vet

build:
	go build ./cmd/...

run-server:
	go run ./cmd/server

migrate:
	go run ./cmd/migrate up

bootstrap-admin:
	go run ./cmd/migrate bootstrap-admin

reset-admin-password:
	go run ./cmd/migrate reset-admin-password

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal ./pkg

vet:
	go vet ./...
