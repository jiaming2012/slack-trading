package fidelity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spyAlerter records every RaiseFidelityAlert invocation so tests can assert the
// alert path fires exactly once per breaching strategy.
type spyAlerter struct {
	calls []Result
}

func (s *spyAlerter) RaiseFidelityAlert(_ context.Context, result Result) {
	s.calls = append(s.calls, result)
}

// --- Pairing determinism (task 9.1) ---

func TestPairTrades_DeterministicRegardlessOfOrder(t *testing.T) {
	set := ZeroDriftSet()

	pairsA, unmatchedA := PairTrades(set.Live, set.Sim)

	// Reverse both inputs; the resulting pair set must be identical.
	revLive := reversed(set.Live)
	revSim := reversed(set.Sim)
	pairsB, unmatchedB := PairTrades(revLive, revSim)

	require.Equal(t, pairsA, pairsB, "pairing must be independent of input order")
	require.Empty(t, unmatchedA.Live)
	require.Empty(t, unmatchedA.Sim)
	require.Equal(t, unmatchedA, unmatchedB)
	require.Len(t, pairsA, 3)
}

func TestPairTrades_UnmatchedReportedAndExcluded(t *testing.T) {
	live := []Trade{
		syntheticLiveBase("s1", "AAPL", 0),
		syntheticLiveBase("s1", "AAPL", 1), // no sim counterpart
	}
	sim := []Trade{
		syntheticLiveBase("s1", "AAPL", 0),
		syntheticLiveBase("s1", "AAPL", 2), // no live counterpart
	}

	pairs, unmatched := PairTrades(live, sim)

	require.Len(t, pairs, 1, "only the shared entry timestamp pairs")
	require.Len(t, unmatched.Live, 1)
	require.Len(t, unmatched.Sim, 1)
	assert.Equal(t, live[1], unmatched.Live[0])
	assert.Equal(t, sim[1], unmatched.Sim[0])

	// The unmatched trades must not affect drift: score of the single matched
	// (identical) pair is zero.
	comp := Score([]PairDelta{ComparePair(pairs[0])}, DefaultConfig())
	assert.Zero(t, comp.DriftScore)
}

// --- Per-pair comparison (task 9.2) ---

func TestComparePair_ZeroDrift(t *testing.T) {
	live := syntheticLiveBase("s", "AAPL", 0)
	d := ComparePair(Pair{Live: live, Sim: live})
	assert.Zero(t, d.PnLDelta)
	assert.Zero(t, d.FillDelta)
	assert.Zero(t, d.HoldDelta)
	assert.True(t, d.ExitReasonMatch)
}

func TestComparePair_PnLDeltaIsSimMinusLive(t *testing.T) {
	live := syntheticLiveBase("s", "AAPL", 0)
	sim := live
	sim.PnL = live.PnL + 7.5
	d := ComparePair(Pair{Live: live, Sim: sim})
	assert.InDelta(t, 7.5, d.PnLDelta, 1e-9)
}

func TestComparePair_ExitReasonMismatchFlagged(t *testing.T) {
	live := syntheticLiveBase("s", "AAPL", 0)
	live.ExitReason = ExitReasonStop
	sim := live
	sim.ExitReason = ExitReasonTarget
	d := ComparePair(Pair{Live: live, Sim: sim})
	assert.False(t, d.ExitReasonMatch)
}

// --- Scoring (task 9.3) ---

func TestScore_PerfectFidelityIsZero(t *testing.T) {
	set := ZeroDriftSet()
	pairs, _ := PairTrades(set.Live, set.Sim)
	deltas := deltasOf(pairs)
	comp := Score(deltas, DefaultConfig())
	assert.Equal(t, 0.0, comp.DriftScore)
}

func TestScore_ExtremeDriftClampedToOne(t *testing.T) {
	set := ExtremeDriftSet()
	pairs, _ := PairTrades(set.Live, set.Sim)
	comp := Score(deltasOf(pairs), DefaultConfig())
	assert.Equal(t, 1.0, comp.DriftScore)
	assert.LessOrEqual(t, comp.DriftScore, 1.0)
}

func TestScore_MonotonicNonDecreasing(t *testing.T) {
	cfg := DefaultConfig()
	small := Score(deltasOf(pairsOf(FixedPnLDriftSet(5.0))), cfg)
	// Larger drift on every dimension: bigger PnL delta, fill delta, and full
	// exit mismatch.
	big := Score(deltasOf(pairsOf(ExtremeDriftSet())), cfg)
	assert.GreaterOrEqual(t, big.DriftScore, small.DriftScore)
}

func TestScore_AggregatedDriftPnLEqualsConstruction(t *testing.T) {
	// Fixed 5.0 sim-minus-live drift on each of 3 pairs → aggregated 15.0.
	comp := Score(deltasOf(pairsOf(FixedPnLDriftSet(5.0))), DefaultConfig())
	assert.InDelta(t, 15.0, comp.DriftPnL, 1e-9)
}

// --- Aggregation and mapping (task 9.4) ---

func TestAggregate_DistinctStrategiesDistinctResults(t *testing.T) {
	var set TradeSet
	for _, s := range []TradeSet{ZeroDriftSet(), FixedPnLDriftSet(5.0)} {
		set.Live = append(set.Live, s.Live...)
		set.Sim = append(set.Sim, s.Sim...)
	}
	pairs, _ := PairTrades(set.Live, set.Sim)
	results := Aggregate(pairs, syntheticPeriod, DefaultConfig())

	require.Len(t, results, 2)
	byID := map[string]Result{}
	for _, r := range results {
		byID[r.StrategyID] = r
	}
	require.Contains(t, byID, "zero-drift")
	require.Contains(t, byID, "pnl-drift")
	assert.Equal(t, 0.0, byID["zero-drift"].DriftScore)
	assert.InDelta(t, 15.0, byID["pnl-drift"].DriftPnL, 1e-9)
}

