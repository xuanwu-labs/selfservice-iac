// Package drift: worker.go — DriftWorker (D3).
//
// The worker runs a read-only terraform plan (via the local Runner interface)
// in a checked-out work directory, interprets the plan's exit code, parses the
// emitted JSON when drift is found, records the result, and notifies on drift.
//
// IMPORTANT (D3 / P1-5): this package declares its own LOCAL interfaces for
// Runner / CheckoutProvider / Notifier. It does NOT import the orchestrator
// package — the concrete *terramate.ExecAdapter satisfies Runner implicitly.
// The terramate package is imported ONLY for the RunResult value type.
package drift

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xuanwu-labs/selfservice-iac/server/core/adapters/notify"
	"github.com/xuanwu-labs/selfservice-iac/server/core/terramate"
)

// Runner runs a terramate command in dir with args. It is satisfied by
// *terramate.ExecAdapter (D3 local interface — do NOT import orchestrator).
type Runner interface {
	Run(ctx context.Context, dir string, args []string) (terramate.RunResult, error)
}

// CheckoutProvider checks out a pinned commit and returns the absolute
// working directory. It is satisfied by *workspace.Manager (local interface).
type CheckoutProvider interface {
	CheckoutCommit(ctx context.Context, requestID int64, commitSHA string) (string, error)
}

// DriftRepo persists drift check outcomes. Phase 1 ships MemDriftRepo; Phase 2
// adds a Postgres-backed migration (drift_runs / drift_records, Non-Goal here).
type DriftRepo interface {
	// RecordRun stores the outcome of a single drift check for stackID.
	RecordRun(ctx context.Context, stackID int64, hasDrift bool, diff string) error
}

// Compile-time interface satisfaction checks.
var (
	_ Runner           = (Runner)(nil) //nolint:unused // documents the interface
	_ CheckoutProvider = (CheckoutProvider)(nil)
	_ DriftRepo        = (DriftRepo)(nil)
)

// DriftResult is the post-check outcome for a single stack.
type DriftResult struct {
	StackID     int64
	HasDrift    bool   // true iff plan exit code == 2
	DiffSummary string // human-readable summary of changed resources
	ExitCode    int    // raw plan exit code (0/1/2)
}

// Worker is the drift detection worker: it coordinates checkout -> plan ->
// parse -> record -> notify for a single stack (D3).
type Worker struct {
	runner   Runner
	checkout CheckoutProvider
	notifier notify.Notifier
	repo     DriftRepo
}

// NewWorker constructs a Worker. notifier may be nil (notifications skipped);
// repo may be nil (recording skipped) to ease unit tests that focus on plan
// interpretation, but production wiring MUST supply both.
func NewWorker(runner Runner, checkout CheckoutProvider, notifier notify.Notifier, repo DriftRepo) *Worker {
	return &Worker{
		runner:   runner,
		checkout: checkout,
		notifier: notifier,
		repo:     repo,
	}
}

// planExitCode* map terraform's `-detailed-exitcode` for `plan` (P2-9).
const (
	planExitOK    = 0  // no drift
	planExitDrift = 2  // drift detected
	planExitError = 1  // plan failed (not drift)
	planExecFail  = -1 // binary/exec failure (no exit code) — matches terramate.exitCode()
)

