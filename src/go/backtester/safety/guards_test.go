package safety

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestController(t *testing.T) *HaltController {
	t.Helper()
	c, err := NewHaltController(NewMemoryHaltStore())
	require.NoError(t, err)
	return c
}

// --- Rejection-rate guard ---

func TestRejectionRateGuard_OverThresholdTrips(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	g := NewRejectionRateGuard(clk, time.Minute, 0.5, 4, c)

	// 3 rejected of 4 => 75% > 50% threshold.
	g.Observe(true)
	g.Observe(false)
	g.Observe(true)
	tripped, reason := g.Observe(true)

	require.True(t, tripped)
	require.Contains(t, reason, "rejection-rate guard")
	require.True(t, c.Status().Engaged)
	require.Equal(t, SourceAuto, c.Status().Source)
	require.ErrorIs(t, c.AllowOrder(), ErrHalted)
}

func TestRejectionRateGuard_AtOrUnderThresholdDoesNotTrip(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	g := NewRejectionRateGuard(clk, time.Minute, 0.5, 4, c)

	// 2 rejected of 4 => 50%, exactly at threshold (must NOT trip, strict >).
	g.Observe(true)
	g.Observe(false)
	g.Observe(true)
	tripped, _ := g.Observe(false)

	require.False(t, tripped)
	require.False(t, c.Status().Engaged)
}

func TestRejectionRateGuard_BelowMinSamplesDoesNotTrip(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	g := NewRejectionRateGuard(clk, time.Minute, 0.5, 4, c)

	// One rejection, 100% but below the 4-sample minimum.
	tripped, _ := g.Observe(true)
	require.False(t, tripped)
	require.False(t, c.Status().Engaged)
}

func TestRejectionRateGuard_WindowPrunesOldRejections(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	g := NewRejectionRateGuard(clk, time.Minute, 0.5, 4, c)

	// Two rejections, then advance past the window so they age out.
	g.Observe(true)
	g.Observe(true)
	clk.Advance(2 * time.Minute)

	// Four fresh accepted orders: the aged-out rejections must not count, so the
	// rate is 0% and the guard does not trip.
	g.Observe(false)
	g.Observe(false)
	g.Observe(false)
	tripped, _ := g.Observe(false)
	require.False(t, tripped, "aged-out rejections should not count toward the rate")
	require.False(t, c.Status().Engaged)
}

// --- Fill-deviation guard ---

func TestFillDeviationGuard_BeyondThresholdTrips(t *testing.T) {
	c := newTestController(t)
	g := NewFillDeviationGuard(0.02, c) // 2%

	// expected 100, actual 103 => 3% > 2%.
	tripped, reason := g.ObserveFill(100.0, 103.0)
	require.True(t, tripped)
	require.Contains(t, reason, "fill-deviation guard")
	require.True(t, c.Status().Engaged)
	require.Equal(t, SourceAuto, c.Status().Source)
}

func TestFillDeviationGuard_WithinToleranceDoesNotTrip(t *testing.T) {
	c := newTestController(t)
	g := NewFillDeviationGuard(0.02, c)

	// expected 100, actual 101.5 => 1.5% < 2%.
	tripped, _ := g.ObserveFill(100.0, 101.5)
	require.False(t, tripped)
	require.False(t, c.Status().Engaged)

	// Negative deviation of the same magnitude also below threshold.
	tripped, _ = g.ObserveFill(100.0, 98.5)
	require.False(t, tripped)
}

func TestFillDeviationGuard_NonPositiveExpectedNeverTrips(t *testing.T) {
	c := newTestController(t)
	g := NewFillDeviationGuard(0.02, c)
	tripped, _ := g.ObserveFill(0, 50)
	require.False(t, tripped)
	require.False(t, c.Status().Engaged)
}

// --- Trades-per-hour sigma guard ---

func TestTradesPerHourGuard_BeyondTwoSigmaTrips(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	// mean 3, std 1 => threshold = 3 + 2*1 = 5. The 6th trade in the hour trips.
	g := NewTradesPerHourGuard(clk, 3.0, 1.0, 1, c)

	var tripped bool
	var reason string
	for i := 0; i < 6; i++ {
		tripped, reason = g.ObserveTrade()
	}
	require.True(t, tripped)
	require.Contains(t, reason, "trades-per-hour guard")
	require.True(t, c.Status().Engaged)
	require.Equal(t, SourceAuto, c.Status().Source)
}

func TestTradesPerHourGuard_WithinTwoSigmaDoesNotTrip(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	g := NewTradesPerHourGuard(clk, 3.0, 1.0, 1, c) // threshold 5

	// 5 trades == threshold, strict > required, so no trip.
	var tripped bool
	for i := 0; i < 5; i++ {
		tripped, _ = g.ObserveTrade()
	}
	require.False(t, tripped)
	require.False(t, c.Status().Engaged)
}

// A degenerate historical norm (mean and σ both zero — no history supplied)
// must leave the guard unarmed: before this fix, threshold = 0 + 2*0 = 0 and
// the very first live trade (rate 1 > 0) tripped the kill switch as a pure
// cold-start artifact.
func TestTradesPerHourGuard_FirstTradeWithZeroSigmaHistoryNeverTrips(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	g := NewTradesPerHourGuard(clk, 0, 0, 1, c)

	require.False(t, g.Armed(), "zero mean and zero stddev must leave the guard unarmed")

	// The first trade must not trip — and neither may any number that follow.
	for i := 0; i < 100; i++ {
		tripped, _ := g.ObserveTrade()
		require.False(t, tripped)
	}
	require.False(t, c.Status().Engaged)
}

