// Package codegen implements the W2 MVP code generation engine. It is the
// single pipeline that converts a CodegenInput (form values + module contract
// + catalog defaults + cross-layer dependencies) into a Terraform/Terramate
// file tree, returned as an in-memory FileSet.
//
// generator.go is the orchestration entry point. It:
//  1. Calls PathGenerator (D29) to derive the stack identity quadruple
//     (repo_path / state_key / stack_id / terramate_tags). codegen MUST NOT
//     string-concatenate paths itself.
//  2. Runs the 5-stage parameter pipeline (pipeline.go, design D3) to produce
//     resolved variable values.
//  3. Renders each .tf / .hcl file from an embedded text/template.
//  4. Returns a FileSet (map[string][]byte) keyed by repo_path/filename.
//
// Determinism (design D19): the same CodegenInput always produces the same
// FileSet. There is no time.Now(), no RNG, no non-deterministic ordering:
// text/template ranges over maps in sorted-key order, slice order is preserved
// from the input, and FileSet keys use forward slashes (path.Join) on every OS.
package codegen

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"path"
	"sort"
	"sync"
	"text/template"

	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
	"github.com/xuanwu-labs/selfservice-iac/server/core/stackmodel/pathgenerator"
)

// templateFS holds the embedded .tmpl files. Templates are the single source
// of truth for HCL structure (design D2) and are NOT user-configurable in MVP.
//
//go:embed templates/*.tmpl
var templateFS embed.FS

// funcMap is the shared template FuncMap. renderHCL is the only custom
// function; it turns a Go value into HCL attribute syntax (see hcl.go).
var funcMap = template.FuncMap{
	"renderHCL": renderHCLValue,
}

// tmplCache caches parsed templates by filename. Parsing is deterministic and
// the embedded bytes never change, so caching is safe across goroutines.
var tmplCache sync.Map // map[string]*template.Template

// BackendConfig describes the Terraform remote state backend. It is read from
// the state_backends table by the orchestrator (doc 09 §6, design D6) — codegen
// never hardcodes a bucket name.
type BackendConfig struct {
	Kind   string // "s3" (MVP); future "oss", "gcs"
	Bucket string // state_backends.bucket
	Region string // state_backends.region
}

// DependencyRef is one cross-layer upstream dependency. Variables maps a
// contract variable name to the HCL expression that reads it from the upstream
// stack's remote_state (e.g. {"vpc_id": "data.terraform_remote_state.vpc.outputs.vpc_id"}).
type DependencyRef struct {
	Alias     string            // data block alias, also used as the module var binding target name
	StateKey  string            // upstream stack's state_key (PathGenerator output)
	Variables map[string]string // {contract_var: "data.terraform_remote_state.<alias>.outputs.<out>"}
}

// CodegenInput is everything Generate needs. The orchestrator (W2 task) is
// responsible for resolving DB rows into these Go-native fields before calling
// Generate; codegen performs no DB access.
type CodegenInput struct {
	Meta          pathgenerator.StackMeta // layer/tenant/team/space/component/env slugs
	PathTemplate  string                  // from layer_rule_set_versions.layers_json
	Contract      *registry.Contract      // module_versions.variables_contract_json
	Defaults      map[string]any          // catalog_items.defaults_json
	Governance    map[string]any          // platform-forced vars (tags/state_key-as-var); highest priority
	FormValues    map[string]any          // form_values_json (user input)
	Cardinality   string                  // "single" (default) | "map"
	Instances     []map[string]any        // map cardinality: [{name:"web",...},...]
	InstanceKey   string                  // map cardinality: key field name in each instance
	ModuleSource  string                  // pre-built: "git::ssh://...//atomic/ecs?ref=abc123"
	ModuleVersion string                  // "" for git, "1.0.0" for registry
	Backend       BackendConfig           // remote state backend
	Dependencies  []DependencyRef         // cross-layer upstream refs
	ComponentName string                  // module block name, e.g. "rds" | "ecs"
}

// FileSet is the generator's output: repo-relative path → file content.
// Persistence (git add/commit) is the workspace manager's job (design D1) —
// codegen never touches the filesystem, which makes output trivially testable
// via golden-file comparison.
type FileSet map[string][]byte

