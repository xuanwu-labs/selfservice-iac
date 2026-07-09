// Package auth: oidc.go — OIDC client placeholder (D43 / D10).
//
// zitadel/oidc is the selected OIDC library (D43). Concrete OIDC client
// logic (token verification, ID token introspection, JWKS fetch) lands with
// the identity module (iac-self-service-platform D10). This file locks the
// dependency into go.mod so it isn't pruned by `go mod tidy`.
package auth

import (
	// Lock zitadel/oidc dependency (D43). The real verifier lands in Phase 0+.
	_ "github.com/zitadel/oidc/v3"
)
