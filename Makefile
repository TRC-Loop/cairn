.PHONY: dev dev-app dev-frontend dev-statuspage-seed test lint build frontend docker-build docker-run clean sqlc migrate-up migrate-down migrate-create

CAIRN_ENCRYPTION_KEY ?= dev-key-32-bytes-minimum-length-here
DB_PATH ?= ./cairn.db

dev:
	CAIRN_ENCRYPTION_KEY=$(CAIRN_ENCRYPTION_KEY) CAIRN_DB_PATH=$(DB_PATH) go run ./cmd/cairn

# Seed the local DB with a default status page + one component + one check so
# `make dev` + browser visits to http://localhost:8080/ render a populated page.
# Safe to re-run; INSERT OR IGNORE keeps it idempotent.
dev-statuspage-seed:
	@which sqlite3 >/dev/null || { echo "sqlite3 CLI required"; exit 1; }
	@sqlite3 $(DB_PATH) "INSERT OR IGNORE INTO status_pages (slug, title, description, is_default) VALUES ('main', 'Cairn Demo Status', 'Monitoring for your self-hosted services.', 1);"
	@sqlite3 $(DB_PATH) "INSERT OR IGNORE INTO components (name, description, display_order) VALUES ('Website', 'Public web frontend.', 0);"
	@sqlite3 $(DB_PATH) "INSERT OR IGNORE INTO components (name, description, display_order) VALUES ('API', 'JSON API.', 1);"
	@sqlite3 $(DB_PATH) "INSERT OR IGNORE INTO status_page_components (status_page_id, component_id, display_order) SELECT (SELECT id FROM status_pages WHERE slug='main'), id, display_order FROM components;"
	@echo "Seeded. Visit http://localhost:8080/"

dev-frontend:
	@echo "→ http://localhost:5173"
	cd frontend && npm run dev -- --open

# Run backend (:8080) + frontend (:5173) together and open the browser.
# Ctrl-C stops both.
dev-app:
	@command -v open >/dev/null 2>&1 && OPENCMD=open || OPENCMD=xdg-open; \
	CAIRN_ENCRYPTION_KEY=$(CAIRN_ENCRYPTION_KEY) CAIRN_DB_PATH=$(DB_PATH) go run ./cmd/cairn & BACK=$$!; \
	(cd frontend && npm run dev) & FRONT=$$!; \
	trap "kill $$BACK $$FRONT 2>/dev/null; wait 2>/dev/null" INT TERM EXIT; \
	sleep 2; \
	$$OPENCMD http://localhost:8080/ >/dev/null 2>&1 || true; \
	$$OPENCMD http://localhost:5173/ >/dev/null 2>&1 || true; \
	wait

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

frontend:
	cd frontend && npm install && npm run build

build: frontend
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
