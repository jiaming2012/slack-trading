// Package shadowdeploy runs a gate-validated scanner-config proposal in
// shadow alongside the active config (OpenSpec change
// shadow-config-deployment): both payloads are evaluated over the exact same
// persisted Simulation scan observations through a pure, payload-driven
// decision engine, and every decision divergence plus the available outcome
// comparison is persisted as evidence for the operator's promotion decision.
//
// The package is Simulation-only and read-only except for its own two tables
// (shadow_runs, shadow_divergences): it places no orders through any broker
// path, and it never mutates scanner_configs, scanner_config_proposals (a
// proposal's status included), scan_results, or sim_outcomes. Promotion stays
// exclusively with the scanner-optimizer's operator promote command.
//
// Known limitation, surfaced in every run report: shadow evaluation replays
// scan_results rows the active pipeline persisted, so a shadow config that
// would loosen Layer-1 admission cannot surface tickers that were never
// scanned. The named future changes scanner-config-hot-swap and
// shadow-outcome-backfill lift this structurally.
package shadowdeploy

import "errors"

// Sentinel errors for shadow-config deployment.
var (
	// ErrNotSimulation is returned by RunShadow when the operating mode is
	// anything but Simulation -- shadow evidence must never be generated
	// from, or mistaken for, a Paper or Margin (real-money) context.
	ErrNotSimulation = errors.New("shadowdeploy: shadow runs are Simulation-only (Paper and Margin are refused)")

	// ErrNoObservations is returned by RunShadow when the requested
	// scanned_at window contains no scan_results rows: nothing is persisted,
	// rather than a misleading zero-divergence run.
	ErrNoObservations = errors.New("shadowdeploy: no scan observations in the requested window")

	// ErrProposalRejectedByGate is returned by RunShadow when the shadow
	// candidate's proposal status is rejected_by_gate -- only gate-validated
	// proposals earn shadow evidence.
	ErrProposalRejectedByGate = errors.New("shadowdeploy: proposal was rejected by the overfitting gate and cannot be shadowed")

	// ErrRunNotFound is returned by ShadowStore.FetchRun when no shadow run
	// exists for the requested id.
	ErrRunNotFound = errors.New("shadowdeploy: shadow run not found")

	// ErrInvalidDivergenceKind is returned when a ShadowDivergence row
	// carries a kind outside {shadow_only, active_only}.
	ErrInvalidDivergenceKind = errors.New("shadowdeploy: invalid divergence kind")

	// ErrMissingProposalTable is returned by MigrateShadowDeployment when
	// the scanner_config_proposals table (the proposal_id foreign-key
	// target) does not exist -- run scanneropt.MigrateScannerOptimizer
	// first.
	ErrMissingProposalTable = errors.New("shadowdeploy: required table scanner_config_proposals is missing (run scanneropt.MigrateScannerOptimizer first)")
)