func TestTradesPerHourGuard_BelowMinSamplesDoesNotTripEvenBeyondTwoSigma(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	// mean 0.5, std 0.25 => threshold = 1. Every trade past the first exceeds
	// the threshold, but the 5-sample minimum must hold the guard back.
	g := NewTradesPerHourGuard(clk, 0.5, 0.25, 5, c)
	require.True(t, g.Armed())

	for i := 0; i < 4; i++ {
		tripped, _ := g.ObserveTrade()
		require.False(t, tripped, "trade %d is below the 5-sample minimum and must not trip", i+1)
	}
	require.False(t, c.Status().Engaged)

	// The 5th trade reaches the arming minimum; rate 5 > threshold 1 trips.
	tripped, reason := g.ObserveTrade()
	require.True(t, tripped, "once the minimum sample count is reached the guard must trip on an over-threshold rate")
	require.Contains(t, reason, "trades-per-hour guard")
	require.True(t, c.Status().Engaged)
	require.Equal(t, SourceAuto, c.Status().Source)
}

func TestTradesPerHourGuard_OldTradesAgeOutOfWindow(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	g := NewTradesPerHourGuard(clk, 3.0, 1.0, 1, c) // threshold 5

	// Five trades early, then advance past the hour so they age out.
	for i := 0; i < 5; i++ {
		g.ObserveTrade()
	}
	clk.Advance(61 * time.Minute)
	// Now a burst of 5 fresh trades: still not > 5.
	var tripped bool
	for i := 0; i < 5; i++ {
		tripped, _ = g.ObserveTrade()
	}
	require.False(t, tripped, "aged-out trades must not count toward the current rate")
	require.False(t, c.Status().Engaged)
}

// --- Feed-staleness guard ---

type fakeStaleness struct {
	age time.Duration
	ok  bool
}

func (f fakeStaleness) LastTickAge() (time.Duration, bool) { return f.age, f.ok }

func TestFeedStalenessGuard_StaleTrips(t *testing.T) {
	c := newTestController(t)
	g := NewFeedStalenessGuard(30*time.Second, fakeStaleness{age: 45 * time.Second, ok: true}, c)

	tripped, reason := g.Evaluate()
	require.True(t, tripped)
	require.Contains(t, reason, "feed-staleness guard")
	require.True(t, c.Status().Engaged)
	require.Equal(t, SourceAuto, c.Status().Source)
}

func TestFeedStalenessGuard_FreshDoesNotTrip(t *testing.T) {
	c := newTestController(t)
	g := NewFeedStalenessGuard(30*time.Second, fakeStaleness{age: 10 * time.Second, ok: true}, c)

	tripped, _ := g.Evaluate()
	require.False(t, tripped)
	require.False(t, c.Status().Engaged)
}

func TestFeedStalenessGuard_AbsentSignalDegradesToInactive(t *testing.T) {
	c := newTestController(t)
	g := NewFeedStalenessGuard(30*time.Second, nil, c)
	require.False(t, g.Active())

	tripped, _ := g.Evaluate()
	require.False(t, tripped)
	require.False(t, c.Status().Engaged)

	// A signal that has no reading yet (ok == false) is also inactive.
	g2 := NewFeedStalenessGuard(30*time.Second, fakeStaleness{ok: false}, c)
	tripped, _ = g2.Evaluate()
	require.False(t, tripped)
	require.False(t, c.Status().Engaged)
}

// --- Registry wiring: any guard trip halts subsequent orders (source=auto) ---

func TestGuardRegistry_AnyTripHaltsSubsequentOrders(t *testing.T) {
	clk := NewFakeClock(time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC))
	c := newTestController(t)
	cfg := GuardConfig{
		RejectionWindow:        time.Minute,
		RejectionThreshold:     0.5,
		RejectionMinSamples:    2,
		FillDeviationPct:       0.02,
		TradesPerHourMean:      3,
		TradesPerHourStdDev:    1,
		FeedStalenessThreshold: 30 * time.Second,
	}
	reg := NewGuardRegistry(c, clk, cfg, nil)

	require.NoError(t, c.AllowOrder(), "orders allowed before any trip")

	// Trip via the fill-deviation guard.
	tripped, _ := reg.FillDeviation.ObserveFill(100, 110)
	require.True(t, tripped)

	// Subsequent order submission is blocked identically to a manual halt,
	// reported as source=auto.
	err := c.AllowOrder()
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrHalted))
	require.Equal(t, SourceAuto, c.Status().Source)
	require.True(t, c.Status().AckRequired)
}

func TestGuardRegistry_NilStalenessSignalIsInactive(t *testing.T) {
	c := newTestController(t)
	reg := NewGuardRegistry(c, NewFakeClock(time.Now()), GuardConfig{FeedStalenessThreshold: time.Second}, nil)
	require.False(t, reg.FeedStaleness.Active())
	tripped, _ := reg.EvaluateFeedStaleness()
	require.False(t, tripped)
	require.False(t, c.Status().Engaged)
}