// Generator is the codegen entry point. It holds the PathGenerator (the only
// collaborator) and is otherwise stateless and safe for concurrent use.
type Generator struct {
	pg *pathgenerator.PathGenerator
}

// NewGenerator returns a Generator bound to the given PathGenerator.
func NewGenerator(pg *pathgenerator.PathGenerator) *Generator {
	return &Generator{pg: pg}
}

// Generate produces the full file tree for one stack (design D1 incremental:
// one catalog item → one stack directory, never a global re-generate).
//
// The ctx is accepted for future cancellation hooks; current MVP work is pure
// CPU and does not block, but the signature is stable so the orchestrator can
// wire cancellation without a later API break.
func (g *Generator) Generate(ctx context.Context, in CodegenInput) (FileSet, error) {
	// Honour cancellation even though rendering is fast; cheap to check and
	// keeps the contract honest.
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("codegen: cancelled before generation: %w", ctx.Err())
	default:
	}

	// Stage 1 — path identity. PathGenerator is the sole authority for
	// repo_path/state_key/stack_id/tags (D24/D29). codegen must not concat.
	pr, err := g.pg.Generate(in.Meta, in.PathTemplate)
	if err != nil {
		return nil, fmt.Errorf("codegen: path generation: %w", err)
	}

	// Stage 2 — parameter pipeline (design D3). Governance always wins.
	// P0-1 fix: state_key is platform metadata for backend.tf ONLY.
	// It MUST NOT be injected into module args (would cause "Unsupported
	// argument" in terraform plan for standard modules). Governance vars
	// from caller (tags, etc.) are still applied.
	resolved := resolveParams(in.Contract, in.Defaults, in.FormValues, in.Governance, in.Dependencies)

	// Remove state_key from resolved if it leaked in (defensive — it shouldn't
	// be there unless a module literally declares a variable named state_key,
	// which is extremely rare and would be wrong for standard modules).
	delete(resolved, "state_key")

	// P0-3 fix: for map cardinality, split resolved vars into per-instance
	// (from Instances' keys, bound via each.value.X) and shared (rest of
	// resolved, rendered directly). For single cardinality, all vars are shared.
	perInstanceFields := []string{}
	sharedVars := make(map[string]any, len(resolved))
	if in.Cardinality == "map" && len(in.Instances) > 0 {
		// Collect field names from the first instance (excluding InstanceKey).
		for k := range in.Instances[0] {
			if k != in.InstanceKey {
				perInstanceFields = append(perInstanceFields, k)
			}
		}
		sort.Strings(perInstanceFields) // deterministic (D19)
		// Shared vars = resolved minus per-instance fields.
		piSet := make(map[string]bool, len(perInstanceFields))
		for _, f := range perInstanceFields {
			piSet[f] = true
		}
		for k, v := range resolved {
			if !piSet[k] {
				sharedVars[k] = v
			}
		}
	} else {
		sharedVars = resolved
	}

	files := make(FileSet, 5)

	// Stage 3 — render. Each file is added to the FileSet under
	// repo_path/filename. path.Join guarantees POSIX separators on every OS,
	// so the same input yields identical keys on Linux and Windows (D19).
	if err := addFile(files, pr.RepoPath, "main.tf", map[string]any{
		"ComponentName":     in.ComponentName,
		"ModuleSource":      in.ModuleSource,
		"ModuleVersion":     in.ModuleVersion,
		"Cardinality":       in.Cardinality,
		"Instances":         in.Instances,
		"InstanceKey":       in.InstanceKey,
		"PerInstanceFields": perInstanceFields,
		"SharedVars":        sharedVars,
		// Keep Vars for backward compat (single cardinality uses it).
		"Vars": sharedVars,
	}); err != nil {
		return nil, fmt.Errorf("codegen: render main.tf: %w", err)
	}

	if err := addFile(files, pr.RepoPath, "backend.tf", map[string]any{
		"Backend":  in.Backend,
		"StateKey": pr.StateKey,
	}); err != nil {
		return nil, fmt.Errorf("codegen: render backend.tf: %w", err)
	}

	if err := addFile(files, pr.RepoPath, "stack.tm.hcl", map[string]any{
		"StackID": pr.StackID,
		"Tags":    pr.TerramateTags,
		"After":   afterPaths(in.Dependencies),
	}); err != nil {
		return nil, fmt.Errorf("codegen: render stack.tm.hcl: %w", err)
	}

	// outputs.tf only when the contract declares outputs (design: avoid an
	// empty file that would add git-diff noise).
	if in.Contract != nil && len(in.Contract.Outputs) > 0 {
		if err := addFile(files, pr.RepoPath, "outputs.tf", map[string]any{
			"Outputs":       in.Contract.Outputs,
			"Cardinality":   in.Cardinality,
			"ComponentName": in.ComponentName,
		}); err != nil {
			return nil, fmt.Errorf("codegen: render outputs.tf: %w", err)
		}
	}

	// cross-layer.tf only when there is at least one upstream dependency.
	if len(in.Dependencies) > 0 {
		if err := addFile(files, pr.RepoPath, "cross-layer.tf", map[string]any{
			"Dependencies": in.Dependencies,
			"Backend":      in.Backend,
		}); err != nil {
			return nil, fmt.Errorf("codegen: render cross-layer.tf: %w", err)
		}
	}

	return files, nil
}

