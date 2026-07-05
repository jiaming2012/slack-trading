package feedhealth

import (
	"sync"
	"time"
)

// Clock abstracts time.Now() so staleness detection can be driven
// deterministically by tests via FakeClock, with no wall-clock sleeps.
type Clock interface {
	Now() time.Time
}

// RealClock delegates to the real wall clock via time.Now().
type RealClock struct{}

// Now returns the current wall-clock time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// FakeClock is a settable, advanceable Clock for deterministic unit tests.
// It is exported for reuse by later packages' tests (e.g. scanner-l1-l2).
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock returns a FakeClock initialized to t.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t}
}

// Now returns the FakeClock's current settable time value.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set overwrites the FakeClock's current time value.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Advance moves the FakeClock's current time value forward by d.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
