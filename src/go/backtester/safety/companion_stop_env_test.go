package safety

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadCompanionStopEnvConfig_UnsetDisables(t *testing.T) {
	t.Setenv(EnvCompanionStopDistance, "")

	cfg, err := LoadCompanionStopEnvConfig()
	require.NoError(t, err)
	require.Nil(t, cfg, "an unset distance must disable the feature, not guess a default")
}

func TestLoadCompanionStopEnvConfig_PositiveEnables(t *testing.T) {
	t.Setenv(EnvCompanionStopDistance, "2.5")

	cfg, err := LoadCompanionStopEnvConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, 2.5, cfg.StopDistance)
	require.NoError(t, cfg.Validate())
}

func TestLoadCompanionStopEnvConfig_NonPositiveRefused(t *testing.T) {
	for _, raw := range []string{"0", "-1", "-0.01"} {
		t.Setenv(EnvCompanionStopDistance, raw)

		cfg, err := LoadCompanionStopEnvConfig()
		require.Errorf(t, err, "distance %q must refuse startup", raw)
		require.Nil(t, cfg)
		require.Contains(t, err.Error(), EnvCompanionStopDistance)
	}
}

func TestLoadCompanionStopEnvConfig_UnparseableRefused(t *testing.T) {
	t.Setenv(EnvCompanionStopDistance, "five dollars")

	cfg, err := LoadCompanionStopEnvConfig()
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), EnvCompanionStopDistance)
}
