// Package riskoverlay is a pre-trade portfolio risk gate. Its heart is a pure,
// side-effect-free evaluation engine (Evaluate) that takes an immutable
// snapshot of portfolio state plus one proposed order and returns an
// allow/reject Decision with a typed list of limit breaches. Keeping the
// engine pure is what makes every limit-breach scenario deterministically
// unit-testable without a database, broker, or clock.
//
// The impure adhesive (YAML config loading, the CrowdingLookup seam that reads
// the crowding tables, and the single Simulation-path wiring point) lives
// alongside the engine but is kept strictly separate so the engine never
// performs I/O.
package riskoverlay

// LogicalPosition is one aggregate position in the portfolio snapshot,
// identified by ticker and sector and carrying a signed notional (positive for
// net-long, negative for net-short exposure in the underlying).
type LogicalPosition struct {
	Ticker         string
	Sector         string
	SignedNotional float64
}

// PortfolioState is an immutable snapshot of the portfolio at the moment an
// order is proposed. Every field is data the caller passes in; the engine
// reads it and nothing else — no DB, no broker, no clock.
type PortfolioState struct {
	// Positions are the current logical positions (one per ticker).
	Positions []LogicalPosition

	// StrategyDeployed maps a strategy ID to the absolute capital that
	// strategy currently has deployed. Used by the per-strategy allocation
	// cap check.
	StrategyDeployed map[string]float64

	// EvWeights maps a strategy ID to its raw strategy_ev_weights.ev_weight
	// value. The per-strategy allocation cap normalizes these against their
	// sum. A strategy absent from this map has weight zero. An empty map means
	// no EV-weight data is participating and the allocation check is skipped.
	EvWeights map[string]float64

	// EquitySeries is the trailing (up to 5-session) portfolio equity series in
	// chronological order; the last element is the current equity. Used by the
	// drawdown circuit breaker.
	EquitySeries []float64
}

// ProposedOrder is the single order being evaluated. SignedNotional is positive
// for a buy/long-increasing order and negative for a sell/short-increasing
// order. IsReduction is true when the order reduces or closes an existing
// logical position rather than opening or increasing one — reductions always
// pass (see Evaluate).
type ProposedOrder struct {
	Ticker         string
	Sector         string
	StrategyID     string
	SignedNotional float64
	IsReduction    bool
}

// RiskLimits are the configured limit values for a single evaluation. Exposure
// and capital values are absolute currency amounts; the *Pct fields are
// percentages in the range 0–100.
type RiskLimits struct {
	MaxGrossExposure          float64
	MaxNetExposure            float64
	MaxSectorConcentrationPct float64
	MaxDrawdownPct            float64
	DeployableCapital         float64
	RejectCrowdedEntries      bool
}
