package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xuanwu-labs/selfservice-iac/server/core/clock"
)

func TestRealClockReturnsUTCNow(t *testing.T) {
	c := clock.New()
	before := time.Now().UTC().Add(-time.Second)
	got := c.Now()
	after := time.Now().UTC().Add(time.Second)

	assert.True(t, got.Before(after), "Now() should not be ahead of wall clock")
	assert.True(t, got.After(before), "Now() should not be behind wall clock")
	assert.Equal(t, time.UTC, got.Location(), "Now() must return UTC")
}

func TestFakeClockSetAndNow(t *testing.T) {
	anchor := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	fc := clock.NewFake(anchor)

	require.Equal(t, anchor, fc.Now())
}

func TestFakeClockAdvance(t *testing.T) {
	anchor := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	fc := clock.NewFake(anchor)

	fc.Advance(5 * time.Minute)
	require.Equal(t, anchor.Add(5*time.Minute), fc.Now())

	fc.Advance(time.Hour)
	require.Equal(t, anchor.Add(5*time.Minute+time.Hour), fc.Now())
}

func TestFakeClockSetJumpsTime(t *testing.T) {
	fc := clock.NewFake(time.Unix(0, 0))

	future := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	fc.Set(future)
	require.Equal(t, future, fc.Now())
}

func TestFakeClockNormalizesToUTC(t *testing.T) {
	// Non-UTC input should be normalized to UTC equivalent.
	loc, _ := time.LoadLocation("America/New_York")
	ny := time.Date(2026, 1, 1, 12, 0, 0, 0, loc)
	fc := clock.NewFake(ny)

	assert.Equal(t, time.UTC, fc.Now().Location())
	assert.True(t, fc.Now().Equal(ny), "UTC-normalized time should represent the same instant")
}

func TestFakeImplementsClock(t *testing.T) {
	// Compile-time: Fake must satisfy the Clock interface.
	var c clock.Clock = clock.NewFake(time.Unix(0, 0))
	_ = c.Now()
}
