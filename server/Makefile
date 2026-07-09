# Aether Platform Makefile
# Usage: make <target>

.PHONY: build test lint migrate-up migrate-down sqlc-gen wire-gen proto-gen clean

# Build all binaries
build:
	go build ./cmd/...

# Run tests (short mode: skip DB-dependent tests, no PG required)
test:
	go test -race -short ./...

# Run ALL tests including DB-dependent ones (requires AETHER_DB_DSN + live PostgreSQL)
test-db:
	go test -race -p 1 ./...

# Run golangci-lint (requires golangci-lint v2.x in PATH)
lint:
	golangci-lint run ./...

# Database migrations
# DB_DSN MUST be provided via env (e.g. export DB_DSN=postgres://user:pass@host:5432/dbname)
# Do NOT hardcode development DSNs here.
DB_DSN ?=

migrate-up:
	@if [ -z "$(DB_DSN)" ]; then echo "ERROR: DB_DSN is required (export DB_DSN=postgres://user:pass@host:5432/dbname)"; exit 1; fi
	go run ./cmd/migrate --dsn $(DB_DSN) up

migrate-down:
	@if [ -z "$(DB_DSN)" ]; then echo "ERROR: DB_DSN is required"; exit 1; fi
	go run ./cmd/migrate --dsn $(DB_DSN) down

migrate-status:
	@if [ -z "$(DB_DSN)" ]; then echo "ERROR: DB_DSN is required"; exit 1; fi
	go run ./cmd/migrate --dsn $(DB_DSN) status

# Code generation
sqlc-gen:
	cd pkg/db && sqlc generate

wire-gen:
	wire gen ./cmd/...

proto-gen:
	cd ../contracts && buf generate

# Clean build artifacts
clean:
	rm -rf bin/
	go clean ./...
