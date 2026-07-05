package telemetry

import (
	"context"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

// HeartbeatStats contains aggregated playground statistics for the heartbeat.
type HeartbeatStats struct {
	LiveCount      int
	ReconcileCount int
	SimulatorCount int
	OpenOrderCount int
	LastTickTime   time.Time
}

// StatsProvider is a function that returns current heartbeat stats.
// Typically backed by DatabaseService.GetHeartbeatStats.
type StatsProvider func() HeartbeatStats

// EmitHeartbeatLog writes a structured log line with heartbeat stats.
func EmitHeartbeatLog(stats HeartbeatStats, startTime time.Time) {
	uptime := time.Since(startTime).Seconds()

	logFields := log.Fields{
		"event":                 "heartbeat",
		"live_playgrounds":      stats.LiveCount,
		"reconcile_playgrounds": stats.ReconcileCount,
		"simulator_playgrounds": stats.SimulatorCount,
		"open_orders":           stats.OpenOrderCount,
		"uptime_seconds":        int64(uptime),
	}

	if !stats.LastTickTime.IsZero() {
		logFields["last_tick_time"] = stats.LastTickTime.Format(time.RFC3339)
	}

	log.WithFields(logFields).Info("server heartbeat")
}

// emitHeartbeatMetrics records the server-stats gauges into the registry.
func emitHeartbeatMetrics(stats HeartbeatStats, startTime time.Time) {
	uptime := time.Since(startTime).Seconds()

	ActivePlaygrounds.Set(float64(stats.LiveCount), Label{Key: "mode", Value: "live"})
	ActivePlaygrounds.Set(float64(stats.ReconcileCount), Label{Key: "mode", Value: "reconcile"})
	ActivePlaygrounds.Set(float64(stats.SimulatorCount), Label{Key: "mode", Value: "simulator"})

	OpenOrders.Set(float64(stats.OpenOrderCount))

	hostname, _ := os.Hostname()
	UptimeSeconds.Set(uptime, Label{Key: "host", Value: hostname})
}

// StartHeartbeat runs a background loop that records server-stats gauges and
// emits a structured log line every 30 seconds. It stops cleanly when the
// context is cancelled. This is server *stats* reporting, not a liveness
// heartbeat: per ADR-0005 the server does not heartbeat to itself.
func StartHeartbeat(ctx context.Context, getStats StatsProvider, startTime time.Time) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("heartbeat: stopping")
			return
		case <-ticker.C:
			stats := getStats()
			emitHeartbeatMetrics(stats, startTime)
			EmitHeartbeatLog(stats, startTime)
		}
	}
}
