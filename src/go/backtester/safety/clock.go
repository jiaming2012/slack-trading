package safety

import "time"

// Clock is the injectable time source used by the rolling-window guards so their
// triggers are deterministically unit-testable. Production uses RealClock; tests
// use FakeClock.
type Clock interface {
	Now() time.Time
}

// RealClock reads the wall clock.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// FakeClock is a manually-advanced Clock for tests.
type FakeClock struct {
	t time.Time
}

// NewFakeClock starts a fake clock at t.
func NewFakeClock(t time.Time) *FakeClock { return &FakeClock{t: t} }

func (c *FakeClock) Now() time.Time { return c.t }

// Advance moves the fake clock forward by d.
func (c *FakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }
