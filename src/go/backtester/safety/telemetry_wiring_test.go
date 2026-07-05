package safety

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// haltEngagedValue reads the safety_halt_engaged gauge from the default
// registry snapshot; ok is false when the series has never been recorded.
func haltEngagedValue(t *testing.T) (float64, bool) {
	t.Helper()
	for _, p := range telemetry.Default.Snapshot() {
		if p.Name == "safety_halt_engaged" {
			return p.Value, true
		}
	}
	return 0, false
}

func TestInstallHaltTelemetry_GaugeTracksTransitions(t *testing.T) {
	telemetry.Init()

	c := newTestController(t)
	InstallHaltTelemetry(c)

	// Startup restore of a clear controller records 0.
	v, ok := haltEngagedValue(t)
	require.True(t, ok, "the gauge must be set at install time from the restored state")
	require.Equal(t, 0.0, v)

	// A guard trip (auto engage) sets the gauge to 1.
	require.NoError(t, c.EngageAuto("rejection-rate guard: test trip"))
	v, _ = haltEngagedValue(t)
	require.Equal(t, 1.0, v)

	// Acknowledge keeps the halt engaged: still 1.
	require.NoError(t, c.Acknowledge())
	v, _ = haltEngagedValue(t)
	require.Equal(t, 1.0, v)

	// Release clears the gauge.
	require.NoError(t, c.Release())
	v, _ = haltEngagedValue(t)
	require.Equal(t, 0.0, v)
}

func TestInstallHaltTelemetry_StartupRestoreOfEngagedHaltSetsGauge(t *testing.T) {
	telemetry.Init()

	store := NewMemoryHaltStore()
	c1, err := NewHaltController(store)
	require.NoError(t, err)
	require.NoError(t, c1.EngageAuto("feed-staleness guard: pre-restart trip"))

	// Simulated restart: a fresh controller restored from the same store must
	// report engaged on the gauge as soon as telemetry is installed.
	c2, err := NewHaltController(store)
	require.NoError(t, err)
	InstallHaltTelemetry(c2)

	v, ok := haltEngagedValue(t)
	require.True(t, ok)
	require.Equal(t, 1.0, v)
}

// A guard tripping through the registry (the production path) must move the
// gauge — this pins the listener to the EngageAuto path the guards use.
func TestInstallHaltTelemetry_GuardTripSetsGauge(t *testing.T) {
	telemetry.Init()

	c := newTestController(t)
	InstallHaltTelemetry(c)

	g := NewFillDeviationGuard(0.02, c)
	tripped, _ := g.ObserveFill(100.0, 150.0)
	require.True(t, tripped)

	v, _ := haltEngagedValue(t)
	require.Equal(t, 1.0, v)
	require.Equal(t, SourceAuto, c.Status().Source)
}
