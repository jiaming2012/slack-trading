package fidelity

import (
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

// Operator-tunable monitor knobs. Hardcoded defaults with env overrides only,
// following the telemetry config pattern — the options-config YAML stays
// trading-only.
const (
	defaultMonitorInterval  = 24 * time.Hour  // evaluate once a day…
	defaultMonitorPeriod    = 168 * time.Hour // …over the trailing week
	defaultMonitorKeepalive = 60 * time.Second
)

func envDuration(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Warnf("fidelity: invalid %s=%q, using default %s", name, raw, fallback)
		return fallback
	}
	return d
}

func envBool(name string, fallback bool) bool {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		log.Warnf("fidelity: invalid %s=%q, using default %t", name, raw, fallback)
		return fallback
	}
	return b
}

// MonitorEnabled reports whether the scheduled fidelity monitor should run.
// Enabled by default; FIDELITY_MONITOR_ENABLED=false disables it (the
// documented rollback knob — everything else stays intact).
func MonitorEnabled() bool {
	return envBool("FIDELITY_MONITOR_ENABLED", true)
}

// MonitorInterval is how often the monitor evaluates fidelity (default daily).
func MonitorInterval() time.Duration {
	return envDuration("FIDELITY_CHECK_INTERVAL", defaultMonitorInterval)
}

// MonitorPeriod is the trailing window each evaluation covers (default the
// last 7 days, ending at the evaluation time). Evaluating the weekly window
// daily keeps the architecture doc's week-over-week comparison while
// detecting degradation up to six days earlier.
func MonitorPeriod() time.Duration {
	return envDuration("FIDELITY_PERIOD", defaultMonitorPeriod)
}
