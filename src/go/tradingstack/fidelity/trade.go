package fidelity

import "time"

// ExitReason reuses the closed exit-reason vocabulary delivered by
// trading-stack-schema (sim_outcomes.exit_reason). Defining it locally keeps the
// fidelity engine decoupled from the persistence models while sharing the same
// vocabulary, so real live-trade ingestion can wire into it later without
// changing the engine.
type ExitReason string

const (
	ExitReasonStop       ExitReason = "stop"
	ExitReasonTarget     ExitReason = "target"
	ExitReasonTimeout    ExitReason = "timeout"
	ExitReasonSignalExit ExitReason = "signal_exit"
)

// Trade is the pure input value type the fidelity engine compares. Both live
// trades and simulator trades are represented identically; the engine never
// depends on where a Trade came from. All fields are plain values so pairing,
// comparison, and scoring are pure functions with no clock, randomness, DB, or
// network in the loop.
type Trade struct {
	StrategyID   string
	Symbol       string
	EntryAt      time.Time
	EntryFill    float64
	ExitFill     float64
	PnL          float64
	HoldDuration time.Duration
	ExitReason   ExitReason
}

// TradeSet wraps the live and simulator trade slices for a single period.
type TradeSet struct {
	Live []Trade
	Sim  []Trade
}

// Period identifies the inclusive time window a fidelity check covers. It is
// carried onto every produced Result and persisted onto the
// tradingstack.SimulatorFidelity record.
type Period struct {
	Start time.Time
	End   time.Time
}
