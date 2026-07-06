package safety

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Environment variables carrying the anomaly-guard thresholds. Every variable
// is optional (the documented conservative default applies when unset), accepts
// the sentinel "off" to disable its guard entirely, and refuses startup on any
// other unparseable value — a silently-misconfigured guard is worse than a
// refused boot.
const (
	EnvGuardRejectionWindow        = "GUARD_REJECTION_WINDOW"
	EnvGuardRejectionThreshold     = "GUARD_REJECTION_THRESHOLD"
	EnvGuardRejectionMinSamples    = "GUARD_REJECTION_MIN_SAMPLES"
	EnvGuardFillDeviationPct       = "GUARD_FILL_DEVIATION_PCT"
	EnvGuardTradesPerHourMean      = "GUARD_TRADES_PER_HOUR_MEAN"
	EnvGuardTradesPerHourStdDev    = "GUARD_TRADES_PER_HOUR_STDDEV"
	EnvGuardTradesPerHourMinSamp   = "GUARD_TRADES_PER_HOUR_MIN_SAMPLES"
	EnvGuardFeedStalenessThreshold = "GUARD_FEED_STALENESS_THRESHOLD"
	EnvGuardStalenessEvalInterval  = "GUARD_STALENESS_EVAL_INTERVAL"
)

// guardOffSentinel disables an individual guard when supplied as the value of
// any of that guard's environment variables.
const guardOffSentinel = "off"

// Conservative defaults (design decision 6 of wire-anomaly-guard-feeds).
const (
	DefaultRejectionWindow         = 10 * time.Minute
	DefaultRejectionThreshold      = 0.5
	DefaultRejectionMinSamples     = 5
	DefaultFillDeviationPct        = 0.05
	DefaultTradesPerHourMean       = 0.0 // 0/0 => trades-per-hour guard unarmed
	DefaultTradesPerHourStdDev     = 0.0
	DefaultTradesPerHourMinSamples = 5
	DefaultFeedStalenessThreshold  = 5 * time.Minute
	DefaultStalenessEvalInterval   = 30 * time.Second
)

// GuardEnvConfig is the effective guard configuration parsed from the
// environment: the thresholds plus a per-guard enable flag ("off" sentinel)
// and the staleness evaluation ticker interval.
type GuardEnvConfig struct {
	Guards GuardConfig

	RejectionEnabled     bool
	FillDeviationEnabled bool
	TradesPerHourEnabled bool
	FeedStalenessEnabled bool

	// StalenessEvalInterval is the period of the feed-staleness evaluation
	// ticker.
	StalenessEvalInterval time.Duration
}

