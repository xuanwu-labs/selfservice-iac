# Aether Platform Makefile
# Usage: make <target>

.PHONY: build test lint migrate-up migrate-down sqlc-gen wire-gen proto-gen clean

# Build all binaries
build:
	go build ./cmd/...

# Run tests (short mode: skip DB-dependent tests, no PG required)
test:
	go test -race -short ./...

# Run ALL tests including DB-dependent ones (requires Docker via DOCKER_HOST)
test-db:
	go test -race -p 1 ./...

# Run golangci-lint (requires golangci-lint v2.x in PATH)
lint:
	golangci-lint run ./...

# Database migrations (reads config.yaml + env vars, same as server)
migrate-up:
	go run ./cmd/migrate -config config.yaml up

migrate-down:
	go run ./cmd/migrate -config config.yaml down

migrate-status:
	go run ./cmd/migrate -config config.yaml status

# Code generation
sqlc-gen:
	cd pkg/db && sqlc generate

wire-gen:
	wire gen ./cmd/...

proto-gen:
	cd ../contracts && buf generate
# Sync error-codes.yaml (contract source of truth) into the server binary via
# //go:embed in internal/asset. The copy is DO NOT EDIT — re-run make proto-gen
# after editing contracts/error-codes.yaml. See internal/asset/asset.go.
	cp ../contracts/error-codes.yaml internal/asset/error-codes.yaml

# Clean build artifacts
clean:
	rm -rf bin/
	go clean ./...
