.PHONY: dev dev-frontend test lint build docker-build docker-run clean sqlc migrate-up migrate-down migrate-create

CAIRN_ENCRYPTION_KEY ?= dev-key-32-bytes-minimum-length-here
DB_PATH ?= ./cairn.db

dev:
	CAIRN_ENCRYPTION_KEY=$(CAIRN_ENCRYPTION_KEY) go run ./cmd/cairn

dev-frontend:
	@echo "→ http://localhost:5173"
	cd frontend && npm run dev -- --open

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/cairn ./cmd/cairn

docker-build:
	docker build -t cairn:dev .

docker-run:
	docker run --rm -p 8080:8080 \
	  -e CAIRN_ENCRYPTION_KEY=$(CAIRN_ENCRYPTION_KEY) \
	  cairn:dev

sqlc:
	sqlc generate

migrate-up:
	goose -dir migrations sqlite3 $(DB_PATH) up

migrate-down:
	goose -dir migrations sqlite3 $(DB_PATH) down

migrate-create:
	@read -p "Migration name: " name; \
	goose -dir migrations create $$name sql

clean:
	rm -rf bin/ $(DB_PATH) $(DB_PATH)-*