// LoadGuardEnvConfig reads the GUARD_* environment variables, applying the
// documented conservative defaults for unset variables, disabling a guard when
// any of its variables carries the "off" sentinel, and returning an error
// naming the variable for any unparseable or out-of-range value (callers
// Fatal: an invalid guard configuration must refuse startup).
func LoadGuardEnvConfig() (GuardEnvConfig, error) {
	cfg := GuardEnvConfig{
		RejectionEnabled:     true,
		FillDeviationEnabled: true,
		TradesPerHourEnabled: true,
		FeedStalenessEnabled: true,
	}

	var err error

	// Rejection-rate guard.
	if cfg.Guards.RejectionWindow, err = envDuration(EnvGuardRejectionWindow, DefaultRejectionWindow, &cfg.RejectionEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.Guards.RejectionThreshold, err = envFloat(EnvGuardRejectionThreshold, DefaultRejectionThreshold, &cfg.RejectionEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.Guards.RejectionMinSamples, err = envInt(EnvGuardRejectionMinSamples, DefaultRejectionMinSamples, &cfg.RejectionEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.RejectionEnabled {
		if cfg.Guards.RejectionThreshold <= 0 || cfg.Guards.RejectionThreshold > 1 {
			return GuardEnvConfig{}, fmt.Errorf("guard config: %s must be a fraction in (0,1], got %v", EnvGuardRejectionThreshold, cfg.Guards.RejectionThreshold)
		}
		if cfg.Guards.RejectionWindow <= 0 {
			return GuardEnvConfig{}, fmt.Errorf("guard config: %s must be a positive duration, got %v", EnvGuardRejectionWindow, cfg.Guards.RejectionWindow)
		}
		if cfg.Guards.RejectionMinSamples < 1 {
			return GuardEnvConfig{}, fmt.Errorf("guard config: %s must be >= 1, got %d", EnvGuardRejectionMinSamples, cfg.Guards.RejectionMinSamples)
		}
	}

	// Fill-deviation guard.
	if cfg.Guards.FillDeviationPct, err = envFloat(EnvGuardFillDeviationPct, DefaultFillDeviationPct, &cfg.FillDeviationEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.FillDeviationEnabled && cfg.Guards.FillDeviationPct <= 0 {
		return GuardEnvConfig{}, fmt.Errorf("guard config: %s must be a positive fraction, got %v", EnvGuardFillDeviationPct, cfg.Guards.FillDeviationPct)
	}

	// Trades-per-hour guard.
	if cfg.Guards.TradesPerHourMean, err = envFloat(EnvGuardTradesPerHourMean, DefaultTradesPerHourMean, &cfg.TradesPerHourEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.Guards.TradesPerHourStdDev, err = envFloat(EnvGuardTradesPerHourStdDev, DefaultTradesPerHourStdDev, &cfg.TradesPerHourEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.Guards.TradesPerHourMinSamples, err = envInt(EnvGuardTradesPerHourMinSamp, DefaultTradesPerHourMinSamples, &cfg.TradesPerHourEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.TradesPerHourEnabled {
		if cfg.Guards.TradesPerHourMean < 0 || cfg.Guards.TradesPerHourStdDev < 0 {
			return GuardEnvConfig{}, fmt.Errorf("guard config: %s / %s must be non-negative, got mean=%v stddev=%v", EnvGuardTradesPerHourMean, EnvGuardTradesPerHourStdDev, cfg.Guards.TradesPerHourMean, cfg.Guards.TradesPerHourStdDev)
		}
		if cfg.Guards.TradesPerHourMinSamples < 1 {
			return GuardEnvConfig{}, fmt.Errorf("guard config: %s must be >= 1, got %d", EnvGuardTradesPerHourMinSamp, cfg.Guards.TradesPerHourMinSamples)
		}
	}

	// Feed-staleness guard.
	if cfg.Guards.FeedStalenessThreshold, err = envDuration(EnvGuardFeedStalenessThreshold, DefaultFeedStalenessThreshold, &cfg.FeedStalenessEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.StalenessEvalInterval, err = envDuration(EnvGuardStalenessEvalInterval, DefaultStalenessEvalInterval, &cfg.FeedStalenessEnabled); err != nil {
		return GuardEnvConfig{}, err
	}
	if cfg.FeedStalenessEnabled {
		if cfg.Guards.FeedStalenessThreshold <= 0 {
			return GuardEnvConfig{}, fmt.Errorf("guard config: %s must be a positive duration, got %v", EnvGuardFeedStalenessThreshold, cfg.Guards.FeedStalenessThreshold)
		}
		if cfg.StalenessEvalInterval <= 0 {
			return GuardEnvConfig{}, fmt.Errorf("guard config: %s must be a positive duration, got %v", EnvGuardStalenessEvalInterval, cfg.StalenessEvalInterval)
		}
	}

	return cfg, nil
}

// LogEffective logs the effective guard configuration at startup: one Info
// line per armed guard and a loud Warn for every disabled one.
func (c GuardEnvConfig) LogEffective() {
	if c.RejectionEnabled {
		log.Infof("safety: rejection-rate guard armed (window=%s threshold=%.0f%% min-samples=%d)", c.Guards.RejectionWindow, c.Guards.RejectionThreshold*100, c.Guards.RejectionMinSamples)
	} else {
		log.Warnf("safety: rejection-rate guard DISABLED via %s=off", EnvGuardRejectionThreshold)
	}

	if c.FillDeviationEnabled {
		log.Infof("safety: fill-deviation guard armed (threshold=%.2f%%)", c.Guards.FillDeviationPct*100)
	} else {
		log.Warnf("safety: fill-deviation guard DISABLED via %s=off", EnvGuardFillDeviationPct)
	}

	if c.TradesPerHourEnabled {
		if c.Guards.TradesPerHourMean == 0 && c.Guards.TradesPerHourStdDev == 0 {
			log.Warnf("safety: trades-per-hour guard enabled but UNARMED — no historical norm supplied; set %s / %s to arm it", EnvGuardTradesPerHourMean, EnvGuardTradesPerHourStdDev)
		} else {
			log.Infof("safety: trades-per-hour guard armed (mean=%.2f stddev=%.2f min-samples=%d)", c.Guards.TradesPerHourMean, c.Guards.TradesPerHourStdDev, c.Guards.TradesPerHourMinSamples)
		}
	} else {
		log.Warnf("safety: trades-per-hour guard DISABLED via %s=off", EnvGuardTradesPerHourMean)
	}

	if c.FeedStalenessEnabled {
		log.Infof("safety: feed-staleness guard armed (threshold=%s, evaluated every %s during market hours with a live playground)", c.Guards.FeedStalenessThreshold, c.StalenessEvalInterval)
	} else {
		log.Warnf("safety: feed-staleness guard DISABLED via %s=off", EnvGuardFeedStalenessThreshold)
	}
}

// envDuration reads a duration env var: unset returns def, "off" flips
// disabled and returns def, anything else must parse via time.ParseDuration.
func envDuration(name string, def time.Duration, enabled *bool) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def, nil
	}
	if strings.EqualFold(raw, guardOffSentinel) {
		*enabled = false
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("guard config: %s=%q is not a valid duration (e.g. \"10m\", \"30s\") or \"off\": %w", name, raw, err)
	}
	return d, nil
}

// envFloat reads a float env var with the same unset/"off" semantics.
func envFloat(name string, def float64, enabled *bool) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def, nil
	}
	if strings.EqualFold(raw, guardOffSentinel) {
		*enabled = false
		return def, nil
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("guard config: %s=%q is not a valid number or \"off\": %w", name, raw, err)
	}
	return f, nil
}

// envInt reads an integer env var with the same unset/"off" semantics.
func envInt(name string, def int, enabled *bool) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def, nil
	}
	if strings.EqualFold(raw, guardOffSentinel) {
		*enabled = false
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("guard config: %s=%q is not a valid integer or \"off\": %w", name, raw, err)
	}
	return n, nil
}
