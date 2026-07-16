// Package audit contains boundary guard tests that enforce architectural
// invariants via static analysis (AST walk). These tests run in -short mode
// and need no external dependencies.
package audit

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenImportPrefixes lists Go import paths that MUST NOT appear in
// server/**. This is the D1 boundary: the platform calls terramate via exec
// (subprocess), never via Go import.
var forbiddenImportPrefixes = []string{
	"github.com/terramate-io/terramate",
}

// TestNoTerramateImports walks every .go file under the server module root
// and asserts that none import terramate internal packages. This closes the
// gap left by .golangci.yml depguard, which is non-enforcing when terramate
// is not in go.mod (the typechecker silently drops the rule).
//
// The test uses go/parser (not go/build) so it has zero runtime dependencies
// and runs in `go test -short` mode.
func TestNoTerramateImports(t *testing.T) {
	if testing.Short() {
		// Even in short mode, this test is cheap (AST parse only) and
		// enforces a critical invariant, so we keep it running.
	}

	// Resolve the server module root: this test file lives at
	// server/internal/audit/, so the module root is ../../.
	// We use filepath.Rel to be robust against symlinks.
	testFile, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve cwd: %v", err)
	}
	// Walk up from internal/audit/ to the server root (2 levels up).
	serverRoot := filepath.Join(testFile, "..", "..")

	violations := []string{}

	err = filepath.WalkDir(serverRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip non-Go files and vendor/testdata directories.
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testdata" || name == ".git" || name == "tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip this test file itself (it mentions the import path in a
		// string literal, not an import statement — but be safe).
		if strings.HasSuffix(path, "d1_guard_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			// Unparseable files are not our concern; skip.
			return nil
		}

		for _, imp := range file.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenImportPrefixes {
				if strings.HasPrefix(impPath, forbidden) {
					rel, _ := filepath.Rel(serverRoot, path)
					violations = append(violations, rel+" imports "+impPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk server root: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("D1 boundary violation: %d file(s) import forbidden packages:\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}