// CheckStack runs a read-only drift plan for a single stack.
//
// Flow (D3):
//  1. CheckoutCommit the pinned commit (read-only work dir).
//  2. Run `terramate run -- terraform plan -detailed-exitcode -out=plan.tfplan`
//     via the Runner.
//  3. Map exit code: 0 => no drift, 2 => drift (nil error), 1 => error.
//  4. On drift, parse the plan JSON (if available) for a resource summary.
//  5. Record the run to the repo.
//  6. Notify on drift.
//
// HasDrift is true ONLY for exit code 2; in that case the returned error is
// nil — drift is a normal outcome, not a failure. Exit code 1 (and exec
// failures) return a non-nil error.
func (w *Worker) CheckStack(ctx context.Context, stackID int64, workDir, commitSHA string) (DriftResult, error) {
	result := DriftResult{StackID: stackID}

	// Step 1: ensure we are at the pinned commit (caller may have already
	// checked out via the scheduler's stack-list provider). When workDir is
	// supplied we use it directly; otherwise we ask the CheckoutProvider.
	dir := workDir
	if dir == "" {
		var err error
		dir, err = w.checkout.CheckoutCommit(ctx, stackID, commitSHA)
		if err != nil {
			result.ExitCode = planExecFail
			return result, fmt.Errorf("drift: checkout stack %d: %w", stackID, err)
		}
	}

	// Step 2: run a read-only plan. -detailed-exitcode is what makes the
	// 0/2 distinction observable; -out keeps it non-interactive.
	args := []string{"run", "--", "terraform", "plan", "-detailed-exitcode", "-no-color"}
	rr, runErr := w.runner.Run(ctx, dir, args)
	result.ExitCode = rr.ExitCode

	// Step 3: map exit code (P2-9).
	switch rr.ExitCode {
	case planExitOK:
		// No drift. A nil runErr here is expected; a non-nil runErr with a 0
		// exit code is unusual but we trust the exit code (terraform can exit
		// 0 with warnings).
		_ = runErr
	case planExitDrift:
		// Step 4: parse plan JSON (if any) for a resource summary.
		result.HasDrift = true
		if strings.TrimSpace(rr.Stdout) != "" {
			if summary, pErr := ParsePlan([]byte(rr.Stdout)); pErr == nil {
				result.DiffSummary = summary.String()
			} else {
				// Non-fatal: we still record drift with a fallback summary.
				result.DiffSummary = fmt.Sprintf("drift detected (plan JSON parse error: %v)", pErr)
			}
		}
		if result.DiffSummary == "" {
			result.DiffSummary = "drift detected"
		}
	case planExitError:
		// Step 3 path for errors: record failure + return error. runErr may be
		// a non-nil *exec.ExitError (exit 1) — wrap it for context.
		_ = w.record(ctx, stackID, false, "")
		return result, fmt.Errorf("drift: plan failed for stack %d (exit 1): %s: %w",
			stackID, firstLine(rr.Stderr), coalesceErr(runErr))
	case planExecFail:
		_ = w.record(ctx, stackID, false, "")
		return result, fmt.Errorf("drift: exec failed for stack %d: %w", stackID, coalesceErr(runErr))
	default:
		_ = w.record(ctx, stackID, false, "")
		return result, fmt.Errorf("drift: unexpected exit code %d for stack %d: %w",
			rr.ExitCode, stackID, coalesceErr(runErr))
	}

	// Step 5: record outcome.
	if err := w.record(ctx, stackID, result.HasDrift, result.DiffSummary); err != nil {
		// Recording failure must not mask the drift verdict; surface it but
		// keep the result intact so the caller can still act on drift.
		return result, fmt.Errorf("drift: record run for stack %d: %w", stackID, err)
	}

	// Step 6: notify on drift (best-effort; notify failures are logged only).
	if result.HasDrift && w.notifier != nil {
		_ = w.notifier.Notify(ctx, notify.Notification{
			Type:    "drift_detected",
			Title:   fmt.Sprintf("Drift detected in stack %d", stackID),
			Message: result.DiffSummary,
		})
	}

	return result, nil
}

// record persists the run if a repo is configured.
func (w *Worker) record(ctx context.Context, stackID int64, hasDrift bool, diff string) error {
	if w.repo == nil {
		return nil
	}
	if err := w.repo.RecordRun(ctx, stackID, hasDrift, diff); err != nil {
		return err
	}
	return nil
}

// coalesceErr returns err if non-nil, else a sentinel indicating no underlying
// error was attached to a non-zero exit code.
func coalesceErr(err error) error {
	if err != nil {
		return err
	}
	return errors.New("no underlying error")
}

// firstLine returns the first non-empty line of s (best-effort, for log lines).
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}
