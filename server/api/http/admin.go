// Package http: admin.go — Phase 1 admin REST endpoints.
//
// These endpoints back the AdminPage forms for which there is no proto RPC
// surface yet (state_backends credentials, workspaces remote_url). They are
// simple Gin handlers writing directly to the DB via the pool. The Connect
// interceptor chain (RBAC / audit) does NOT cover these — Phase 1 relies on
// the gin middleware chain + network-level admin boundary; Phase 2 will route
// admin config through a proper AdminService proto.
//
// Endpoints:
//
//	POST /admin/state-backends   upsert the default state backend credentials
//	POST /admin/workspaces       upsert the global workspace (infra-repo) row
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StateBackendPayload is the POST /admin/state-backends body. Mirrors the
// state_backends columns surfaced in the Admin UI; accessKey/secretKey are
// Phase 1 passthrough (stored on the row's credentials_ref as an opaque ref
// the operator must wire to Vault/KMS in Phase 2 — we never persist the raw
// secret to the DB).
type StateBackendPayload struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

// WorkspacePayload is the POST /admin/workspaces body. id=1 is the single
// global infra-repo workspace the platform operates on.
type WorkspacePayload struct {
	RemoteURL     string `json:"remoteUrl"`
	DefaultBranch string `json:"defaultBranch"`
}

// RegisterAdminRoutes mounts the admin REST handlers on the gin engine. The
// pool is required; pass nil to no-op (test isolation).
func RegisterAdminRoutes(r *gin.Engine, pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	r.POST("/admin/state-backends", saveStateBackend(pool))
	r.POST("/admin/workspaces", saveWorkspace(pool))
}

// saveStateBackend upserts the default state_backends row with the operator-
// supplied bucket/endpoint/region. Only the named (is_default=true) row is
// touched; accessKey/secretKey are validated non-empty in Phase 1 and recorded
// only in the credentials_ref column as a placeholder (the raw secret is never
// persisted — Phase 2 wires Vault/KMS).
func saveStateBackend(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body StateBackendPayload
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
			return
		}
		if body.Bucket == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bucket is required"})
			return
		}
		if body.AccessKey == "" || body.SecretKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "accessKey and secretKey are required"})
			return
		}
		name := body.Name
		if name == "" {
			name = "default"
		}
		kind := body.Kind
		if kind == "" {
			kind = "s3"
		}

		// Upsert the default backend row. credentials_ref is a placeholder ref
		// so the operator knows credentials were configured; the raw AK/SK are
		// NOT stored in the DB (Phase 2 resolves them via Vault/KMS).
		const sql = `INSERT INTO state_backends (id, name, kind, bucket, region, endpoint, is_default, credentials_ref)
			VALUES (0, $1, $2, $3, $4, $5, TRUE, 'admin-configured')
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				kind = EXCLUDED.kind,
				bucket = EXCLUDED.bucket,
				region = EXCLUDED.region,
				endpoint = EXCLUDED.endpoint,
				credentials_ref = 'admin-configured'`
		if _, err := pool.Exec(c.Request.Context(), sql, name, kind, body.Bucket, body.Region, body.Endpoint); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "save state backend: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "state backend saved (credentials not persisted; wire Vault/KMS in Phase 2)"})
	}
}

// saveWorkspace upserts the single global workspace row (id=1) holding the
// infra-repo remote_url + default_branch. codegen commits generated HCL back
// to this repo via go-git.
func saveWorkspace(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body WorkspacePayload
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
			return
		}
		if body.RemoteURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "remoteUrl is required"})
			return
		}
		branch := body.DefaultBranch
		if branch == "" {
			branch = "main"
		}

		const sql = `INSERT INTO workspaces (id, name, remote_url, default_branch)
			VALUES (1, 'infra', $1, $2)
			ON CONFLICT (id) DO UPDATE SET
				remote_url = EXCLUDED.remote_url,
				default_branch = EXCLUDED.default_branch`
		if _, err := pool.Exec(c.Request.Context(), sql, body.RemoteURL, branch); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "save workspace: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
