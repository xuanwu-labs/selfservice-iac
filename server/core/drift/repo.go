// Package drift: repo.go — MemDriftRepo, the Phase 1 in-memory DriftRepo (D3).
//
// Phase 2 will add a Postgres-backed migration (drift_runs / drift_records);
// until then, MemDriftRepo keeps the in-process history that the worker and
// its tests need. It is safe for concurrent use.
package drift

import (
	"context"
	"sync"
)

// DriftRecord is a single stored drift check outcome.
type DriftRecord struct {
	StackID  int64
	HasDrift bool
	Diff     string
}

// MemDriftRepo is the in-memory DriftRepo implementation (Phase 1).
type MemDriftRepo struct {
	mu      sync.Mutex
	records []DriftRecord
}

// Compile-time: MemDriftRepo satisfies DriftRepo.
var _ DriftRepo = (*MemDriftRepo)(nil)

// NewMemDriftRepo constructs an empty MemDriftRepo.
func NewMemDriftRepo() *MemDriftRepo {
	return &MemDriftRepo{}
}

// RecordRun appends the outcome for stackID.
func (m *MemDriftRepo) RecordRun(_ context.Context, stackID int64, hasDrift bool, diff string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, DriftRecord{
		StackID:  stackID,
		HasDrift: hasDrift,
		Diff:     diff,
	})
	return nil
}

// Records returns a snapshot copy of all recorded runs.
func (m *MemDriftRepo) Records() []DriftRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]DriftRecord, len(m.records))
	copy(out, m.records)
	return out
}
