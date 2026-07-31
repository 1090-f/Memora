.PHONY: test lint build

test:
	go test ./...

lint:
	go vet ./...

build:
	mkdir -p bin
	go build -o bin/memora-api ./cmd/memora-api
	go build -o bin/memora-worker ./cmd/memora-worker
	go build -o bin/memora-migrate ./cmd/memora-migrate
