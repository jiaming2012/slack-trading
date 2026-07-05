package feedhealth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestFakeClockReportsSetTime verifies a FakeClock created with an initial
// time T0 and no further mutation returns exactly T0 from Now().
func TestFakeClockReportsSetTime(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(t0)

	assert.True(t, t0.Equal(clock.Now()))
}

// TestFakeClockAdvanceChangesNow verifies advancing a FakeClock by a
// duration D changes subsequent Now() calls to T0 + D.
func TestFakeClockAdvanceChangesNow(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(t0)

	clock.Advance(30 * time.Second)

	assert.True(t, t0.Add(30*time.Second).Equal(clock.Now()))
}

// TestFakeClockSetOverwritesTime verifies Set replaces the current time value.
func TestFakeClockSetOverwritesTime(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	t1 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	clock.Set(t1)

	assert.True(t, t1.Equal(clock.Now()))
}