// addFile renders a single template into the FileSet at repoPath/filename.
// It centralizes template lookup + execution so each file only differs by its
// data payload, not by boilerplate.
func addFile(files FileSet, repoPath, filename string, data any) error {
	content, err := renderTemplate(filename, data)
	if err != nil {
		return err
	}
	files[path.Join(repoPath, filename)] = content
	return nil
}

// renderTemplate parses (and caches) the named embedded template and executes
// it against data. All templates use Option("missingkey=error") so a typo'd
// template key is a hard error instead of silently rendering empty (W1-04 P1-4).
func renderTemplate(filename string, data any) ([]byte, error) {
	tmpl, err := getTemplate(filename)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute %s: %w", filename, err)
	}
	return buf.Bytes(), nil
}

// getTemplate returns a cached parsed template for filename, parsing it on
// first use. Parsing reads from the embedded FS, applies the shared FuncMap
// (renderHCL) and the missingkey=error option.
func getTemplate(filename string) (*template.Template, error) {
	if v, ok := tmplCache.Load(filename); ok {
		return v.(*template.Template), nil
	}
	// Embedded template files live under templates/<filename>.tmpl; the .tmpl
	// suffix keeps them out of Terraform's tooling (which would otherwise try
	// to parse them as real .tf) while still embedding their bytes.
	raw, err := templateFS.ReadFile("templates/" + filename + ".tmpl")
	if err != nil {
		return nil, fmt.Errorf("read embedded template %s: %w", filename, err)
	}
	tmpl, err := template.New(filename).
		Funcs(funcMap).
		Option("missingkey=error").
		Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse embedded template %s: %w", filename, err)
	}
	// Store the parsed result; concurrent stores resolve to the same value,
	// and Parse is deterministic so any winner is correct.
	tmplCache.Store(filename, tmpl)
	return tmpl, nil
}

// mergeGovernance returns a new governance map that overlays `extra` onto
// `base` (extra wins). Neither input is mutated. Used to fold the
// PathGenerator-derived state_key into the caller-provided governance set
// without leaking platform-forced values back into user-supplied maps.
func mergeGovernance(base, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// afterPaths derives the Terramate `after` list for stack.tm.hcl from the
// cross-layer dependencies. MVP uses the upstream state_key's parent path as a
// stable relative reference; the orchestrator may override by pre-building the
// After list when relative-path semantics get richer (Phase 2 watch/after).
//
// The result is sorted for deterministic output (D19) regardless of upstream
// dependency map iteration order.
func afterPaths(deps []DependencyRef) []string {
	if len(deps) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(deps))
	out := make([]string, 0, len(deps))
	for _, d := range deps {
		if d.StateKey == "" {
			continue
		}
		if _, dup := seen[d.StateKey]; dup {
			continue
		}
		seen[d.StateKey] = struct{}{}
		out = append(out, d.StateKey)
	}
	sort.Strings(out)
	return out
}
