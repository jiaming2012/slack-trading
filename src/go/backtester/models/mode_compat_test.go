package models

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestModeFromLegacyMappingTable exercises every documented legacy
// (environment, live_account_type) combination (design.md mapping table).
func TestModeFromLegacyMappingTable(t *testing.T) {
	cases := []struct {
		environment       string
		liveAccountType   string
		wantMode          Mode
		wantInternalRecon bool
	}{
		{"simulator", "mock", ModeSimulation, false},
		{"simulator", "simulator", ModeSimulation, false},
		{"simulator", "", ModeSimulation, false},
		{"live", "paper", ModePaper, false},
		{"live", "margin", ModeMargin, false},
		{"live", "mock", ModePaper, false},
		{"reconcile", "reconcilation", "", true},
		// reconcile containers persist the UNDERLYING account role, not
		// "reconcilation" (see PopulatePlayground); all of them are internal.
		{"reconcile", "paper", "", true},
		{"reconcile", "margin", "", true},
		{"reconcile", "mock", "", true},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%s", tc.environment, tc.liveAccountType), func(t *testing.T) {
			mode, internalRecon, err := ModeFromLegacy(tc.environment, tc.liveAccountType)
			require.NoError(t, err)
			require.Equal(t, tc.wantMode, mode)
			require.Equal(t, tc.wantInternalRecon, internalRecon)
		})
	}
}

// TestModeFromLegacyRejectsUnknownCombinations asserts combinations outside
// the documented mapping return an explicit error naming the offending
// values — never a silent default.
func TestModeFromLegacyRejectsUnknownCombinations(t *testing.T) {
	cases := []struct{ environment, liveAccountType string }{
		{"live", "simulator"}, // the contradictory pair from the spec scenario
		{"live", ""},
		{"live", "reconcilation"},
		{"simulator", "paper"},
		{"simulator", "margin"},
		{"simulator", "reconcilation"},
		{"", ""},
		{"production", "margin"},
		{"simulation", "simulator"}, // Mode strings are not legacy strings
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%s", tc.environment, tc.liveAccountType), func(t *testing.T) {
			_, _, err := ModeFromLegacy(tc.environment, tc.liveAccountType)
			require.Error(t, err)
			require.Contains(t, err.Error(), fmt.Sprintf("environment=%q", tc.environment), "error must name the offending environment")
			require.Contains(t, err.Error(), fmt.Sprintf("live_account_type=%q", tc.liveAccountType), "error must name the offending account type")
		})
	}
}

// TestModeToLegacyCanonicalPairs pins the canonical write pairs.
func TestModeToLegacyCanonicalPairs(t *testing.T) {
	env, role := ModeSimulation.ToLegacy()
	require.Equal(t, "simulator", env)
	require.Equal(t, "simulator", role)

	env, role = ModePaper.ToLegacy()
	require.Equal(t, "live", env)
	require.Equal(t, "paper", role)

	env, role = ModeMargin.ToLegacy()
	require.Equal(t, "live", env)
	require.Equal(t, "margin", role)
}

// TestModeLegacyRoundTrip: ModeFromLegacy(ToLegacy(m)) == m for all three
// modes (idempotent round-trip), and every legacy combination in the mapping
// table re-reads to the same Mode after being written back through the
// preserved (environment, role) pair — the MIG-03 round-trip property run by
// `task test:migrate-crossed-enums`.
func TestModeLegacyRoundTrip(t *testing.T) {
	for _, m := range []Mode{ModeSimulation, ModePaper, ModeMargin} {
		env, role := m.ToLegacy()
		got, internalRecon, err := ModeFromLegacy(env, role)
		require.NoError(t, err)
		require.False(t, internalRecon)
		require.Equal(t, m, got, "round-trip must be idempotent for %q", m)
	}

	// Legacy rows are written back with their ORIGINAL role preserved
	// (Meta.Role) and the environment derived from the resolved state; the
	// re-read must yield the same Mode even for non-canonical roles.
	legacyRows := []struct{ environment, role string }{
		{"simulator", "mock"},
		{"simulator", "simulator"},
		{"live", "paper"},
		{"live", "margin"},
		{"live", "mock"},
	}
	for _, row := range legacyRows {
		first, internalRecon, err := ModeFromLegacy(row.environment, row.role)
		require.NoError(t, err)
		require.False(t, internalRecon)

		// Write-back: environment derived from Mode, role preserved as-is.
		writtenEnv := first.LegacyEnvironment()
		require.Equal(t, row.environment, writtenEnv, "write-back must not change the environment bytes for (%s, %s)", row.environment, row.role)

		second, internalRecon, err := ModeFromLegacy(writtenEnv, row.role)
		require.NoError(t, err)
		require.False(t, internalRecon)
		require.Equal(t, first, second, "re-read after write-back must yield the same mode for (%s, %s)", row.environment, row.role)
	}

	// Reconcile rows round-trip as internal reconciliation, never a Mode.
	_, internalRecon, err := ModeFromLegacy("reconcile", "paper")
	require.NoError(t, err)
	require.True(t, internalRecon)
}

func TestModeDefaultAccountRole(t *testing.T) {
	require.Equal(t, AccountRoleSimulator, ModeSimulation.DefaultAccountRole())
	require.Equal(t, AccountRolePaper, ModePaper.DefaultAccountRole())
	require.Equal(t, AccountRoleMargin, ModeMargin.DefaultAccountRole())
}
