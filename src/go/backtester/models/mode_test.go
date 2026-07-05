package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestModeValidate asserts the Mode enumeration contains exactly the three
// operator presets — and in particular no reconcile value and none of the
// internal AccountRole values (mode-presets spec, reconcile-concept-retirement 6.3).
func TestModeValidate(t *testing.T) {
	valid := []Mode{ModeSimulation, ModePaper, ModeMargin}
	for _, m := range valid {
		require.NoError(t, m.Validate(), "mode %q must validate", m)
	}

	invalid := []Mode{"", "reconcile", "reconcilation", "mock", "simulator", "live", "SIMULATION", "backtest"}
	for _, m := range invalid {
		require.Error(t, m.Validate(), "mode %q must be rejected", m)
	}
}

func TestModeIsRealtime(t *testing.T) {
	require.False(t, ModeSimulation.IsRealtime())
	require.True(t, ModePaper.IsRealtime())
	require.True(t, ModeMargin.IsRealtime())
	require.False(t, Mode("").IsRealtime(), "zero-value mode (internal reconciliation containers) is not realtime")
}

// TestAccountRoleValidate asserts the internal role tag accepts exactly the
// five legacy-compatible persisted values.
func TestAccountRoleValidate(t *testing.T) {
	valid := []AccountRole{AccountRolePaper, AccountRoleMargin, AccountRoleReconcilation, AccountRoleMock, AccountRoleSimulator}
	for _, r := range valid {
		require.NoError(t, r.Validate(), "role %q must validate", r)
	}

	invalid := []AccountRole{"", "simulation", "reconcile", "real"}
	for _, r := range invalid {
		require.Error(t, r.Validate(), "role %q must be rejected", r)
	}
}

// TestAccountRolePersistedStrings pins the five persisted string values —
// order_records.account_type / live_accounts.account_type /
// playground_sessions.live_account_type must stay byte-identical.
func TestAccountRolePersistedStrings(t *testing.T) {
	require.Equal(t, "paper", string(AccountRolePaper))
	require.Equal(t, "margin", string(AccountRoleMargin))
	require.Equal(t, "reconcilation", string(AccountRoleReconcilation))
	require.Equal(t, "mock", string(AccountRoleMock))
	require.Equal(t, "simulator", string(AccountRoleSimulator))
}
