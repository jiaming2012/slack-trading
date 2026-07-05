package fidelity

import "time"

// PairDelta holds the per-pair deltas on the four architecture-named drift
// dimensions. Every delta is simulator-minus-live so the sign is consistent
// across the engine.
type PairDelta struct {
	// PnLDelta is simulator PnL minus live PnL.
	PnLDelta float64
	// FillDelta is the simulator fill price minus the live fill price, averaged
	// over the entry and exit legs of the trade.
	FillDelta float64
	// HoldDelta is simulator hold duration minus live hold duration.
	HoldDelta time.Duration
	// ExitReasonMatch is true when the simulator and live exit reasons are equal.
	ExitReasonMatch bool
}

// ComparePair computes the per-pair deltas for a matched sim-vs-live pair. It is
// a pure function of the pair: no wall-clock time, randomness, database state,
// or external services are consulted.
func ComparePair(p Pair) PairDelta {
	entryFillDelta := p.Sim.EntryFill - p.Live.EntryFill
	exitFillDelta := p.Sim.ExitFill - p.Live.ExitFill
	return PairDelta{
		PnLDelta:        p.Sim.PnL - p.Live.PnL,
		FillDelta:       (entryFillDelta + exitFillDelta) / 2.0,
		HoldDelta:       p.Sim.HoldDuration - p.Live.HoldDuration,
		ExitReasonMatch: p.Sim.ExitReason == p.Live.ExitReason,
	}
}
