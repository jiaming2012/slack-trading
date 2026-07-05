package models

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// legacyRowFixture mirrors a persisted playground_sessions row at the
// persistence boundary: Role and LegacyEnv are exactly what GORM scans from
// the live_account_type / environment columns, and HydrateMode is what the
// Playground AfterFind hook runs on load.
type legacyRowFixture struct {
	environment     string
	liveAccountType AccountRole
	wantMode        Mode
	wantReconcile   bool
}

var legacyRowFixtures = []legacyRowFixture{
	{"simulator", AccountRoleMock, ModeSimulation, false}, // legacy mock row
	{"simulator", AccountRoleSimulator, ModeSimulation, false},
	{"live", AccountRolePaper, ModePaper, false},
	{"live", AccountRoleMargin, ModeMargin, false},
	{"live", AccountRoleMock, ModePaper, false}, // mock-broker live playground (e2e double)
	// reconcile containers persist the UNDERLYING account role
	{"reconcile", AccountRolePaper, "", true},
	{"reconcile", AccountRoleMargin, "", true},
	{"reconcile", AccountRoleMock, "", true},
	{"reconcile", AccountRoleReconcilation, "", true}, // proposal-documented legacy value
}

// TestMetaLegacyRowRoundTrip verifies, for every legacy fixture row:
// read → Mode (via the AfterFind hydration path), write-back → byte-identical
// column values, and re-read → the same Mode (idempotence). This is the
// persistence-boundary contract of legacy-enum-compat-mapping: stored bytes
// are reinterpreted, never rewritten.
func TestMetaLegacyRowRoundTrip(t *testing.T) {
	for _, row := range legacyRowFixtures {
		t.Run(fmt.Sprintf("%s_%s", row.environment, row.liveAccountType), func(t *testing.T) {
			// read path: GORM scans the two columns, AfterFind hydrates Mode
			meta := Meta{
				Role:      row.liveAccountType,
				LegacyEnv: row.environment,
			}
			require.NoError(t, meta.HydrateMode())
			require.Equal(t, row.wantMode, meta.Mode)
			require.Equal(t, row.wantReconcile, meta.IsReconciliation())

			// write path: GORM persists Role and LegacyEnv verbatim — assert
			// the would-be column bytes are identical to what was loaded.
			require.Equal(t, row.liveAccountType, meta.Role, "live_account_type column bytes must be preserved")
			require.Equal(t, row.environment, meta.LegacyEnv, "environment column bytes must be preserved")

			// re-read: hydrating again from the written values yields the same
			// mode (round-trip idempotence).
			reread := Meta{Role: meta.Role, LegacyEnv: meta.LegacyEnv}
			require.NoError(t, reread.HydrateMode())
			require.Equal(t, meta.Mode, reread.Mode)
			require.Equal(t, meta.IsReconciliation(), reread.IsReconciliation())
		})
	}
}

// TestMetaLegacyReconcileRowLoads pins reconcile-concept-retirement: a legacy
// environment="reconcile" row loads without error, is flagged as internal
// reconciliation, and is NOT represented by any operator Mode.
func TestMetaLegacyReconcileRowLoads(t *testing.T) {
	meta := Meta{
		Role:      AccountRolePaper,
		LegacyEnv: "reconcile",
	}

	require.NoError(t, meta.HydrateMode(), "loading a legacy reconcile row must not fail")
	require.True(t, meta.IsReconciliation())
	require.Error(t, meta.Mode.Validate(), "a reconcile row must not carry a valid operator Mode")

	// The internal container still routes to the ReconcileBroker seam adapter.
	broker, err := brokerFor(&meta)
	require.NoError(t, err)
	require.IsType(t, ReconcileBroker{}, broker)
}

// TestMetaUnknownLegacyRowFailsLoad: an unrecognized column combination
// surfaces an explicit error naming the offending values (no silent default).
func TestMetaUnknownLegacyRowFailsLoad(t *testing.T) {
	meta := Meta{
		Role:      AccountRoleSimulator,
		LegacyEnv: "live", // contradictory pair
	}

	err := meta.HydrateMode()
	require.Error(t, err)
	require.Contains(t, err.Error(), `environment="live"`)
	require.Contains(t, err.Error(), `live_account_type="simulator"`)
}

// TestNewMetaConstructionWritesCanonicalLegacyBytes pins the bytes a fresh
// (post-migration) playground writes to the legacy environment column per
// mode — identical to what the pre-migration code wrote.
func TestNewMetaConstructionWritesCanonicalLegacyBytes(t *testing.T) {
	require.Equal(t, "simulator", NewMeta(ModeSimulation, nil).LegacyEnv)
	require.Equal(t, "live", NewMeta(ModePaper, nil).LegacyEnv)
	require.Equal(t, "live", NewMeta(ModeMargin, nil).LegacyEnv)
	require.Equal(t, "reconcile", NewReconciliationMeta(nil).LegacyEnv)
}
