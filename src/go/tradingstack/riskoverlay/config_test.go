package riskoverlay

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// 6.10 — missing block yields documented defaults.
func TestLoadRiskLimits_MissingBlockDefaults(t *testing.T) {
	raw := []byte("someOtherConfig:\n  foo: bar\n")
	limits, err := LoadRiskLimits(raw)
	require.NoError(t, err)
	require.Equal(t, DefaultRiskLimits, limits)
}

func TestLoadRiskLimits_EmptyInputDefaults(t *testing.T) {
	limits, err := LoadRiskLimits(nil)
	require.NoError(t, err)
	require.Equal(t, DefaultRiskLimits, limits)
}

// A present block overrides only the fields it sets; absent fields keep defaults.
func TestLoadRiskLimits_PartialOverride(t *testing.T) {
	raw := []byte(`riskOverlay:
  max_gross_exposure: 100000
  max_sector_concentration_pct: 50
  reject_crowded_entries: true
`)
	limits, err := LoadRiskLimits(raw)
	require.NoError(t, err)
	require.Equal(t, 100_000.0, limits.MaxGrossExposure)
	require.Equal(t, 50.0, limits.MaxSectorConcentrationPct)
	require.True(t, limits.RejectCrowdedEntries)
	// untouched fields keep defaults
	require.Equal(t, DefaultRiskLimits.MaxNetExposure, limits.MaxNetExposure)
	require.Equal(t, DefaultRiskLimits.MaxDrawdownPct, limits.MaxDrawdownPct)
	require.Equal(t, DefaultRiskLimits.DeployableCapital, limits.DeployableCapital)
}

// An explicit zero is honored (distinguished from absent via pointer fields).
func TestLoadRiskLimits_ExplicitZeroHonored(t *testing.T) {
	raw := []byte("riskOverlay:\n  max_gross_exposure: 0\n")
	limits, err := LoadRiskLimits(raw)
	require.NoError(t, err)
	require.Equal(t, 0.0, limits.MaxGrossExposure)
}

// 6.10 — negative / out-of-range limits are rejected with the sentinel.
func TestLoadRiskLimits_InvalidRejected(t *testing.T) {
	cases := map[string]string{
		"negative gross":       "riskOverlay:\n  max_gross_exposure: -1\n",
		"negative net":         "riskOverlay:\n  max_net_exposure: -5\n",
		"negative capital":     "riskOverlay:\n  deployable_capital: -100\n",
		"sector over 100":      "riskOverlay:\n  max_sector_concentration_pct: 101\n",
		"sector negative":      "riskOverlay:\n  max_sector_concentration_pct: -1\n",
		"drawdown over 100":    "riskOverlay:\n  max_drawdown_pct: 150\n",
		"drawdown negative":    "riskOverlay:\n  max_drawdown_pct: -2\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadRiskLimits([]byte(raw))
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInvalidRiskLimits), "want ErrInvalidRiskLimits, got %v", err)
		})
	}
}

func TestValidateRiskLimits_DefaultsValid(t *testing.T) {
	require.NoError(t, ValidateRiskLimits(DefaultRiskLimits))
}

// B3 — the enablement flag exists in the config, defaults to false, and is
// loaded + validated. (Installation of the gate is deferred; this test only
// exercises the flag round-trip through the loader.)
func TestLoadRiskLimits_EnabledFlag(t *testing.T) {
	t.Run("absent enabled defaults to false", func(t *testing.T) {
		limits, err := LoadRiskLimits([]byte("riskOverlay:\n  max_gross_exposure: 100000\n"))
		require.NoError(t, err)
		require.False(t, limits.Enabled)
	})

	t.Run("explicit true is honored", func(t *testing.T) {
		limits, err := LoadRiskLimits([]byte("riskOverlay:\n  enabled: true\n"))
		require.NoError(t, err)
		require.True(t, limits.Enabled)
	})

	t.Run("explicit false is honored", func(t *testing.T) {
		limits, err := LoadRiskLimits([]byte("riskOverlay:\n  enabled: false\n"))
		require.NoError(t, err)
		require.False(t, limits.Enabled)
	})

	t.Run("missing block leaves enabled false", func(t *testing.T) {
		limits, err := LoadRiskLimits(nil)
		require.NoError(t, err)
		require.False(t, limits.Enabled)
	})
}

func TestLoadRiskLimitsFromFile_MissingFileDefaults(t *testing.T) {
	limits, err := LoadRiskLimitsFromFile("/no/such/risk-overlay-config.yaml")
	require.NoError(t, err)
	require.Equal(t, DefaultRiskLimits, limits)
}
