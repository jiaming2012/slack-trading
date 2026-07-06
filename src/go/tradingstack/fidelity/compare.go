package fidelity

import (
	"math"
	"time"
)

// PairDelta holds the per-pair deltas on the four architecture-named drift
// dimensions. Every delta is simulator-minus-live so the sign is consistent
// across the engine.
//
// Fill drift is carried in two forms with distinct consumers:
//   - FillDelta (signed mean) feeds the reported and persisted drift_fill —
//     its sign says whether the simulator fills better or worse than live.
//   - FillDeltaAbs (absolute mean) feeds the composite drift score — opposite-
//     signed entry/exit leg drifts must not cancel.
type PairDelta struct {
	// PnLDelta is simulator PnL minus live PnL.
	PnLDelta float64
	// FillDelta is the mean of the SIGNED per-leg fill deltas (simulator minus
	// live, entry and exit legs). It is the reporting/persistence form and must
	// never feed the composite score: opposite-signed legs cancel here by
	// construction.
	FillDelta float64
	// FillDeltaAbs is the fill-drift magnitude: the mean of the ABSOLUTE
	// per-leg fill deltas, so an entry leg drifting +x and an exit leg drifting
	// −x contribute x (not zero). This is the form the composite score consumes.
	FillDeltaAbs float64
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
		FillDeltaAbs:    (math.Abs(entryFillDelta) + math.Abs(exitFillDelta)) / 2.0,
		HoldDelta:       p.Sim.HoldDuration - p.Live.HoldDuration,
		ExitReasonMatch: p.Sim.ExitReason == p.Live.ExitReason,
	}
}
