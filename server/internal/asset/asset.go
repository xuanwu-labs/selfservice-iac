// Package asset embeds static, build-time resources into the server binary
// via //go:embed. This mirrors the pattern used by cmd/migrate (which embeds
// migrations/*.sql): resources live as files on disk for editability and are
// compiled in for self-contained deployment.
//
// Synchronized resources (error-codes.yaml) are copied here by
// `make proto-gen` from ../../../contracts/ — DO NOT edit the copies in place;
// edit the source in contracts/ and re-run make proto-gen.
package asset

import _ "embed"

// ErrorCodes is the error-code registry YAML, embedded at build time.
// The source of truth is contracts/error-codes.yaml (repo root); this copy
// is synchronized by `make proto-gen`. Loaded once at startup by
// internal/errors.Load to build the in-memory Registry.
//
//go:embed error-codes.yaml
var ErrorCodes string
