package safety

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EnvCompanionStopDistance carries the broker-side companion-stop distance
// (an absolute price distance from the fill price). The feature is explicit
// opt-in: when the variable is unset, companion-stop placement is disabled
// entirely — a guessed default distance would silently place mispriced stops
// on real accounts, which is more dangerous than no stop plus a loud warning.
const EnvCompanionStopDistance = "COMPANION_STOP_DISTANCE"

// LoadCompanionStopEnvConfig reads COMPANION_STOP_DISTANCE.
//
//   - Unset (or blank): returns (nil, nil) — the feature is disabled; the
//     caller logs the loud startup warning.
//   - Unparseable or non-positive: returns an error — the caller must refuse
//     startup rather than error on every live fill.
//   - Positive: returns the validated CompanionStopConfig to install for the
//     live fill pipeline.
func LoadCompanionStopEnvConfig() (*CompanionStopConfig, error) {
	raw := strings.TrimSpace(os.Getenv(EnvCompanionStopDistance))
	if raw == "" {
		return nil, nil
	}

	distance, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("companion-stop config: %s=%q is not a valid number: %w", EnvCompanionStopDistance, raw, err)
	}

	cfg := CompanionStopConfig{StopDistance: distance}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("companion-stop config: %s: %w", EnvCompanionStopDistance, err)
	}

	return &cfg, nil
}
