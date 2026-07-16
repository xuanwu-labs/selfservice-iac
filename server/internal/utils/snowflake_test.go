package utils

import (
	"sync"
	"testing"
	"time"
)

// TestNewNodeRange validates machineID/datacenterID bounds.
func TestNewNodeRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		machineID    int64
		datacenterID int64
		wantErr      bool
	}{
		{"zero ok", 0, 0, false},
		{"max ok", 31, 31, false},
		{"machine over", 32, 0, true},
		{"datacenter over", 0, 32, true},
		{"machine negative", -1, 0, true},
		{"datacenter negative", 0, -1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewNode(tc.machineID, tc.datacenterID)
			if tc.wantErr && err == nil {
				t.Fatalf("NewNode(%d,%d): want error, got nil", tc.machineID, tc.datacenterID)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("NewNode(%d,%d): want nil, got %v", tc.machineID, tc.datacenterID, err)
			}
		})
	}
}

// TestUniqueness verifies N sequential IDs are all distinct.
func TestUniqueness(t *testing.T) {
	t.Parallel()
	node, _ := NewNode(1, 1)
	const n = 10000
	seen := make(map[int64]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := node.NextID()
		if err != nil {
			t.Fatalf("NextID[%d]: %v", i, err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID %d at iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}

// TestMonotonic verifies IDs are time-ordered (each strictly greater than the
// previous). This is the B-tree insert-friendliness guarantee.
func TestMonotonic(t *testing.T) {
	t.Parallel()
	node, _ := NewNode(0, 0)
	var prev int64
	for i := 0; i < 1000; i++ {
		id, err := node.NextID()
		if err != nil {
			t.Fatalf("NextID[%d]: %v", i, err)
		}
		if id <= prev {
			t.Fatalf("ID not monotonic at %d: prev=%d curr=%d", i, prev, id)
		}
		prev = id
	}
}

// TestConcurrentSafety hammers NextID from many goroutines and confirms no
// duplicates under real parallelism.
func TestConcurrentSafety(t *testing.T) {
	t.Parallel()
	node, _ := NewNode(7, 3)
	const workers = 50
	const perWorker = 500
	var mu sync.Mutex
	seen := make(map[int64]struct{}, workers*perWorker)
	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]int64, 0, perWorker)
			for i := 0; i < perWorker; i++ {
				id, err := node.NextID()
				if err != nil {
					errOnce.Do(func() { firstErr = err })
					return
				}
				local = append(local, id)
			}
			mu.Lock()
			for _, id := range local {
				if _, dup := seen[id]; dup {
					t.Errorf("duplicate ID under concurrency: %d", id)
				}
				seen[id] = struct{}{}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("concurrent NextID error: %v", firstErr)
	}
	if got := len(seen); got != workers*perWorker {
		t.Fatalf("unique count = %d, want %d (lost %d to dups/errors)", got, workers*perWorker, workers*perWorker-got)
	}
}

// TestClockBackward simulates a clock rollback by manually rewinding
// lastTimestamp, then asserts NextID returns ErrClockMovedBackwards.
func TestClockBackward(t *testing.T) {
	t.Parallel()
	node, _ := NewNode(0, 0)
	// Produce one ID to set lastTimestamp.
	if _, err := node.NextID(); err != nil {
		t.Fatalf("seed NextID: %v", err)
	}
	// Force lastTimestamp into the future so the next real now appears backward.
	node.mu.Lock()
	node.lastTimestamp += 10_000 // 10s into the future
	node.mu.Unlock()

	_, err := node.NextID()
	if err == nil {
		t.Fatal("expected ErrClockMovedBackwards, got nil")
	}
	if !isClockBackward(err) {
		t.Fatalf("expected ErrClockMovedBackwards, got %v", err)
	}
}

// isClockBackward checks error chain without importing errors.Is semantics
// that would couple this test to the wrapping format.
func isClockBackward(err error) bool {
	return err != nil && err.Error() != "" && contains(err.Error(), "clock moved backwards")
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestInitGenerateID exercises the package-level Init + GenerateID path.
func TestInitGenerateID(t *testing.T) {
	// Not t.Parallel — mutates the global defaultNode.
	if err := Init(1, 1); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !IsInitialized() {
		t.Fatal("IsInitialized = false after Init")
	}
	id := GenerateID()
	if id <= 0 {
		t.Fatalf("GenerateID = %d, want positive", id)
	}
	id2 := GenerateID()
	if id2 <= id {
		t.Fatalf("second GenerateID %d not greater than first %d", id2, id)
	}
}

// TestGenerateIDPanicsIfNotInit confirms the fail-loud guard.
func TestGenerateIDPanicsIfNotInit(t *testing.T) {
	// Reset global to uninitialized state. Use a fresh package-level state by
	// storing nil — safe because this test does not run in parallel with others
	// that depend on the global.
	defaultNode.Store(nil)
	defer Init(0, 0) // restore for subsequent tests in the package

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when GenerateID called before Init")
		}
	}()
	_ = GenerateID()
}

// TestIDStructure confirms the bit layout matches the documented scheme
// (timestamp | datacenterID | machineID | sequence). Uses a node with
// known machine/datacenter IDs and verifies the encoded bits.
func TestIDStructure(t *testing.T) {
	t.Parallel()
	const machineID, datacenterID int64 = 5, 9
	node, _ := NewNode(machineID, datacenterID)
	id, err := node.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}

	// Decode the machine/datacenter bits.
	gotMachine := (id >> machineIDShift) & maxMachineID
	gotDatacenter := (id >> datacenterIDShift) & maxDatacenterID
	if gotMachine != machineID {
		t.Errorf("machineID bits: got %d want %d", gotMachine, machineID)
	}
	if gotDatacenter != datacenterID {
		t.Errorf("datacenterID bits: got %d want %d", gotDatacenter, datacenterID)
	}
}

// TestTimestampEmbedded confirms the timestamp encoded in the ID is within a
// reasonable window of "now" (sanity check that epoch math is correct).
func TestTimestampEmbedded(t *testing.T) {
	t.Parallel()
	node, _ := NewNode(0, 0)
	before := time.Now().UnixMilli()
	id, err := node.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	after := time.Now().UnixMilli()

	encoded := (id >> timestampShift) + epoch
	if encoded < before || encoded > after {
		t.Errorf("embedded timestamp %d not in [%d, %d]", encoded, before, after)
	}
}
