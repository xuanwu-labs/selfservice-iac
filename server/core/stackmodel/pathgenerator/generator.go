// Package pathgenerator implements the D29 layer-first Path Contract.
//
// PathGenerator is a PURE FUNCTION: given a StackMeta (layer/tenant/team/space/
// component/env) + a path_template (from layer_rule_set_versions.layers_json),
// it outputs the stack identity quadruple: repo_path + state_key + stack_id +
// terramate_tags. It does NOT access the DB — the caller resolves team slug
// and layer template before calling Generate.
//
// codegen (W2) MUST call this component; it MUST NOT string-concatenate paths.
package pathgenerator

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// StackMeta is the input to PathGenerator. All fields are slugs (human-readable
// identifiers), NOT database IDs. The caller (codegen) is responsible for
// resolving IDs to slugs before calling Generate:
//   - Team: from teams.slug (JOIN teams ON stacks.owner_team_id = teams.id)
//   - Tenant: from stacks.tenant_id (already TEXT slug)
//   - Space: from spaces.name (nullable; empty = no space segment)
type StackMeta struct {
	Layer     string // "global" | "middleware" | "application"
	Tenant    string // "platform-default" | "corp-a"
	Team      string // "team-a" | "dba" (empty for global/middleware layers)
	Space     string // "orders" (empty = no space segment, only for application)
	Component string // "vpc" | "rds" | "ecs"
	Env       string // "prod" | "dev" | "staging" | "dr"
}

// PathResult is the stack identity quadruple (D29). All four fields are
// generated atomically in one call to ensure consistency.
type PathResult struct {
	RepoPath      string   // "application/platform-default/team-a/orders/ecs-prod"
	StateKey      string   // default = RepoPath (MVP; can diverge in Phase 2)
	StackID       string   // "application-platform-default-team-a-orders-ecs-prod"
	TerramateTags []string // ["layer:application","tenant:platform-default",...]
}

// PathGenerator renders path templates into the identity quadruple.
// It holds no state — the same input always produces the same output
// (deterministic, required by D19 codegen determinism).
type PathGenerator struct{}

// NewPathGenerator returns a PathGenerator.
func NewPathGenerator() *PathGenerator { return &PathGenerator{} }

// Generate renders the path_template (Go text/template syntax) with StackMeta
// and derives the full identity quadruple.
//
// The template comes from layer_rule_set_versions.layers_json[path_template].
// Example templates (seed v1):
//
//	global:      "global/{{.component}}-{{.tenant}}-{{.env}}"
//	middleware:  "middleware/{{.tenant}}/{{.component}}-{{.env}}"
//	application: "application/{{.tenant}}/{{.team}}/{{if .space}}{{.space}}/{{end}}{{.component}}-{{.env}}"
func (g *PathGenerator) Generate(meta StackMeta, pathTemplate string) (*PathResult, error) {
	if pathTemplate == "" {
		return nil, fmt.Errorf("pathgenerator: path_template is empty for layer %q", meta.Layer)
	}

	// Render the path template. Seed templates use lowercase keys (e.g.
	// {{.tenant}}, {{.team}}) which don't match Go exported field names.
	// Pass a map with lowercase keys to bridge the gap.
	tmpl, err := template.New("path").Parse(pathTemplate)
	if err != nil {
		return nil, fmt.Errorf("pathgenerator: parse template %q: %w", pathTemplate, err)
	}
	var buf bytes.Buffer
	tmplData := map[string]string{
		"layer":     meta.Layer,
		"tenant":    meta.Tenant,
		"team":      meta.Team,
		"space":     meta.Space,
		"component": meta.Component,
		"env":       meta.Env,
	}
	if err := tmpl.Execute(&buf, tmplData); err != nil {
		return nil, fmt.Errorf("pathgenerator: execute template: %w", err)
	}
	repoPath := strings.TrimSuffix(buf.String(), "/")

	// Derive stack_id: replace "/" with "-" (filesystem-safe + human-readable).
	stackID := strings.ReplaceAll(repoPath, "/", "-")

	// Derive terramate_tags from metadata (all non-empty fields).
	tags := buildTags(meta)

	return &PathResult{
		RepoPath:      repoPath,
		StateKey:      repoPath, // MVP: state_key = repo_path (Phase 2 can diverge)
		StackID:       stackID,
		TerramateTags: tags,
	}, nil
}

// buildTags constructs the Terramate tags array from StackMeta. Only non-empty
// fields are included (e.g. global layer has no team/space).
func buildTags(meta StackMeta) []string {
	tags := []string{
		"layer:" + meta.Layer,
		"tenant:" + meta.Tenant,
		"env:" + meta.Env,
		"component:" + meta.Component,
	}
	if meta.Team != "" {
		tags = append(tags, "team:"+meta.Team)
	}
	if meta.Space != "" {
		tags = append(tags, "space:"+meta.Space)
	}
	return tags
}
