package telemetry

import (
	"bytes"
	"context"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

)

func TestHeartbeatStatsComputation(t *testing.T) {
	t.Run("correct counts for mix of live, reconcile, and simulator playgrounds", func(t *testing.T) {
		stats := HeartbeatStats{
			LiveCount:      3,
			ReconcileCount: 2,
			SimulatorCount: 5,
			OpenOrderCount: 7,
			LastTickTime:   time.Date(2025, 3, 26, 14, 30, 0, 0, time.UTC),
		}

		assert.Equal(t, 3, stats.LiveCount)
		assert.Equal(t, 2, stats.ReconcileCount)
		assert.Equal(t, 5, stats.SimulatorCount)
		assert.Equal(t, 7, stats.OpenOrderCount)
		assert.False(t, stats.LastTickTime.IsZero())
	})

	t.Run("counts open orders only from live/reconcile playgrounds", func(t *testing.T) {
		// Verify ShouldEmitOrderTelemetry filters correctly
		assert.True(t, ShouldEmitOrderTelemetry("live"))
		assert.True(t, ShouldEmitOrderTelemetry("reconcile"))
		assert.False(t, ShouldEmitOrderTelemetry("simulator"))
	})

	t.Run("zero counts when no playgrounds exist", func(t *testing.T) {
		stats := HeartbeatStats{}

		assert.Equal(t, 0, stats.LiveCount)
		assert.Equal(t, 0, stats.ReconcileCount)
		assert.Equal(t, 0, stats.SimulatorCount)
		assert.Equal(t, 0, stats.OpenOrderCount)
		assert.True(t, stats.LastTickTime.IsZero())
	})
}

func TestHeartbeatStructuredLog(t *testing.T) {
	t.Run("emits structured log with expected fields", func(t *testing.T) {
		// Capture log output
		var buf bytes.Buffer
		origOutput := log.StandardLogger().Out
		origFormatter := log.StandardLogger().Formatter
		log.SetOutput(&buf)
		log.SetFormatter(&log.JSONFormatter{})
		defer func() {
			log.SetOutput(origOutput)
			log.SetFormatter(origFormatter)
		}()

		tickTime := time.Date(2025, 3, 26, 14, 30, 0, 0, time.UTC)
		stats := HeartbeatStats{
			LiveCount:      2,
			ReconcileCount: 1,
			SimulatorCount: 4,
			OpenOrderCount: 3,
			LastTickTime:   tickTime,
		}

		startTime := time.Now().Add(-60 * time.Second)
		EmitHeartbeatLog(stats, startTime)

		output := buf.String()
		assert.Contains(t, output, "server heartbeat")
		assert.Contains(t, output, "\"event\":\"heartbeat\"")
		assert.Contains(t, output, "\"live_playgrounds\":2")
		assert.Contains(t, output, "\"reconcile_playgrounds\":1")
		assert.Contains(t, output, "\"simulator_playgrounds\":4")
		assert.Contains(t, output, "\"open_orders\":3")
		assert.Contains(t, output, "\"uptime_seconds\":")
		assert.Contains(t, output, "\"last_tick_time\":")
	})
}

func TestHeartbeatGoroutineStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	provider := func() HeartbeatStats {
		return HeartbeatStats{}
	}

	done := make(chan struct{})
	go func() {
		StartHeartbeat(ctx, provider, time.Now())
		close(done)
	}()

	// Cancel immediately -- goroutine should exit quickly
	cancel()

	select {
	case <-done:
		// Success: goroutine exited
	case <-time.After(2 * time.Second):
		require.Fail(t, "StartHeartbeat did not stop within 2 seconds after context cancellation")
	}
}
