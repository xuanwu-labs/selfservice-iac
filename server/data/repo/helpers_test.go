package repo_test

import (
	"fmt"
	"sync/atomic"
)

// envSuffix generates a unique suffix for test logical IDs to avoid collisions
// across tests in the same DB (GetByLogicalId uniqueness).
var envCounter uint64

func envSuffix() string {
	n := atomic.AddUint64(&envCounter, 1)
	return fmt.Sprintf("-%d", n)
}
