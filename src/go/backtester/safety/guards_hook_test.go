package safety

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

func installTestRegistry(t *testing.T, reg *GuardRegistry) {
	t.Helper()
	SetGuardRegistry(reg)
	t.Cleanup(func() { SetGuardRegistry(nil) })
}

// counterValue sums a named counter's series (across guard labels when
// guardName is empty, or the single matching series otherwise).
func counterValue(t *testing.T, name, guardName string) float64 {
	t.Helper()
	var total float64
	for _, p := range telemetry.Default.Snapshot() {
		if p.Name != name {
			continue
		}
		if guardName != "" && p.Labels["guard"] != guardName {
			continue
		}
		total += p.Value
	}
	return total
}

// With no registry installed every observation helper must be a no-op — the
// nil-hook inertness that keeps simulation and model-diff paths byte-for-byte
// unaffected by the guard wiring.
func TestGuardHooks_NilRegistryIsInert(t *testing.T) {
	SetGuardRegistry(nil)

	require.NotPanics(t, func() {
		for i := 0; i < 20; i++ {
			ObserveLiveOrderOutcome(true)
			ObserveLiveFillDeviation(100, 500)
			ObserveLiveTrade()
		}
		tripped, reason := EvaluateFeedStalenessGuard()
		require.False(t, tripped)
		require.Empty(t, reason)
	})
}

func TestGuardHooks_ObservationsFlowToGuardsAndTelemetry(t *testing.T) {
	telemetry.Init()

	clk := NewFakeClock(time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC))
	c := newTestController(t)
	cfg := GuardEnvConfig{
		Guards: GuardConfig{
			RejectionWindow:         time.Minute,
			RejectionThreshold:      0.5,
			RejectionMinSamples:     2,
			FillDeviationPct:        0.02,
			TradesPerHourMean:       0, // unarmed: observes, never trips
			TradesPerHourStdDev:     0,
			TradesPerHourMinSamples: 1,
			FeedStalenessThreshold:  30 * time.Second,
		},
		RejectionEnabled:     true,
		FillDeviationEnabled: true,
		TradesPerHourEnabled: true,
		FeedStalenessEnabled: true,
	}
	reg := BuildGuardRegistry(c, clk, cfg, fakeStaleness{age: 5 * time.Second, ok: true})
	installTestRegistry(t, reg)

	// One accepted outcome, one trade, one in-tolerance fill, one staleness
	// evaluation: observation counters move, no trips.
	ObserveLiveOrderOutcome(false)
	ObserveLiveTrade()
	ObserveLiveFillDeviation(100.0, 100.5)
	tripped, _ := EvaluateFeedStalenessGuard()
	require.False(t, tripped)

	require.Equal(t, 1.0, counterValue(t, "safety_guard_observations_total", "rejection-rate guard"))
	require.Equal(t, 1.0, counterValue(t, "safety_guard_observations_total", "trades-per-hour guard"))
	require.Equal(t, 1.0, counterValue(t, "safety_guard_observations_total", "fill-deviation guard"))
	require.Equal(t, 1.0, counterValue(t, "safety_guard_observations_total", "feed-staleness guard"))
	require.Equal(t, 0.0, counterValue(t, "safety_guard_trips_total", ""))
	require.False(t, c.Status().Engaged)

	// A wild fill deviation trips: trip counter increments and the shared
	// controller engages (source auto).
	ObserveLiveFillDeviation(100.0, 150.0)
	require.Equal(t, 1.0, counterValue(t, "safety_guard_trips_total", "fill-deviation guard"))
	require.True(t, c.Status().Engaged)
	require.Equal(t, SourceAuto, c.Status().Source)
}

func TestGuardHooks_DisabledGuardNeverObservesOrTrips(t *testing.T) {
	telemetry.Init()

	c := newTestController(t)
	cfg := GuardEnvConfig{
		Guards: GuardConfig{
			FillDeviationPct: 0.02,
		},
		FillDeviationEnabled: false, // GUARD_FILL_DEVIATION_PCT=off
	}
	reg := BuildGuardRegistry(c, NewFakeClock(time.Now()), cfg, nil)
	installTestRegistry(t, reg)

	// A deviation that would trip an enabled guard is silently dropped.
	ObserveLiveFillDeviation(100.0, 500.0)
	require.Equal(t, 0.0, counterValue(t, "safety_guard_observations_total", "fill-deviation guard"))
	require.Equal(t, 0.0, counterValue(t, "safety_guard_trips_total", "fill-deviation guard"))
	require.False(t, c.Status().Engaged)
}

func TestGuardHooks_StalenessTripThroughHook(t *testing.T) {
	telemetry.Init()

	c := newTestController(t)
	reg := &GuardRegistry{
		Controller:    c,
		FeedStaleness: NewFeedStalenessGuard(30*time.Second, fakeStaleness{age: 5 * time.Minute, ok: true}, c),
	}
	installTestRegistry(t, reg)

	tripped, reason := EvaluateFeedStalenessGuard()
	require.True(t, tripped)
	require.Contains(t, reason, "feed-staleness guard")
	require.Equal(t, 1.0, counterValue(t, "safety_guard_trips_total", "feed-staleness guard"))
	require.True(t, c.Status().Engaged)
}
