package fidelity

import "errors"

// Sentinel errors for the fidelity checker.
var (
	// ErrNoLiveTrades is returned when a fidelity check is requested for a
	// period whose live-trade set is empty. It signals a clean no-input outcome
	// (not a failure): there is nothing to compare, so no fidelity records are
	// produced.
	ErrNoLiveTrades = errors.New("fidelity: no live trades available for the period")

	// ErrSimulatorFidelityTableMissing is returned by MigrateFidelityMonitoring
	// when the simulator_fidelity table does not exist — i.e. the
	// trading-stack-schema migration (tradingstack.MigrateTradingStack) has not
	// run yet. Mirrors the costmodel.ErrStrategyEvWeightsTableMissing precedent.
	ErrSimulatorFidelityTableMissing = errors.New("fidelity: simulator_fidelity table missing; run tradingstack.MigrateTradingStack first")
)
