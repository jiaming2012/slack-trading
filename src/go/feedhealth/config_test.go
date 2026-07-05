package feedhealth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadThresholdConfigPerAssetClassNoCrossContamination verifies a config
// file declaring thresholds for two asset classes loads each independently.
func TestLoadThresholdConfigPerAssetClassNoCrossContamination(t *testing.T) {
	path := writeTempConfig(t, `
asset_classes:
  equity:
    expected_interval: 15s
    staleness_threshold: 60s
  option:
    expected_interval: 30s
    staleness_threshold: 120s
`)

	cfg, err := LoadThresholdConfig(path)
	require.NoError(t, err)

	require.Contains(t, cfg, "equity")
	require.Contains(t, cfg, "option")

	assert.Equal(t, 15*time.Second, cfg["equity"].ExpectedInterval)
	assert.Equal(t, 60*time.Second, cfg["equity"].StalenessThreshold)
	assert.Equal(t, 30*time.Second, cfg["option"].ExpectedInterval)
	assert.Equal(t, 120*time.Second, cfg["option"].StalenessThreshold)
}

// TestLoadThresholdConfigInvalidOrderingRejected verifies a config file
// declaring staleness_threshold <= expected_interval is rejected at load
// time and no threshold set is usable.
func TestLoadThresholdConfigInvalidOrderingRejected(t *testing.T) {
	path := writeTempConfig(t, `
asset_classes:
  equity:
    expected_interval: 60s
    staleness_threshold: 60s
`)

	cfg, err := LoadThresholdConfig(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidThresholdOrdering)
	assert.Nil(t, cfg)
}

// TestLoadThresholdConfigMissingFile verifies a nonexistent path returns
// ErrConfigNotFound.
func TestLoadThresholdConfigMissingFile(t *testing.T) {
	cfg, err := LoadThresholdConfig(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrConfigNotFound)
	assert.Nil(t, cfg)
}

// TestResolveConfigPathPrefersEnvVar verifies FEED_HEALTH_CONFIG_PATH, when
// set, is used verbatim over the TRADING_PROJECT_DIR fallback.
func TestResolveConfigPathPrefersEnvVar(t *testing.T) {
	t.Setenv("FEED_HEALTH_CONFIG_PATH", "/custom/path/feed-health-config.yaml")
	t.Setenv("TRADING_PROJECT_DIR", "/should/not/be/used")

	path, err := ResolveConfigPath()
	require.NoError(t, err)
	assert.Equal(t, "/custom/path/feed-health-config.yaml", path)
}

// TestResolveConfigPathFallsBackToProjectDir verifies the default path is
// derived from TRADING_PROJECT_DIR when FEED_HEALTH_CONFIG_PATH is unset.
func TestResolveConfigPathFallsBackToProjectDir(t *testing.T) {
	t.Setenv("FEED_HEALTH_CONFIG_PATH", "")
	t.Setenv("TRADING_PROJECT_DIR", "/repo/root")

	path, err := ResolveConfigPath()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/repo/root", "src", "go", "feed-health-config.yaml"), path)
}

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "feed-health-config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	return path
}
