package telemetry

import (
	"context"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
		"event":                   "heartbeat",
		"live_playgrounds":       stats.LiveCount,
		"reconcile_playgrounds": stats.ReconcileCount,
		"simulator_playgrounds": stats.SimulatorCount,
		"open_orders":            stats.OpenOrderCount,
		"uptime_seconds":         int64(uptime),
	}

	if !stats.LastTickTime.IsZero() {
		logFields["last_tick_time"] = stats.LastTickTime.Format(time.RFC3339)
	}

	log.WithFields(logFields).Info("server heartbeat")
}

// emitHeartbeatMetrics records OTel gauge metrics for the heartbeat.
func emitHeartbeatMetrics(ctx context.Context, stats HeartbeatStats, startTime time.Time) {
	uptime := time.Since(startTime).Seconds()

	if ActivePlaygrounds != nil {
		ActivePlaygrounds.Record(ctx, int64(stats.LiveCount),
			metric.WithAttributes(
				attribute.String("environment", "live"),
			))
		ActivePlaygrounds.Record(ctx, int64(stats.ReconcileCount),
			metric.WithAttributes(
				attribute.String("environment", "reconcile"),
			))
		ActivePlaygrounds.Record(ctx, int64(stats.SimulatorCount),
			metric.WithAttributes(
				attribute.String("environment", "simulator"),
			))
	}

	if OpenOrders != nil {
		OpenOrders.Record(ctx, int64(stats.OpenOrderCount))
	}

	if UptimeSeconds != nil {
		hostname, _ := os.Hostname()
		UptimeSeconds.Record(ctx, uptime,
			metric.WithAttributes(
				attribute.String("host", hostname),
			))
	}
}

// StartHeartbeat runs a background loop that emits OTel gauge metrics and
// structured logs every 30 seconds with live playground stats.
// It stops cleanly when the context is cancelled.
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
			emitHeartbeatMetrics(ctx, stats, startTime)
			EmitHeartbeatLog(stats, startTime)
		}
	}
}
