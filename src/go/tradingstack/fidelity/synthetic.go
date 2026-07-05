package fidelity

import (
	"math"
	"time"
)

// syntheticEpoch is the fixed reference entry time for synthetic trades. Using a
// constant keeps the synthetic data deterministic (no wall clock).
var syntheticEpoch = time.Date(2026, time.January, 5, 14, 30, 0, 0, time.UTC)

// syntheticPeriod bounds the synthetic entry times generously so every synthetic
// trade falls inside it.
var syntheticPeriod = Period{
	Start: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	End:   time.Date(2026, time.January, 8, 0, 0, 0, 0, time.UTC),
}

// syntheticLiveBase returns a deterministic baseline live trade for a strategy,
// offset by i so successive trades key distinctly on entry timestamp.
func syntheticLiveBase(strategyID, symbol string, i int) Trade {
	return Trade{
		StrategyID:   strategyID,
		Symbol:       symbol,
		EntryAt:      syntheticEpoch.Add(time.Duration(i) * time.Hour),
		EntryFill:    100.0,
		ExitFill:     110.0,
		PnL:          10.0,
		HoldDuration: 2 * time.Hour,
		ExitReason:   ExitReasonTarget,
	}
}

// ZeroDriftSet builds a strategy whose simulator trades mirror its live trades
// exactly: every drift dimension is zero by construction, so its drift_score is
// 0.0 and it is within tolerance.
func ZeroDriftSet() TradeSet {
	const sid = "zero-drift"
	var set TradeSet
	for i := 0; i < 3; i++ {
		live := syntheticLiveBase(sid, "AAPL", i)
		sim := live // identical
		set.Live = append(set.Live, live)
		set.Sim = append(set.Sim, sim)
	}
	return set
}

// FixedPnLDriftSet builds a strategy whose simulator PnL exceeds live PnL by a
// fixed known amount on every pair. The aggregated drift_pnl equals that amount
// times the number of pairs.
func FixedPnLDriftSet(driftPerPair float64) TradeSet {
	const sid = "pnl-drift"
	var set TradeSet
	for i := 0; i < 3; i++ {
		live := syntheticLiveBase(sid, "MSFT", i)
		sim := live
		sim.PnL = live.PnL + driftPerPair
		set.Live = append(set.Live, live)
		set.Sim = append(set.Sim, sim)
	}
	return set
}

// ExitReasonMismatchSet builds a strategy whose simulator exits differ from live
// exits on every pair (live target vs. sim stop) while all other dimensions
// match. Its mismatch rate is 1.0.
func ExitReasonMismatchSet() TradeSet {
	const sid = "exit-mismatch"
	var set TradeSet
	for i := 0; i < 3; i++ {
		live := syntheticLiveBase(sid, "TSLA", i)
		live.ExitReason = ExitReasonTarget
		sim := live
		sim.ExitReason = ExitReasonStop
		set.Live = append(set.Live, live)
		set.Sim = append(set.Sim, sim)
	}
	return set
}

// ExtremeDriftSet builds a strategy whose simulator trades diverge from live on
// every dimension by arbitrarily large amounts and fully mismatch exit reasons,
// so its drift_score clamps to 1.0 and it is out of tolerance.
func ExtremeDriftSet() TradeSet {
	const sid = "extreme-drift"
	var set TradeSet
	for i := 0; i < 3; i++ {
		live := syntheticLiveBase(sid, "NVDA", i)
		sim := live
		// Arbitrarily large (infinite) drift on the magnitude dimensions so the
		// composite reaches exactly 1.0 rather than merely approaching it.
		sim.EntryFill = math.Inf(1)
		sim.ExitFill = math.Inf(1)
		sim.PnL = math.Inf(1)
		sim.HoldDuration = live.HoldDuration + 1000*time.Hour
		sim.ExitReason = ExitReasonSignalExit // != target
		set.Live = append(set.Live, live)
		set.Sim = append(set.Sim, sim)
	}
	return set
}

// SyntheticTradeSet combines the four known-drift strategies into a single
// TradeSet (with its covering Period) so the operator command's --synthetic mode
// exercises the full engine with no live-trade input. The zero-drift and small
// PnL-drift strategies land within tolerance (Proceed); the exit-mismatch and
// extreme-drift strategies breach tolerance (Pause).
func SyntheticTradeSet() (TradeSet, Period) {
	var set TradeSet
	for _, s := range []TradeSet{
		ZeroDriftSet(),
		FixedPnLDriftSet(5.0),
		ExitReasonMismatchSet(),
		ExtremeDriftSet(),
	} {
		set.Live = append(set.Live, s.Live...)
		set.Sim = append(set.Sim, s.Sim...)
	}
	return set, syntheticPeriod
}
