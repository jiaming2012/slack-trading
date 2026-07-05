package safety

import "time"

// GuardConfig holds the static thresholds for the anomaly guards. Adaptive /
// per-regime tuning is explicitly out of scope for this change; thresholds are
// static configuration.
type GuardConfig struct {
	// RejectionWindow is the rolling window for the rejection-rate guard.
	RejectionWindow time.Duration
	// RejectionThreshold is the tripping fraction (0..1) of rejected orders.
	RejectionThreshold float64
	// RejectionMinSamples is the minimum orders in the window before the
	// rejection-rate guard can trip.
	RejectionMinSamples int

	// FillDeviationPct is the tripping fraction of expected price for the
	// fill-deviation guard (e.g. 0.02 for 2%).
	FillDeviationPct float64

	// TradesPerHourMean and TradesPerHourStdDev describe the historical
	// trades-per-hour norm for the trades-per-hour sigma guard.
	TradesPerHourMean   float64
	TradesPerHourStdDev float64

	// FeedStalenessThreshold is the maximum tolerated age of the most recent
	// Tick before the feed-staleness guard trips.
	FeedStalenessThreshold time.Duration
}

// GuardRegistry wires every anomaly guard to the same shared HaltController, so
// a trip from any guard engages the identical halt the manual kill switch uses
// and is reported through the same status surface. The staleness signal source
// is optional (nil ⇒ the staleness guard is inactive).
type GuardRegistry struct {
	Controller    *HaltController
	RejectionRate *RejectionRateGuard
	FillDeviation *FillDeviationGuard
	TradesPerHour *TradesPerHourGuard
	FeedStaleness *FeedStalenessGuard
}

// NewGuardRegistry builds all guards bound to controller. stalenessSignal may be
// nil, in which case the feed-staleness guard degrades to inactive.
func NewGuardRegistry(controller *HaltController, clock Clock, cfg GuardConfig, stalenessSignal StalenessSignal) *GuardRegistry {
	if clock == nil {
		clock = RealClock{}
	}
	return &GuardRegistry{
		Controller:    controller,
		RejectionRate: NewRejectionRateGuard(clock, cfg.RejectionWindow, cfg.RejectionThreshold, cfg.RejectionMinSamples, controller),
		FillDeviation: NewFillDeviationGuard(cfg.FillDeviationPct, controller),
		TradesPerHour: NewTradesPerHourGuard(clock, cfg.TradesPerHourMean, cfg.TradesPerHourStdDev, controller),
		FeedStaleness: NewFeedStalenessGuard(cfg.FeedStalenessThreshold, stalenessSignal, controller),
	}
}

// EvaluateFeedStaleness runs the feed-staleness guard once (it is time-driven,
// not observation-driven). It is safe to call on a ticker; when no signal is
// wired it is a no-op.
func (r *GuardRegistry) EvaluateFeedStaleness() (bool, string) {
	if r.FeedStaleness == nil {
		return false, ""
	}
	return r.FeedStaleness.Evaluate()
}
