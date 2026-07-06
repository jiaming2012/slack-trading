package safety

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// The guard registry is a package-level hook, mirroring models.SetOrderGate:
// wired once by main at process startup, nil everywhere else. Every
// observation helper below is nil-safe, so simulation runs, model-diff paths,
// and any process that never calls SetGuardRegistry are byte-for-byte
// unaffected — the guard feeds are completely inert until wired.
var (
	guardRegistryMu sync.RWMutex
	guardRegistry   *GuardRegistry
)

// SetGuardRegistry installs the process-wide guard registry. Passing nil
// removes it (used to reset in tests). Safe for concurrent use.
func SetGuardRegistry(r *GuardRegistry) {
	guardRegistryMu.Lock()
	defer guardRegistryMu.Unlock()
	guardRegistry = r
}

// ActiveGuardRegistry returns the installed registry, or nil when the guard
// feeds are unwired.
func ActiveGuardRegistry() *GuardRegistry {
	guardRegistryMu.RLock()
	defer guardRegistryMu.RUnlock()
	return guardRegistry
}

// BuildGuardRegistry constructs a GuardRegistry per the env-derived config,
// leaving disabled guards nil — a nil guard never observes and never trips,
// while the remaining guards operate normally. stalenessSignal may be nil
// (feed-staleness guard degrades to inactive).
func BuildGuardRegistry(controller *HaltController, clock Clock, cfg GuardEnvConfig, stalenessSignal StalenessSignal) *GuardRegistry {
	if clock == nil {
		clock = RealClock{}
	}

	reg := &GuardRegistry{Controller: controller}
	if cfg.RejectionEnabled {
		reg.RejectionRate = NewRejectionRateGuard(clock, cfg.Guards.RejectionWindow, cfg.Guards.RejectionThreshold, cfg.Guards.RejectionMinSamples, controller)
	}
	if cfg.FillDeviationEnabled {
		reg.FillDeviation = NewFillDeviationGuard(cfg.Guards.FillDeviationPct, controller)
	}
	if cfg.TradesPerHourEnabled {
		reg.TradesPerHour = NewTradesPerHourGuard(clock, cfg.Guards.TradesPerHourMean, cfg.Guards.TradesPerHourStdDev, cfg.Guards.TradesPerHourMinSamples, controller)
	}
	if cfg.FeedStalenessEnabled {
		reg.FeedStaleness = NewFeedStalenessGuard(cfg.Guards.FeedStalenessThreshold, stalenessSignal, controller)
	}
	return reg
}

// ObserveLiveOrderOutcome feeds one live (Paper/Margin pipeline) order outcome
// to the rejection-rate guard: rejected=true for a broker rejection,
// rejected=false for a committed fill. No-op when unwired or the guard is
// disabled.
func ObserveLiveOrderOutcome(rejected bool) {
	r := ActiveGuardRegistry()
	if r == nil || r.RejectionRate == nil {
		return
	}
	g := r.RejectionRate
	telemetry.GuardObservations.Add(1, telemetry.GuardLabel(g.Name()))
	if tripped, reason := g.Observe(rejected); tripped {
		recordGuardTrip(g.Name(), reason)
	}
}

// ObserveLiveFillDeviation feeds one live fill's requested-vs-actual price to
// the fill-deviation guard. Callers skip orders with no positive requested
// price; the guard also refuses expected <= 0. No-op when unwired or disabled.
func ObserveLiveFillDeviation(expectedPrice, actualPrice float64) {
	r := ActiveGuardRegistry()
	if r == nil || r.FillDeviation == nil {
		return
	}
	g := r.FillDeviation
	telemetry.GuardObservations.Add(1, telemetry.GuardLabel(g.Name()))
	if tripped, reason := g.ObserveFill(expectedPrice, actualPrice); tripped {
		recordGuardTrip(g.Name(), reason)
	}
}

// ObserveLiveTrade feeds one new live trade to the trades-per-hour guard.
// No-op when unwired or disabled.
func ObserveLiveTrade() {
	r := ActiveGuardRegistry()
	if r == nil || r.TradesPerHour == nil {
		return
	}
	g := r.TradesPerHour
	telemetry.GuardObservations.Add(1, telemetry.GuardLabel(g.Name()))
	if tripped, reason := g.ObserveTrade(); tripped {
		recordGuardTrip(g.Name(), reason)
	}
}

// EvaluateFeedStalenessGuard runs one feed-staleness evaluation through the
// installed registry (the ticker's per-cycle call). Each evaluation counts as
// an observation for the staleness guard. No-op when unwired or disabled.
func EvaluateFeedStalenessGuard() (bool, string) {
	r := ActiveGuardRegistry()
	if r == nil || r.FeedStaleness == nil {
		return false, ""
	}
	g := r.FeedStaleness
	telemetry.GuardObservations.Add(1, telemetry.GuardLabel(g.Name()))
	tripped, reason := g.Evaluate()
	if tripped {
		recordGuardTrip(g.Name(), reason)
	}
	return tripped, reason
}

// recordGuardTrip records a guard trip in telemetry and the log. The guard has
// already engaged the shared halt controller by the time this runs.
func recordGuardTrip(guardName, reason string) {
	telemetry.GuardTrips.Add(1, telemetry.GuardLabel(guardName))
	log.Errorf("safety: %s TRIPPED the kill switch — order submission halted (acknowledge + release to resume): %s", guardName, reason)
}
