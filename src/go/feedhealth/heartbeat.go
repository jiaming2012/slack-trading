package feedhealth

import (
	"sync"
	"time"
)

// HeartbeatMonitor tracks the last-observed tick timestamp per asset class
// and derives a FeedHealthStatus from elapsed time versus configured
// thresholds. All time reads flow through an injectable Clock so behavior is
// deterministic under test.
type HeartbeatMonitor struct {
	clock      Clock
	thresholds ThresholdConfig

	mu           sync.Mutex
	lastObserved map[string]time.Time
}

// NewHeartbeatMonitor constructs a HeartbeatMonitor driven by clock and
// configured with thresholds per asset class.
func NewHeartbeatMonitor(clock Clock, thresholds ThresholdConfig) *HeartbeatMonitor {
	return &HeartbeatMonitor{
		clock:        clock,
		thresholds:   thresholds,
		lastObserved: make(map[string]time.Time),
	}
}

// Observe records a tick for assetClass at time at, replacing any previously
// recorded timestamp for that asset class. Other asset classes are
// unaffected.
func (m *HeartbeatMonitor) Observe(assetClass string, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastObserved[assetClass] = at
}

// LastObserved returns the last-observed timestamp for assetClass and true,
// or the zero time and false if that asset class has never been observed.
func (m *HeartbeatMonitor) LastObserved(assetClass string) (time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.lastObserved[assetClass]
	return t, ok
}

// Status computes the current FeedHealthStatus for assetClass from elapsed
// time between the monitor's Clock.Now() and that asset class's
// LastObserved timestamp, compared against its configured thresholds. An
// asset class with no observation yet, or with no configured thresholds,
// reports Stale.
func (m *HeartbeatMonitor) Status(assetClass string) FeedHealthStatus {
	last, ok := m.LastObserved(assetClass)
	if !ok {
		return Stale
	}

	thresholds, ok := m.thresholds[assetClass]
	if !ok {
		return Stale
	}

	elapsed := m.clock.Now().Sub(last)

	switch {
	case elapsed <= thresholds.ExpectedInterval:
		return Healthy
	case elapsed <= thresholds.StalenessThreshold:
		return Degraded
	default:
		return Stale
	}
}

// ShouldPauseScanning returns true iff assetClass's current FeedHealthStatus
// is Stale. It re-evaluates fresh on every call — there is no latch, so a
// subsequent fresh Observe call clears the pause automatically on the next
// call, with no explicit acknowledgment or reset step.
func (m *HeartbeatMonitor) ShouldPauseScanning(assetClass string) bool {
	return m.Status(assetClass) == Stale
}
