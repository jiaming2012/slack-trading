package models

import "fmt"

// Mode is the operator-selected pairing of a market-data feed source and a
// broker execution venue, fixed at playground creation (ADR-0001,
// CONTEXT.md "Modes"). Exactly three presets exist; because a Mode is a
// single value rather than a cross-product of independent enums, illegal
// feed/venue combinations are unrepresentable.
//
// Mode is in-memory only: at the persistence and RPC boundaries it is
// translated to/from the legacy environment / live_account_type string pair
// by ModeFromLegacy and ToLegacy (mode_compat.go). Reconciliation is NOT a
// Mode — it is an internal netting mechanism behind the Broker seam (see
// Meta.IsReconciliation).
type Mode string

const (
	// ModeSimulation binds a historical replay feed to a Simulation Account;
	// our fill engine (SimulatedBroker) decides fills.
	ModeSimulation Mode = "simulation"

	// ModePaper binds the real-time feed to a Paper Account — real broker
	// API traffic, mock fills, no real money.
	ModePaper Mode = "paper"

	// ModeMargin binds the real-time feed to a Margin Account — real order
	// execution, real money.
	ModeMargin Mode = "margin"
)

// Validate accepts exactly the three mode presets.
func (m Mode) Validate() error {
	switch m {
	case ModeSimulation, ModePaper, ModeMargin:
		return nil
	default:
		return fmt.Errorf("invalid mode: %s (valid modes: %s, %s, %s)", m, ModeSimulation, ModePaper, ModeMargin)
	}
}

// IsRealtime reports whether the mode binds the real-time feed to a broker
// API venue (Paper or Margin) — the replacement for the legacy
// environment=="live" check. It is false for ModeSimulation and for the
// zero value carried by internal reconciliation containers.
func (m Mode) IsRealtime() bool {
	return m == ModePaper || m == ModeMargin
}
