// Package utils provides platform-wide utility helpers.
//
// snowflake.go implements Twitter Snowflake ID generation (63-bit:
// 41ms timestamp + 5 datacenterID + 5 machineID + 12 sequence).
//
// Design rationale (see openspec/changes/platform-db-schema/design.md §1.2):
//   - App-generated (not DB IDENTITY) so multi-instance deployment only needs
//     distinct machineID/datacenterID per instance — no cross-instance
//     sequence coordination.
//   - Time-ordered → B-tree insert-friendly (unlike UUIDv4 random page splits).
//   - int64 → half the storage/index cost of UUID (128-bit).
//   - Does not leak increment info (unlike BIGSERIAL).
//
// The ID is the DB primary key (BIGINT). At the API/proto boundary it is
// formatted as string (JavaScript/JSON loses precision above 2^53).
package utils

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// epoch is the custom epoch (2020-01-01 00:00:00 UTC in ms). Using a
	// recent epoch extends the 41-bit timestamp lifespan to ~2088.
	epoch            = int64(1577808000000)
	machineIDBits    = 5  // machineID bit width
	datacenterIDBits = 5  // datacenterID bit width
	sequenceBits     = 12 // sequence bit width

	maxMachineID    = -1 ^ (-1 << machineIDBits)    // 31
	maxDatacenterID = -1 ^ (-1 << datacenterIDBits) // 31
	maxSequence     = -1 ^ (-1 << sequenceBits)     // 4095

	machineIDShift    = sequenceBits
	datacenterIDShift = sequenceBits + machineIDBits
	timestampShift    = sequenceBits + machineIDBits + datacenterIDBits
)

// ErrNotInitialized is returned by GenerateID when Init has not been called.
// In practice wire calls Init at startup; this guards against misuse.
var ErrNotInitialized = errors.New("snowflake: not initialized, call Init first")

// ErrClockMovedBackwards is returned when the system clock moves backward
// beyond the tolerance window. The caller MUST surface this (do not silently
// return 0 — a zero ID is a valid collision with an uninitialized row).
var ErrClockMovedBackwards = errors.New("snowflake: clock moved backwards")

// Snowflake is a single-node ID generator. Fields are guarded by mu for
// sequence/timestamp consistency under concurrency.
type Snowflake struct {
	mu            sync.Mutex
	lastTimestamp int64
	machineID     int64
	datacenterID  int64
	sequence      int64
}

// NewNode creates a Snowflake generator bound to (machineID, datacenterID).
// Returns an error if either ID is out of range [0, 31].
func NewNode(machineID, datacenterID int64) (*Snowflake, error) {
	if machineID < 0 || machineID > maxMachineID {
		return nil, fmt.Errorf("snowflake: machineID %d out of range [0, %d]", machineID, maxMachineID)
	}
	if datacenterID < 0 || datacenterID > maxDatacenterID {
		return nil, fmt.Errorf("snowflake: datacenterID %d out of range [0, %d]", datacenterID, maxDatacenterID)
	}
	return &Snowflake{
		machineID:    machineID,
		datacenterID: datacenterID,
	}, nil
}

// NextID generates the next unique int64 ID. Thread-safe.
func (s *Snowflake) NextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	ts := now - epoch

	if ts < s.lastTimestamp {
		// Clock moved backward. Refuse rather than risk duplicate IDs.
		// Ferret returns a generic error; we expose a typed one for the
		// caller to handle (typically: log + retry after a short sleep).
		return 0, fmt.Errorf("%w: last=%d now=%d delta=%dms",
			ErrClockMovedBackwards, s.lastTimestamp+epoch, now, s.lastTimestamp-ts)
	}

	if ts == s.lastTimestamp {
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			// Sequence exhausted in this ms — spin until next ms.
			// At 4096 IDs/ms (~4M/s) per node this is effectively unreachable
			// for a control-plane metadata DB, but correctness demands it.
			for now <= s.lastTimestamp+epoch {
				now = time.Now().UnixMilli()
			}
			ts = now - epoch
		}
	} else {
		s.sequence = 0
	}

	s.lastTimestamp = ts

	id := (ts << timestampShift) |
		(s.datacenterID << datacenterIDShift) |
		(s.machineID << machineIDShift) |
		s.sequence
	return id, nil
}

// defaultNode holds the process-wide generator. Populated by Init at startup
// (called from wire). Until Init runs, the pointer is nil and GenerateID
// returns ErrNotInitialized — this is a fail-loud guard against forgetting
// to wire Init (which would silently produce machineID=0 collisions in a
// multi-instance deployment).
var defaultNode atomic.Pointer[Snowflake]

// Init initializes the process-wide default generator. MUST be called once at
// startup (wire provider) before any GenerateID call. Single-instance dev uses
// (0, 0); multi-instance assigns distinct (machineID, datacenterID) per node.
// Calling Init more than once replaces the generator (useful in tests).
func Init(machineID, datacenterID int64) error {
	node, err := NewNode(machineID, datacenterID)
	if err != nil {
		return err
	}
	defaultNode.Store(node)
	return nil
}

// GenerateID returns the next ID from the default generator. Panics if Init
// was not called — this is intentional: forgetting Init is a startup bug that
// should surface immediately, not silently produce wrong IDs. All INSERT
// paths call this to populate BIGINT primary keys.
func GenerateID() int64 {
	node := defaultNode.Load()
	if node == nil {
		panic(ErrNotInitialized)
	}
	id, err := node.NextID()
	if err != nil {
		// Clock rollback is the only error path. We panic rather than return 0
		// because a zero ID would collide and corrupt data. The operator must
		// fix NTP sync; the process should restart after the clock stabilizes.
		panic(err)
	}
	return id
}

// IsInitialized reports whether Init has been called. Useful for tests.
func IsInitialized() bool { return defaultNode.Load() != nil }
