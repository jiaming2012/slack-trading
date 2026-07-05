package feedhealth

import (
	"fmt"
	"os"
	"path"
	"time"

	"gopkg.in/yaml.v3"
)

// AssetClassThresholds holds the healthy boundary (expected_interval) and the
// auto-pause boundary (staleness_threshold) for a single asset class.
type AssetClassThresholds struct {
	ExpectedInterval   time.Duration
	StalenessThreshold time.Duration
}

// ThresholdConfig maps asset class name (e.g. "equity", "option") to its
// configured thresholds.
type ThresholdConfig map[string]AssetClassThresholds

// thresholdConfigYAML mirrors the on-disk YAML shape:
//
//	asset_classes:
//	  equity:
//	    expected_interval: 15s
//	    staleness_threshold: 60s
type thresholdConfigYAML struct {
	AssetClasses map[string]struct {
		ExpectedInterval   time.Duration `yaml:"expected_interval"`
		StalenessThreshold time.Duration `yaml:"staleness_threshold"`
	} `yaml:"asset_classes"`
}

// LoadThresholdConfig reads and parses the YAML file at path, validating that
// staleness_threshold > expected_interval for every configured asset class.
// It returns ErrConfigNotFound if the file cannot be read, and
// ErrInvalidThresholdOrdering if any asset class violates the ordering
// invariant.
func LoadThresholdConfig(path string) (ThresholdConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfigNotFound, path, err)
	}

	var parsed thresholdConfigYAML
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("feedhealth: failed to parse threshold config %s: %w", path, err)
	}

	cfg := make(ThresholdConfig, len(parsed.AssetClasses))
	for assetClass, thresholds := range parsed.AssetClasses {
		if thresholds.StalenessThreshold <= thresholds.ExpectedInterval {
			return nil, fmt.Errorf("%w: asset class %q has staleness_threshold=%s expected_interval=%s", ErrInvalidThresholdOrdering, assetClass, thresholds.StalenessThreshold, thresholds.ExpectedInterval)
		}

		cfg[assetClass] = AssetClassThresholds{
			ExpectedInterval:   thresholds.ExpectedInterval,
			StalenessThreshold: thresholds.StalenessThreshold,
		}
	}

	return cfg, nil
}

// ResolveConfigPath resolves the feed health config file path: the
// FEED_HEALTH_CONFIG_PATH env var if set, else
// ${TRADING_PROJECT_DIR}/src/go/feed-health-config.yaml, mirroring the
// existing OPTIONS_CONFIG_PATH / OPTIONS_CONFIG_FILE convention.
func ResolveConfigPath() (string, error) {
	if p := os.Getenv("FEED_HEALTH_CONFIG_PATH"); p != "" {
		return p, nil
	}

	projectDir := os.Getenv("TRADING_PROJECT_DIR")
	if projectDir == "" {
		return "", fmt.Errorf("feedhealth: neither FEED_HEALTH_CONFIG_PATH nor TRADING_PROJECT_DIR is set")
	}

	return path.Join(projectDir, "src", "go", "feed-health-config.yaml"), nil
}
