package models

import "fmt"

// Legacy persisted values of the playground_sessions.environment column.
// These strings are a persistence/RPC boundary artifact: they are read and
// written byte-identically so that no stored production data is mutated
// (migrate-crossed-enums; canonicalization is the separate
// migrate-live-account-type-data change). Business logic must branch on
// Mode / AccountRole / Meta.IsReconciliation, never on these strings.
const (
	LegacyEnvSimulator = "simulator"
	LegacyEnvLive      = "live"
	LegacyEnvReconcile = "reconcile"
)

// ModeFromLegacy translates a legacy (environment, live_account_type) pair
// into a Mode. It has four outcomes:
//
//   - a valid Mode (Simulation, Paper, Margin)
//   - internalReconcile=true for environment="reconcile" rows — the internal
//     netting containers behind the Broker seam, which are NOT a Mode
//   - an error naming the offending values for any combination outside the
//     documented mapping (never a silent default)
//
// Mapping table (see design.md):
//
//	simulator × {mock, simulator, ""}  → Simulation
//	live      × paper                  → Paper
//	live      × margin                 → Margin
//	live      × mock                   → Paper  (mock broker: fake-money test double of the broker API venue)
//	reconcile × *                      → internal reconciliation (not a Mode)
func ModeFromLegacy(environment string, liveAccountType string) (mode Mode, internalReconcile bool, err error) {
	switch environment {
	case LegacyEnvSimulator:
		switch liveAccountType {
		case "", string(AccountRoleMock), string(AccountRoleSimulator):
			return ModeSimulation, false, nil
		}
	case LegacyEnvLive:
		switch liveAccountType {
		case string(AccountRolePaper):
			return ModePaper, false, nil
		case string(AccountRoleMargin):
			return ModeMargin, false, nil
		case string(AccountRoleMock):
			// The mock broker is a fake-money test double speaking the broker
			// API protocol; its operator-facing preset is Paper. The mock role
			// itself is preserved internally (AccountRole) and on disk.
			return ModePaper, false, nil
		}
	case LegacyEnvReconcile:
		return "", true, nil
	}

	return "", false, fmt.Errorf("no mode preset for legacy combination (environment=%q, live_account_type=%q)", environment, liveAccountType)
}

// ToLegacy returns the canonical legacy (environment, live_account_type)
// string pair for a Mode. It is used where only a Mode is known (e.g. fresh
// playground creation defaults); persisted rows round-trip their original
// role value via Meta.Role instead, so stored bytes are never rewritten.
func (m Mode) ToLegacy() (environment string, liveAccountType string) {
	switch m {
	case ModeSimulation:
		return LegacyEnvSimulator, string(AccountRoleSimulator)
	case ModePaper:
		return LegacyEnvLive, string(AccountRolePaper)
	case ModeMargin:
		return LegacyEnvLive, string(AccountRoleMargin)
	default:
		return "", ""
	}
}

// LegacyEnvironment returns the legacy environment column value for the
// mode: "simulator" for Simulation, "live" for Paper and Margin.
func (m Mode) LegacyEnvironment() string {
	env, _ := m.ToLegacy()
	return env
}

// DefaultAccountRole returns the canonical internal role for a mode, used
// when no explicit role accompanies the request (e.g. Simulation
// playgrounds, which have no broker account source).
func (m Mode) DefaultAccountRole() AccountRole {
	_, role := m.ToLegacy()
	return AccountRole(role)
}
