package safety

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/go/feedhealth"
)

// FeedHealthStalenessSignal adapts the feed-health-staleness capability's
// HeartbeatMonitor to the StalenessSignal interface the FeedStalenessGuard
// consumes. It reports the age of the most recent observed Tick for a given
// asset class. If the monitor has no observation for the asset class yet, the
// age is reported as unavailable so the guard degrades to inactive rather than
// tripping on a cold start.
//
// Wiring this adapter is optional: when feed-health-staleness is not present,
// the guard is constructed with a nil signal and stays inactive (graceful
// degradation).
type FeedHealthStalenessSignal struct {
	monitor    *feedhealth.HeartbeatMonitor
	assetClass string
	clock      Clock
}

// NewFeedHealthStalenessSignal wraps a feedhealth HeartbeatMonitor for the given
// asset class. A nil monitor yields a signal that is always unavailable, which
// keeps the guard inactive.
func NewFeedHealthStalenessSignal(monitor *feedhealth.HeartbeatMonitor, assetClass string, clock Clock) *FeedHealthStalenessSignal {
	if clock == nil {
		clock = RealClock{}
	}
	return &FeedHealthStalenessSignal{monitor: monitor, assetClass: assetClass, clock: clock}
}

// LastTickAge returns the elapsed time since the last observed Tick for the
// asset class. The bool is false (unavailable) when no monitor is wired or no
// Tick has been observed yet.
func (s *FeedHealthStalenessSignal) LastTickAge() (time.Duration, bool) {
	if s.monitor == nil {
		return 0, false
	}
	last, ok := s.monitor.LastObserved(s.assetClass)
	if !ok {
		return 0, false
	}
	age := s.clock.Now().Sub(last)
	if age < 0 {
		age = 0
	}
	return age, true
}
