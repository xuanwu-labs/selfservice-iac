package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/xuanwu-labs/selfservice-iac/server/core/codegen"
)

// --- Fakes ----------------------------------------------------------------
//
// Each fake implements exactly one Pipeline collaborator interface. They are
// configurable via fields so individual tests can dial behaviour (success /
// failure / inspect call args) without a mock framework. All fakes are safe
// for concurrent use only when the test itself is single-threaded, which the
// Phase 1 synchronous Pipeline guarantees.

// fakeCodeGenerator records calls and returns the configured FileSet / error.
type fakeCodeGenerator struct {
	mu        sync.Mutex
	calls     int
	lastInput codegen.CodegenInput
	files     codegen.FileSet
	err       error
}

func (f *fakeCodeGenerator) Generate(_ context.Context, in codegen.CodegenInput) (codegen.FileSet, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastInput = in
	if f.err != nil {
		return nil, f.err
	}
	if f.files == nil {
		return codegen.FileSet{"stack/main.tf": []byte("# generated")}, nil
	}
	return f.files, nil
}

// fakeTerramateRunner records plan/apply calls and returns configured results.
type fakeTerramateRunner struct {
	mu          sync.Mutex
	planCalls   int
	applyCalls  int
	planResult  PlanResult
	planErr     error
	applyResult ApplyResult
	applyErr    error
}

func (f *fakeTerramateRunner) RunPlan(_ context.Context, _ string, _ []string) (PlanResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.planCalls++
	return f.planResult, f.planErr
}

func (f *fakeTerramateRunner) RunApplySavedPlan(_ context.Context, _ string, _ string) (ApplyResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applyCalls++
	return f.applyResult, f.applyErr
}

// fakeWorkspaceManager records WriteFiles calls and returns the configured SHA.
type fakeWorkspaceManager struct {
	mu        sync.Mutex
	calls     int
	lastFiles codegen.FileSet
	commitSHA string
	err       error
}

func (f *fakeWorkspaceManager) WriteFiles(_ context.Context, _ int64, files codegen.FileSet) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastFiles = files
	if f.err != nil {
		return "", f.err
	}
	return f.commitSHA, nil
}

// fakeDecisionRecorder captures approval decisions made by ApprovalService.
type fakeDecisionRecorder struct {
	mu        sync.Mutex
	decisions []recordedDecision
	err       error
}

type recordedDecision struct {
	RequestID int64
	Approver  string
	Decision  string
	Comment   string
}

func (f *fakeDecisionRecorder) RecordApprovalDecision(_ context.Context, requestID int64, approver, decision, comment string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.decisions = append(f.decisions, recordedDecision{requestID, approver, decision, comment})
	return nil
}

// memRequestStore is an in-memory RequestStore: it holds one request row and
// applies optimistic-lock semantics on UpdateStatus (version must match).
type memRequestStore struct {
	mu  sync.Mutex
	req Request
	// failOnUpdate, if set, makes the next UpdateStatus return this error
	// (for error-path tests).
	updateErr error
}

func (s *memRequestStore) GetRequest(_ context.Context, _ int64) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.req, nil
}

func (s *memRequestStore) UpdateStatus(_ context.Context, _ int64, status string, version int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updateErr != nil {
		err := s.updateErr
		s.updateErr = nil // one-shot
		return err
	}
	if s.req.Version != version {
		return errors.New("optimistic lock mismatch")
	}
	s.req.Status = status
	s.req.Version = version + 1
	return nil
}

// newTestPipeline assembles a Pipeline with fakes and returns the fakes alongside
// so a test can assert on their state after Execute runs.
func newTestPipeline(initialStatus string) (*Pipeline, *fakeCodeGenerator, *fakeTerramateRunner, *fakeWorkspaceManager, *memRequestStore, *MemEventRecorder) {
	cg := &fakeCodeGenerator{}
	tr := &fakeTerramateRunner{}
	ws := &fakeWorkspaceManager{commitSHA: "deadbeef"}
	store := &memRequestStore{req: Request{ID: 1, Status: initialStatus, Version: 1}}
	rec := &MemEventRecorder{}
	logger := NewEventLogger(rec)
	p := NewPipeline(cg, tr, ws, store, logger)
	return p, cg, tr, ws, store, rec
}

// --- Pipeline happy path --------------------------------------------------