func TestAggregate_WithinToleranceReflectsThreshold(t *testing.T) {
	var set TradeSet
	for _, s := range []TradeSet{ZeroDriftSet(), ExitReasonMismatchSet()} {
		set.Live = append(set.Live, s.Live...)
		set.Sim = append(set.Sim, s.Sim...)
	}
	pairs, _ := PairTrades(set.Live, set.Sim)
	results := Aggregate(pairs, syntheticPeriod, DefaultConfig())
	byID := indexByStrategy(results)

	assert.True(t, byID["zero-drift"].WithinTolerance)
	// Exit-mismatch rate 1.0 → 0.3 weight → 0.3 > 0.2 tolerance.
	assert.False(t, byID["exit-mismatch"].WithinTolerance)
}

func TestResult_ToRecordMapsFieldForField(t *testing.T) {
	r := Result{
		StrategyID:      "abc",
		Period:          Period{Start: time.Unix(100, 0).UTC(), End: time.Unix(200, 0).UTC()},
		DriftPnL:        12.5,
		DriftFill:       -0.75,
		DriftScore:      0.33,
		WithinTolerance: true,
	}
	rec := r.ToRecord()

	require.NotNil(t, rec.StrategyID)
	assert.Equal(t, "abc", *rec.StrategyID)
	require.NotNil(t, rec.PeriodStart)
	assert.True(t, r.Period.Start.Equal(*rec.PeriodStart))
	require.NotNil(t, rec.PeriodEnd)
	assert.True(t, r.Period.End.Equal(*rec.PeriodEnd))
	require.NotNil(t, rec.DriftPnl)
	assert.Equal(t, 12.5, *rec.DriftPnl)
	require.NotNil(t, rec.DriftFill)
	assert.Equal(t, -0.75, *rec.DriftFill)
	require.NotNil(t, rec.DriftScore)
	assert.Equal(t, 0.33, *rec.DriftScore)
	assert.True(t, rec.WithinTolerance)
	assert.Equal(t, "simulator_fidelity", rec.TableName())
}

// --- Gate and alert (task 9.5) ---

func TestGate_WithinToleranceProceedsWithoutAlert(t *testing.T) {
	spy := &spyAlerter{}
	results := []Result{{StrategyID: "ok", WithinTolerance: true}}
	decisions := Gate(context.Background(), results, spy)
	assert.Equal(t, Proceed, decisions["ok"])
	assert.Empty(t, spy.calls)
}

func TestGate_BreachPausesAndRaisesExactlyOneAlert(t *testing.T) {
	spy := &spyAlerter{}
	results := []Result{{StrategyID: "bad", WithinTolerance: false, DriftScore: 0.9}}
	decisions := Gate(context.Background(), results, spy)
	assert.Equal(t, Pause, decisions["bad"])
	require.Len(t, spy.calls, 1)
	assert.Equal(t, "bad", spy.calls[0].StrategyID)
}

func TestGate_MixedBatchOneAlertTotal(t *testing.T) {
	spy := &spyAlerter{}
	results := []Result{
		{StrategyID: "ok", WithinTolerance: true},
		{StrategyID: "bad", WithinTolerance: false},
	}
	decisions := Gate(context.Background(), results, spy)
	assert.Equal(t, Proceed, decisions["ok"])
	assert.Equal(t, Pause, decisions["bad"])
	require.Len(t, spy.calls, 1)
	assert.Equal(t, "bad", spy.calls[0].StrategyID)
}

// --- Checker orchestration ---

func TestRunFidelityCheck_NoLiveTrades(t *testing.T) {
	_, _, err := RunFidelityCheck(context.Background(), TradeSet{}, syntheticPeriod, DefaultConfig(), nil)
	require.ErrorIs(t, err, ErrNoLiveTrades)
}

func TestRunFidelityCheck_SyntheticEndToEnd(t *testing.T) {
	set, period := SyntheticTradeSet()
	spy := &spyAlerter{}
	results, decisions, err := RunFidelityCheck(context.Background(), set, period, DefaultConfig(), spy)
	require.NoError(t, err)
	require.Len(t, results, 4)

	byID := indexByStrategy(results)
	assert.Equal(t, Proceed, decisions["zero-drift"])
	assert.Equal(t, Proceed, decisions["pnl-drift"])
	assert.Equal(t, Pause, decisions["exit-mismatch"])
	assert.Equal(t, Pause, decisions["extreme-drift"])
	assert.Equal(t, 1.0, byID["extreme-drift"].DriftScore)
	// Exactly the two breaching strategies raised alerts.
	require.Len(t, spy.calls, 2)
}

// --- helpers ---

func reversed(in []Trade) []Trade {
	out := make([]Trade, len(in))
	for i, t := range in {
		out[len(in)-1-i] = t
	}
	return out
}

func pairsOf(set TradeSet) []Pair {
	pairs, _ := PairTrades(set.Live, set.Sim)
	return pairs
}

func deltasOf(pairs []Pair) []PairDelta {
	deltas := make([]PairDelta, len(pairs))
	for i, p := range pairs {
		deltas[i] = ComparePair(p)
	}
	return deltas
}

func indexByStrategy(results []Result) map[string]Result {
	byID := map[string]Result{}
	for _, r := range results {
		byID[r.StrategyID] = r
	}
	return byID
}
