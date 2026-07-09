// Package clock abstracts time so business logic can be tested deterministically.
//
// Per D44: drift detection (D5), task scheduling (D39), approval timeout (D21),
// and break-glass TTL (D30) all depend on time. Injecting a Clock lets tests
// control time without sleeping. All timestamps are stored as UTC timestamptz.
package clock

import "time"

// Clock is the minimal time abstraction (D44). Business logic receives a Clock
// and calls Now() instead of time.Now().
type Clock interface {
	// Now returns the current time, always in UTC.
	Now() time.Time
}

// realClock is the production implementation, wrapping time.Now.
type realClock struct{}

// New returns the production Clock (uses wall-clock time).
func New() Clock { return realClock{} }

func (realClock) Now() time.Time { return time.Now().UTC() }

// Fake is a controllable Clock for tests. Advance its time by setting Now_
// directly or calling Advance.
//
//	var fc clock.Fake
//	fc.Set(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
//	// ... run code under test ...
//	fc.Advance(5 * time.Minute)
type Fake struct {
	now time.Time
}

// NewFake returns a Fake clock anchored at t. If t is zero, anchors at the Unix epoch.
func NewFake(t time.Time) *Fake {
	return &Fake{now: t.UTC()}
}

// Now returns the fake's current time.
func (f *Fake) Now() time.Time { return f.now }

// Set jumps the fake clock to t (normalized to UTC).
func (f *Fake) Set(t time.Time) { f.now = t.UTC() }

// Advance moves the fake clock forward by d.
func (f *Fake) Advance(d time.Duration) { f.now = f.now.Add(d) }