// TestPipeline_SubmittedToPlanReady drives a fresh request from submitted
// all the way to pending_approval, asserting each stage ran exactly once and
// the request stopped at the approval wait point (Execute returns nil with
// status pending_approval, NOT auto-applying).
func TestPipeline_SubmittedToPlanReady(t *testing.T) {
	p, cg, tr, ws, store, rec := newTestPipeline(StatusSubmitted)

	if err := p.Execute(context.Background(), 1); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := store.req.Status; got != StatusPendingApproval {
		t.Errorf("final status = %q, want %q", got, StatusPendingApproval)
	}
	if cg.calls != 1 {
		t.Errorf("codegen called %d times, want 1", cg.calls)
	}
	if ws.calls != 1 {
		t.Errorf("workspace.WriteFiles called %d times, want 1", ws.calls)
	}
	if tr.planCalls != 1 {
		t.Errorf("terramate.RunPlan called %d times, want 1", tr.planCalls)
	}
	if tr.applyCalls != 0 {
		t.Errorf("terramate.RunApplySavedPlan called %d times, want 0 (no approval yet)", tr.applyCalls)
	}
	// 4 transitions: submit, gen_done, plan_done, request_approval.
	if got := rec.Count(); got != 4 {
		t.Errorf("logged %d events, want 4 (submit/gen_done/plan_done/request_approval)", got)
	}
}

// TestPipeline_ApplyAfterApproval starts at applying (post-approval) and
// confirms the apply + reconcile stages run and the request ends succeeded.
func TestPipeline_ApplyAfterApproval(t *testing.T) {
	p, _, tr, _, store, rec := newTestPipeline(StatusApplying)

	if err := p.Execute(context.Background(), 1); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := store.req.Status; got != StatusSucceeded {
		t.Errorf("final status = %q, want %q", got, StatusSucceeded)
	}
	if tr.applyCalls != 1 {
		t.Errorf("terramate.RunApplySavedPlan called %d times, want 1", tr.applyCalls)
	}
	// 2 transitions: apply_done, reconcile_done.
	if got := rec.Count(); got != 2 {
		t.Errorf("logged %d events, want 2 (apply_done/reconcile_done)", got)
	}
}

// TestPipeline_PendingApprovalNoOp confirms Execute on a pending_approval
// request does nothing — the request is waiting for a human, not the Pipeline.
func TestPipeline_PendingApprovalNoOp(t *testing.T) {
	p, cg, tr, ws, store, rec := newTestPipeline(StatusPendingApproval)

	if err := p.Execute(context.Background(), 1); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if store.req.Status != StatusPendingApproval {
		t.Errorf("status changed to %q, want pending_approval (no work to do)", store.req.Status)
	}
	if cg.calls != 0 || tr.planCalls != 0 || tr.applyCalls != 0 || ws.calls != 0 {
		t.Errorf("collaborators were called despite pending_approval wait")
	}
	if rec.Count() != 0 {
		t.Errorf("logged %d events, want 0", rec.Count())
	}
}

// --- Pipeline failure paths -----------------------------------------------

// TestPipeline_GenerateFails verifies a codegen error transitions the request
// to failed_retryable (gen_fail) and Execute returns a wrapped error.
func TestPipeline_GenerateFails(t *testing.T) {
	p, cg, _, _, store, rec := newTestPipeline(StatusGenerating)
	cg.err = errors.New("codegen boom")

	err := p.Execute(context.Background(), 1)
	if err == nil {
		t.Fatalf("Execute returned nil, want error from failed codegen")
	}
	if got := store.req.Status; got != StatusFailedRetryable {
		t.Errorf("final status = %q, want %q", got, StatusFailedRetryable)
	}
	last := rec.Last()
	if last == nil {
		t.Fatalf("expected a logged failure event, got none")
	}
	if last.FromStatus == nil || last.ToStatus == nil ||
		*last.FromStatus != StatusGenerating || *last.ToStatus != StatusFailedRetryable {
		t.Errorf("last event from/to = %v->%v, want generating->failed_retryable", last.FromStatus, last.ToStatus)
	}
}

// TestPipeline_PlanFails verifies a plan error transitions to failed_retryable.
func TestPipeline_PlanFails(t *testing.T) {
	p, _, tr, _, store, _ := newTestPipeline(StatusPlanning)
	tr.planErr = errors.New("plan boom")

	if err := p.Execute(context.Background(), 1); err == nil {
		t.Fatalf("Execute returned nil, want error from failed plan")
	}
	if got := store.req.Status; got != StatusFailedRetryable {
		t.Errorf("final status = %q, want %q", got, StatusFailedRetryable)
	}
}

