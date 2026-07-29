package drift_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/notify"
	"github.com/xuanwu-labs/selfservice-iac/server/core/drift"
	"github.com/xuanwu-labs/selfservice-iac/server/core/terramate"
)

// fakeRunner is a programmable Runner stub for worker tests.
type fakeRunner struct {
	result   terramate.RunResult
	runErr   error
	called   int
	lastDir  string
	lastArgs []string
}

func (f *fakeRunner) Run(_ context.Context, dir string, args []string) (terramate.RunResult, error) {
	f.called++
	f.lastDir = dir
	f.lastArgs = args
	return f.result, f.runErr
}

// fakeCheckout is a stub CheckoutProvider that yields a fixed work dir.
type fakeCheckout struct {
	dir string
	err error
}

func (f *fakeCheckout) CheckoutCommit(_ context.Context, _ int64, _ string) (string, error) {
	return f.dir, f.err
}

// captureNotifier collects notifications for assertions.
type captureNotifier struct {
	events []notify.Notification
}

func (n *captureNotifier) Notify(_ context.Context, event notify.Notification) error {
	n.events = append(n.events, event)
	return nil
}

func newWorker(r drift.Runner) (*drift.Worker, *drift.MemDriftRepo, *captureNotifier) {
	repo := drift.NewMemDriftRepo()
	notifier := &captureNotifier{}
	return drift.NewWorker(r, &fakeCheckout{dir: "/tmp/work"}, notifier, repo), repo, notifier
}

func TestWorker_ExitZero_NoDrift(t *testing.T) {
	runner := &fakeRunner{result: terramate.RunResult{ExitCode: 0}}
	worker, repo, notifier := newWorker(runner)

	res, err := worker.CheckStack(context.Background(), 42, "/tmp/work", "deadbeef")
	require.NoError(t, err)

	assert.Equal(t, int64(42), res.StackID)
	assert.False(t, res.HasDrift, "exit 0 must report no drift")
	assert.Equal(t, 0, res.ExitCode)

	records := repo.Records()
	require.Len(t, records, 1)
	assert.False(t, records[0].HasDrift)
	assert.Empty(t, notifier.events, "no notification on no-drift")
}

func TestWorker_ExitTwo_DriftDetected(t *testing.T) {
	planJSON := []byte(`{"resource_changes":[
		{"address":"alicloud_db_instance.this","change":{"actions":["update"]}},
		{"address":"alicloud_vpc.main","change":{"actions":["create"]}}
	]}`)
	runner := &fakeRunner{result: terramate.RunResult{
		ExitCode: 2,
		Stdout:   string(planJSON),
	}}
	worker, repo, notifier := newWorker(runner)

	res, err := worker.CheckStack(context.Background(), 7, "/tmp/work", "cafebabe")
	require.NoError(t, err, "drift is a normal outcome, not an error")

	assert.True(t, res.HasDrift, "exit 2 must report drift")
	assert.Equal(t, 2, res.ExitCode)
	assert.Contains(t, res.DiffSummary, "alicloud_db_instance.this:update")

	require.Len(t, repo.Records(), 1)
	assert.True(t, repo.Records()[0].HasDrift)
	assert.Equal(t, repo.Records()[0].Diff, res.DiffSummary)

	require.Len(t, notifier.events, 1, "drift must notify")
	assert.Equal(t, "drift_detected", notifier.events[0].Type)
	assert.Contains(t, notifier.events[0].Message, "alicloud_db_instance.this")
}

func TestWorker_ExitOne_ErrorRecorded(t *testing.T) {
	runner := &fakeRunner{
		result: terramate.RunResult{
			ExitCode: 1,
			Stderr:   "Error: something blew up\n",
		},
		runErr: errors.New("exit status 1"),
	}
	worker, repo, notifier := newWorker(runner)

	res, err := worker.CheckStack(context.Background(), 9, "/tmp/work", "abc")
	require.Error(t, err, "exit 1 must surface an error")
	assert.Contains(t, err.Error(), "something blew up")

	assert.False(t, res.HasDrift, "error path must NOT report drift")
	assert.Equal(t, 1, res.ExitCode)

	require.Len(t, repo.Records(), 1, "error outcome must still be recorded")
	assert.False(t, repo.Records()[0].HasDrift)
	assert.Empty(t, notifier.events, "errors do not notify")
}

func TestWorker_CheckoutFailureReturnsError(t *testing.T) {
	runner := &fakeRunner{}
	checkout := &fakeCheckout{err: errors.New("git checkout failed")}
	repo := drift.NewMemDriftRepo()
	notifier := &captureNotifier{}
	worker := drift.NewWorker(runner, checkout, notifier, repo)

	// Empty workDir forces the worker to use the CheckoutProvider.
	res, err := worker.CheckStack(context.Background(), 1, "", "sha")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checkout")
	assert.Equal(t, 0, runner.called, "plan must not run after checkout failure")
	assert.False(t, res.HasDrift)
}

func TestWorker_NilRepoIsAllowed(t *testing.T) {
	runner := &fakeRunner{result: terramate.RunResult{ExitCode: 0}}
	// repo + notifier both nil to confirm the worker tolerates them.
	worker := drift.NewWorker(runner, &fakeCheckout{dir: "/x"}, nil, nil)

	res, err := worker.CheckStack(context.Background(), 1, "/x", "sha")
	require.NoError(t, err)
	assert.False(t, res.HasDrift)
}

func TestWorker_DriftWithoutPlanJSONStillSummary(t *testing.T) {
	// Exit 2 but no stdout JSON: the worker must still record drift with a
	// fallback summary and notify.
	runner := &fakeRunner{result: terramate.RunResult{ExitCode: 2}}
	worker, repo, notifier := newWorker(runner)

	res, err := worker.CheckStack(context.Background(), 1, "/tmp/work", "sha")
	require.NoError(t, err)
	assert.True(t, res.HasDrift)
	assert.Equal(t, "drift detected", res.DiffSummary)
	require.Len(t, repo.Records(), 1)
	require.Len(t, notifier.events, 1)
}

func TestExecAdapterSatisfiesRunner(t *testing.T) {
	// Compile-time: *terramate.ExecAdapter must satisfy drift.Runner (D3).
	var _ drift.Runner = (*terramate.ExecAdapter)(nil)
}
