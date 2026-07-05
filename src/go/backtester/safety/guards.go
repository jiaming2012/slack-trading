package safety

import (
	"fmt"
	"math"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// HaltEngager is the narrow slice of the HaltController that a guard needs: the
// ability to trip an automatic halt. *HaltController satisfies it. Guards depend
// on this interface (not the concrete controller) so guard logic can be unit
// tested against a recording double.
type HaltEngager interface {
	EngageAuto(reason string) error
}

// --- Order-rejection-rate guard ---

type rejectionEvent struct {
	at       time.Time
	rejected bool
}

// RejectionRateGuard trips the halt when the fraction of rejected orders within
// a rolling window exceeds a configured threshold. A minimum sample count avoids
// tripping on a single early rejection.
type RejectionRateGuard struct {
	clock      Clock
	window     time.Duration
	threshold  float64
	minSamples int
	engager    HaltEngager

	mu     sync.Mutex
	events []rejectionEvent
}

// NewRejectionRateGuard constructs the guard. threshold is a fraction in (0,1];
// window is the rolling window; minSamples is the minimum number of observations
// in the window before the guard can trip.
func NewRejectionRateGuard(clock Clock, window time.Duration, threshold float64, minSamples int, engager HaltEngager) *RejectionRateGuard {
	return &RejectionRateGuard{
		clock:      clock,
		window:     window,
		threshold:  threshold,
		minSamples: minSamples,
		engager:    engager,
	}
}

func (g *RejectionRateGuard) Name() string { return "rejection-rate guard" }

// Observe records an order outcome (rejected or accepted) and then evaluates the
// window. It returns (tripped, reason). On trip it engages the halt.
func (g *RejectionRateGuard) Observe(rejected bool) (bool, string) {
	g.mu.Lock()
	now := g.clock.Now()
	g.events = append(g.events, rejectionEvent{at: now, rejected: rejected})
	g.prune(now)

	total := len(g.events)
	rejectedCount := 0
	for _, e := range g.events {
		if e.rejected {
			rejectedCount++
		}
	}
	g.mu.Unlock()

	if total < g.minSamples {
		return false, ""
	}

	rate := float64(rejectedCount) / float64(total)
	if rate > g.threshold {
		reason := fmt.Sprintf("%s: rejection rate %.0f%% over %d orders exceeds threshold %.0f%%", g.Name(), rate*100, total, g.threshold*100)
		g.engage(reason)
		return true, reason
	}
	return false, ""
}

func (g *RejectionRateGuard) prune(now time.Time) {
	cutoff := now.Add(-g.window)
	i := 0
	for i < len(g.events) && !g.events[i].at.After(cutoff) {
		i++
	}
	if i > 0 {
		g.events = g.events[i:]
	}
}

func (g *RejectionRateGuard) engage(reason string) {
	if g.engager != nil {
		_ = g.engager.EngageAuto(reason)
	}
}

// --- Fill-price-deviation guard ---

// FillDeviationGuard trips the halt when a fill's absolute price deviation from
// the order's expected price exceeds a configured fraction of the expected
// price. It is evaluated per fill; no window is needed.
type FillDeviationGuard struct {
	pct     float64
	engager HaltEngager
}

// NewFillDeviationGuard constructs the guard. pct is a fraction (e.g. 0.02 for
// 2%).
func NewFillDeviationGuard(pct float64, engager HaltEngager) *FillDeviationGuard {
	return &FillDeviationGuard{pct: pct, engager: engager}
}

func (g *FillDeviationGuard) Name() string { return "fill-deviation guard" }

// ObserveFill compares actual against expected and trips when the absolute
// deviation exceeds pct of expected. A non-positive expected price cannot be
// evaluated and never trips.
func (g *FillDeviationGuard) ObserveFill(expected, actual float64) (bool, string) {
	if expected <= 0 {
		return false, ""
	}
	dev := math.Abs(actual-expected) / expected
	if dev > g.pct {
		reason := fmt.Sprintf("%s: fill %.4f deviates %.2f%% from expected %.4f (threshold %.2f%%)", g.Name(), actual, dev*100, expected, g.pct*100)
		if g.engager != nil {
			_ = g.engager.EngageAuto(reason)
		}
		return true, reason
	}
	return false, ""
}

// --- Trades-per-hour sigma guard ---

// TradesPerHourGuard trips the halt when the current trades-per-hour rate
// exceeds the supplied historical mean by more than two standard deviations of
// the historical norm.
//
// Two cold-start protections keep the guard from tripping as an artifact
// rather than an anomaly:
//   - it never trips before minSamples trades sit in its rolling one-hour
//     window, and
//   - a degenerate historical norm (mean and σ both zero — no history was
//     supplied) leaves the guard UNARMED: it observes but never trips, logged
//     once at construction.
type TradesPerHourGuard struct {
	clock       Clock
	histMean    float64
	histStdDev  float64
	minSamples  int
	armed       bool
	engager     HaltEngager
	sigmaFactor float64

	mu     sync.Mutex
	trades []time.Time
}

// NewTradesPerHourGuard constructs the guard from the historical mean and
// standard deviation of trades-per-hour. minSamples is the minimum number of
// trades in the rolling window before the guard may trip. A degenerate norm
// (histMean == 0 && histStdDev == 0) yields an unarmed guard that observes but
// never trips.
func NewTradesPerHourGuard(clock Clock, histMean, histStdDev float64, minSamples int, engager HaltEngager) *TradesPerHourGuard {
	armed := !(histMean == 0 && histStdDev == 0)
	if !armed {
		log.Warnf("safety: trades-per-hour guard is UNARMED — no historical trades-per-hour norm supplied (mean=0, stddev=0). It will observe but never trip; set GUARD_TRADES_PER_HOUR_MEAN / GUARD_TRADES_PER_HOUR_STDDEV to arm it.")
	}
	return &TradesPerHourGuard{
		clock:       clock,
		histMean:    histMean,
		histStdDev:  histStdDev,
		minSamples:  minSamples,
		armed:       armed,
		engager:     engager,
		sigmaFactor: 2.0,
	}
}

func (g *TradesPerHourGuard) Name() string { return "trades-per-hour guard" }

// Armed reports whether a usable historical norm was supplied; an unarmed
// guard observes but never trips.
func (g *TradesPerHourGuard) Armed() bool { return g.armed }

// ObserveTrade records a trade at the current time and evaluates the last-hour
// rate against mean + 2σ. It returns (tripped, reason). It never trips while
// unarmed or while fewer than minSamples trades are in the window.
func (g *TradesPerHourGuard) ObserveTrade() (bool, string) {
	g.mu.Lock()
	now := g.clock.Now()
	g.trades = append(g.trades, now)

	cutoff := now.Add(-time.Hour)
	i := 0
	for i < len(g.trades) && !g.trades[i].After(cutoff) {
		i++
	}
	if i > 0 {
		g.trades = g.trades[i:]
	}
	windowCount := len(g.trades)
	rate := float64(windowCount)
	g.mu.Unlock()

	if !g.armed {
		return false, ""
	}

	if windowCount < g.minSamples {
		return false, ""
	}

	threshold := g.histMean + g.sigmaFactor*g.histStdDev
	if rate > threshold {
		reason := fmt.Sprintf("%s: current rate %.0f trades/hr exceeds mean %.2f + 2σ (%.2f)", g.Name(), rate, g.histMean, threshold)
		if g.engager != nil {
			_ = g.engager.EngageAuto(reason)
		}
		return true, reason
	}
	return false, ""
}

// --- Data-feed-staleness guard ---

// StalenessSignal reports the age of the most recent Tick from the Feed. The
// bool return is false when no fresh signal is available (no observations yet or
// no source wired), in which case the guard treats staleness as unknown and does
// not trip.
type StalenessSignal interface {
	LastTickAge() (time.Duration, bool)
}

// FeedStalenessGuard trips the halt when the most-recent-Tick age reported by
// the staleness signal exceeds a configured threshold. When no signal source is
// registered (signal == nil), the guard degrades gracefully to inactive: it
// never trips and never blocks the other guards.
type FeedStalenessGuard struct {
	threshold time.Duration
	signal    StalenessSignal
	engager   HaltEngager
}

// NewFeedStalenessGuard constructs the guard. A nil signal yields an inactive
// guard (graceful degradation for the case where feed-health-staleness is not
// wired).
func NewFeedStalenessGuard(threshold time.Duration, signal StalenessSignal, engager HaltEngager) *FeedStalenessGuard {
	return &FeedStalenessGuard{threshold: threshold, signal: signal, engager: engager}
}

func (g *FeedStalenessGuard) Name() string { return "feed-staleness guard" }

// Active reports whether a staleness signal source is wired.
func (g *FeedStalenessGuard) Active() bool { return g.signal != nil }

// Evaluate reads the staleness signal and trips when the last-Tick age exceeds
// the threshold. An absent signal source or an unavailable reading never trips.
func (g *FeedStalenessGuard) Evaluate() (bool, string) {
	if g.signal == nil {
		return false, ""
	}
	age, ok := g.signal.LastTickAge()
	if !ok {
		return false, ""
	}
	if age > g.threshold {
		reason := fmt.Sprintf("%s: last tick age %s exceeds threshold %s", g.Name(), age, g.threshold)
		if g.engager != nil {
			_ = g.engager.EngageAuto(reason)
		}
		return true, reason
	}
	return false, ""
}