// TestPipeline_ApplyTransientFail verifies a non-zero-exit apply error
// transitions to failed_retryable (transient, retryable).
func TestPipeline_ApplyTransientFail(t *testing.T) {
	p, _, tr, _, store, _ := newTestPipeline(StatusApplying)
	tr.applyResult = ApplyResult{ExitCode: 1, Stderr: "rate limited"}
	tr.applyErr = errors.New("exit status 1")

	if err := p.Execute(context.Background(), 1); err == nil {
		t.Fatalf("Execute returned nil, want error from failed apply")
	}
	if got := store.req.Status; got != StatusFailedRetryable {
		t.Errorf("final status = %q, want %q (transient)", got, StatusFailedRetryable)
	}
}

// TestPipeline_ApplyPermanentFail verifies an apply error with no structured
// result (binary missing, config problem) transitions to failed_terminal.
func TestPipeline_ApplyPermanentFail(t *testing.T) {
	p, _, tr, _, store, _ := newTestPipeline(StatusApplying)
	tr.applyErr = errors.New("terramate: binary not found")

	if err := p.Execute(context.Background(), 1); err == nil {
		t.Fatalf("Execute returned nil, want error from failed apply")
	}
	if got := store.req.Status; got != StatusFailedTerminal {
		t.Errorf("final status = %q, want %q (permanent)", got, StatusFailedTerminal)
	}
}

// --- ApprovalService ------------------------------------------------------

// TestApproval_Approve drives the full approval path: pending_approval ->
// applying, decision recorded, event logged.
func TestApproval_Approve(t *testing.T) {
	store := &memRequestStore{req: Request{ID: 42, Status: StatusPendingApproval, Version: 7}}
	rec := &MemEventRecorder{}
	logger := NewEventLogger(rec)
	drec := &fakeDecisionRecorder{}
	svc := NewApprovalService(store, logger, drec)

	if err := svc.Approve(context.Background(), 42, "alice"); err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	if got := store.req.Status; got != StatusApplying {
		t.Errorf("status = %q, want %q", got, StatusApplying)
	}
	if store.req.Version != 8 {
		t.Errorf("version = %d, want 8 (incremented after transition)", store.req.Version)
	}
	if len(drec.decisions) != 1 {
		t.Fatalf("recorded %d decisions, want 1", len(drec.decisions))
	}
	d := drec.decisions[0]
	if d.Approver != "alice" || d.Decision != ApprovalDecisionApproved || d.RequestID != 42 {
		t.Errorf("decision = %+v, want {alice approved 42}", d)
	}
	last := rec.Last()
	if last == nil || last.ToStatus == nil || *last.ToStatus != StatusApplying {
		t.Errorf("last event to = %v, want applying", last)
	}
}

// TestApproval_Reject drives reject: pending_approval -> rejected (terminal),
// with the reason recorded as the decision comment.
func TestApproval_Reject(t *testing.T) {
	store := &memRequestStore{req: Request{ID: 42, Status: StatusPendingApproval, Version: 3}}
	logger := NewEventLogger(&MemEventRecorder{})
	drec := &fakeDecisionRecorder{}
	svc := NewApprovalService(store, logger, drec)

	if err := svc.Reject(context.Background(), 42, "bob", "wrong env"); err != nil {
		t.Fatalf("Reject returned error: %v", err)
	}
	if got := store.req.Status; got != StatusRejected {
		t.Errorf("status = %q, want %q", got, StatusRejected)
	}
	if len(drec.decisions) != 1 || drec.decisions[0].Comment != "wrong env" {
		t.Errorf("decision = %+v, want comment 'wrong env'", drec.decisions)
	}
}

// TestApproval_WrongStatus confirms Approve on a non-pending_approval request
// returns ErrNotPendingApproval and does NOT touch state or record a decision.
func TestApproval_WrongStatus(t *testing.T) {
	store := &memRequestStore{req: Request{ID: 1, Status: StatusPlanReady, Version: 1}}
	logger := NewEventLogger(&MemEventRecorder{})
	drec := &fakeDecisionRecorder{}
	svc := NewApprovalService(store, logger, drec)

	err := svc.Approve(context.Background(), 1, "alice")
	if !errors.Is(err, ErrNotPendingApproval) {
		t.Fatalf("err = %v, want ErrNotPendingApproval", err)
	}
	if store.req.Status != StatusPlanReady {
		t.Errorf("status changed to %q, want unchanged plan_ready", store.req.Status)
	}
	if len(drec.decisions) != 0 {
		t.Errorf("recorded %d decisions, want 0 (guard failed before recording)", len(drec.decisions))
	}
}
