package telemetry

import (
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

// Operator-tunable knobs. Hardcoded defaults with env overrides only —
// the options-config YAML stays trading-only (design D6).
const (
	defaultSnapshotInterval   = 60 * time.Second
	defaultHeartbeatStale     = 90 * time.Second
	defaultErrorWindow        = 5 * time.Minute
	defaultErrorThreshold     = 10
	defaultRenotifyInterval   = 30 * time.Minute
	defaultRetention          = 30 * 24 * time.Hour
	defaultEvaluationInterval = 30 * time.Second
	defaultDegradedWindow     = 5 * time.Minute
	defaultDegradedThreshold  = 10
)

func envDuration(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Warnf("telemetry: invalid %s=%q, using default %s", name, raw, fallback)
		return fallback
	}
	return d
}

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Warnf("telemetry: invalid %s=%q, using default %d", name, raw, fallback)
		return fallback
	}
	return n
}

// SnapshotInterval is the cadence of the metric snapshot writer.
func SnapshotInterval() time.Duration {
	return envDuration("TELEMETRY_SNAPSHOT_INTERVAL", defaultSnapshotInterval)
}

// HeartbeatStaleAfter is how long a source may go silent before it is stale.
func HeartbeatStaleAfter() time.Duration {
	return envDuration("TELEMETRY_HEARTBEAT_STALE_AFTER", defaultHeartbeatStale)
}

// ErrorWindow is the rolling window for the error-rate alert rule.
func ErrorWindow() time.Duration {
	return envDuration("TELEMETRY_ERROR_WINDOW", defaultErrorWindow)
}

// ErrorThreshold is the error count within ErrorWindow that fires an alert.
func ErrorThreshold() int {
	return envInt("TELEMETRY_ERROR_THRESHOLD", defaultErrorThreshold)
}

// RenotifyInterval is how often a firing, unacknowledged alert re-posts.
func RenotifyInterval() time.Duration {
	return envDuration("TELEMETRY_ALERT_RENOTIFY_INTERVAL", defaultRenotifyInterval)
}

// RiskOverlayDegradedWindow is the rolling window for the riskoverlay
// degradation alert rule (wire-risk-overlay-state).
func RiskOverlayDegradedWindow() time.Duration {
	return envDuration("TELEMETRY_RISKOVERLAY_DEGRADED_WINDOW", defaultDegradedWindow)
}

// RiskOverlayDegradedThreshold is the degradation count within
// RiskOverlayDegradedWindow that fires the riskoverlay_degraded alert.
func RiskOverlayDegradedThreshold() int {
	return envInt("TELEMETRY_RISKOVERLAY_DEGRADED_THRESHOLD", defaultDegradedThreshold)
}
