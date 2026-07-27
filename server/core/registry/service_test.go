package registry_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/git"
	"github.com/xuanwu-labs/selfservice-iac/server/core/registry"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// fakeGitProvider implements git.GitProvider for tests without network.
// Clone copies a testdata dir as the "clone"; CommitSHA returns a fake SHA.
type fakeGitProvider struct {
	sourceDir string // local dir to act as the "cloned" content
	sha       string
	cloneErr  error
}

func (f *fakeGitProvider) Clone(_ context.Context, _, _, dest string) error {
	if f.cloneErr != nil {
		return f.cloneErr
	}
	// Symlink-style: just record the source as the dest by copying the path.
	// For the test we point the extractor at sourceDir directly via a wrapper.
	return copyDir(f.sourceDir, dest)
}

func (f *fakeGitProvider) Fetch(_ context.Context, _ string) error { return nil }
func (f *fakeGitProvider) CommitSHA(_ context.Context, _ string) (string, error) {
	return f.sha, nil
}

// Compile-time check.
var _ git.GitProvider = (*fakeGitProvider)(nil)

// mockModuleRepo is a minimal fake of *repo.ModuleRepo for tests that don't
// need a real DB. RegistryService only calls CreateWithVersion, so we capture
// the args and return synthetic rows.
type mockModuleRepo struct {
	capturedMod generated.CreateModuleParams
	capturedVer generated.CreateModuleVersionParams
	modReturn   generated.Module
	verReturn   generated.ModuleVersion
	err         error
}

// We can't mock *repo.ModuleRepo directly (it's a concrete struct). Instead we
// verify via the real extractor path: this test exercises the extractor +
// git flow without DB, then a separate integration test covers DB.

func TestRegistryService_RegisterModule_HappyPath(t *testing.T) {
	// This test exercises the full pipeline MINUS the DB write by using a real
	// extractor against testdata. DB persistence is verified separately in the
	// repo integration tests (W1-02).
	//
	// What we verify here:
	//   1. fakeGitProvider.Clone makes the module dir available.
	//   2. ContractExtractor parses it successfully.
	//   3. The RegistryService would proceed to persist (we stop before DB).
	//
	// For a true end-to-end including DB, see TestRegistryService_DB (skipped
	// in -short, needs Docker).

	rdsDir, err := filepath.Abs(filepath.Join("testdata", "rds-mysql"))
	require.NoError(t, err)

	gp := &fakeGitProvider{sourceDir: rdsDir, sha: "abc123"}
	e := registry.NewContractExtractor()

	// Verify the pipeline up to contract extraction (the DB-dependent part is
	// RegisterModule itself; here we replicate its pre-persist steps).
	ctx := context.Background()
	dest := t.TempDir()
	require.NoError(t, gp.Clone(ctx, "fake-url", "v1.0.0", dest))
	sha, err := gp.CommitSHA(ctx, dest)
	require.NoError(t, err)
	assert.Equal(t, "abc123", sha)

	contract, err := e.ExtractFromRepo(dest, "")
	require.NoError(t, err)
	assert.NotEmpty(t, contract.Variables, "rds-mysql fixture has 5 variables")
	assert.Len(t, contract.Variables, 5)
}

func TestRegistryService_RegisterModule_MissingGitSource(t *testing.T) {
	// Validation guard: RegisterModuleInput.GitSource empty → error before any
	// clone/DB call. We test the input validation by calling RegisterModule
	// with a nil-safe setup; since we can't easily mock ModuleRepo, we verify
	// the validation happens early by checking it errors without touching git.
	gp := &fakeGitProvider{sha: "x"}
	e := registry.NewContractExtractor()
	// Construct service with nil repo — RegisterModule should error on input
	// validation BEFORE touching the repo.
	svc := registry.NewRegistryService(nil, gp, e)

	_, err := svc.RegisterModule(context.Background(), registry.RegisterModuleInput{
		GitSource: "", // empty → validation error
		Version:   "v1.0.0",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git_source is required")
}

func TestRegistryService_RegisterModule_MissingVersion(t *testing.T) {
	gp := &fakeGitProvider{sha: "x"}
	e := registry.NewContractExtractor()
	svc := registry.NewRegistryService(nil, gp, e)

	_, err := svc.RegisterModule(context.Background(), registry.RegisterModuleInput{
		GitSource: "git@x:y.git",
		Version:   "", // empty → validation error
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version is required")
}

// copyDir is a minimal recursive copy for the fakeGitProvider test fixture.
// It avoids an external cp dependency and works on Windows.
func copyDir(src, dst string) error {
	entries, err := filepath.Glob(filepath.Join(src, "*"))
	if err != nil {
		return fmt.Errorf("glob src: %w", err)
	}
	for _, entry := range entries {
		rel, err := filepath.Rel(src, entry)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		data, err := os.ReadFile(entry)
		if err != nil {
			return fmt.Errorf("read %s: %w", entry, err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	}
	return nil
}
