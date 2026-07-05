package feedhealth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testThresholds() ThresholdConfig {
	return ThresholdConfig{
		"equity": {ExpectedInterval: 15 * time.Second, StalenessThreshold: 60 * time.Second},
		"option": {ExpectedInterval: 30 * time.Second, StalenessThreshold: 120 * time.Second},
	}
}

// TestObservePerAssetClassIsolation verifies observing a tick for one asset
// class does not affect another, unobserved asset class.
func TestObservePerAssetClassIsolation(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	t1 := clock.Now()
	m.Observe("equity", t1)

	got, ok := m.LastObserved("equity")
	require.True(t, ok)
	assert.True(t, t1.Equal(got))

	_, ok = m.LastObserved("option")
	assert.False(t, ok, "option should report no observation has occurred")
}

// TestLaterObservationReplacesTimestamp verifies a subsequent Observe call
// with a later timestamp replaces the recorded value for that asset class.
func TestLaterObservationReplacesTimestamp(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	t1 := clock.Now()
	t2 := t1.Add(5 * time.Second)

	m.Observe("equity", t1)
	m.Observe("equity", t2)

	got, ok := m.LastObserved("equity")
	require.True(t, ok)
	assert.True(t, t2.Equal(got))
}

// TestStatusHealthyWithinExpectedInterval verifies elapsed time within
// expected_interval reports healthy.
func TestStatusHealthyWithinExpectedInterval(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	t0 := clock.Now()
	m.Observe("equity", t0)
	clock.Advance(10 * time.Second)

	assert.Equal(t, Healthy, m.Status("equity"))
}

// TestStatusDegradedBetweenThresholds verifies elapsed time between the two
// thresholds reports degraded.
func TestStatusDegradedBetweenThresholds(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	m.Observe("equity", clock.Now())
	clock.Advance(30 * time.Second)

	assert.Equal(t, Degraded, m.Status("equity"))
}

// TestStatusStaleBeyondStalenessThreshold verifies elapsed time beyond
// staleness_threshold reports stale.
func TestStatusStaleBeyondStalenessThreshold(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	m.Observe("equity", clock.Now())
	clock.Advance(90 * time.Second)

	assert.Equal(t, Stale, m.Status("equity"))
}

// TestStatusNeverObservedIsStale verifies an asset class with no Observe
// call ever made reports stale.
func TestStatusNeverObservedIsStale(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	assert.Equal(t, Stale, m.Status("option"))
}

// TestShouldPauseScanningTrueWhenStale verifies ShouldPauseScanning returns
// true while an asset class's status is stale.
func TestShouldPauseScanningTrueWhenStale(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	m.Observe("equity", clock.Now())
	clock.Advance(90 * time.Second)

	assert.True(t, m.ShouldPauseScanning("equity"))
}

// TestShouldPauseScanningFalseWhenDegraded verifies ShouldPauseScanning
// returns false while an asset class's status is degraded (not stale).
func TestShouldPauseScanningFalseWhenDegraded(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	m.Observe("equity", clock.Now())
	clock.Advance(30 * time.Second)

	assert.False(t, m.ShouldPauseScanning("equity"))
}

// TestAutoPauseClearsAutomaticallyOnFreshObserve verifies that once a stale
// asset class receives a fresh Observe call, the next ShouldPauseScanning
// call returns false with no explicit acknowledgment or reset call.
func TestAutoPauseClearsAutomaticallyOnFreshObserve(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	m := NewHeartbeatMonitor(clock, testThresholds())

	m.Observe("equity", clock.Now())
	clock.Advance(90 * time.Second)
	require.True(t, m.ShouldPauseScanning("equity"), "precondition: asset class should be stale")

	m.Observe("equity", clock.Now())

	assert.False(t, m.ShouldPauseScanning("equity"), "auto-pause should clear automatically on fresh observation")
}
