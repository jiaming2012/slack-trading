package telemetry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeartbeatTracker_FreshToStaleTransition(t *testing.T) {
	tracker := NewHeartbeatTracker()
	threshold := 90 * time.Second

	beatAt := time.Now().UTC()
	tracker.Beat(SourceKindStrategy, "covered-call", nil, beatAt)
	tracker.Beat(SourceKindDatasource, "polygon-options", nil, beatAt)

	// Just after the beat: nothing is stale.
	assert.Empty(t, tracker.StaleSources(threshold, beatAt.Add(30*time.Second)))

	// One source keeps beating, the other goes silent.
	tracker.Beat(SourceKindDatasource, "polygon-options", nil, beatAt.Add(2*time.Minute))

	stale := tracker.StaleSources(threshold, beatAt.Add(2*time.Minute))
	require.Len(t, stale, 1, "only the silent source is stale")
	assert.Equal(t, "covered-call", stale[0].Name)
}

func TestHeartbeatTracker_BeatUpdatesMetaAndCount(t *testing.T) {
	tracker := NewHeartbeatTracker()

	tracker.Beat(SourceKindStrategy, "covered-call", map[string]string{"state": "idle"}, time.Now())
	tracker.Beat(SourceKindStrategy, "covered-call", map[string]string{"state": "active"}, time.Now())

	sources := tracker.Sources()
	require.Len(t, sources, 1)
	assert.Equal(t, int64(2), sources[0].BeatCount)
	assert.Equal(t, "active", sources[0].Meta["state"])
}

func TestHeartbeatTracker_SourcesAreCopies(t *testing.T) {
	tracker := NewHeartbeatTracker()
	tracker.Beat(SourceKindStrategy, "covered-call", map[string]string{"state": "idle"}, time.Now())

	sources := tracker.Sources()
	sources[0].Meta["state"] = "mutated"

	fresh := tracker.Sources()
	assert.Equal(t, "idle", fresh[0].Meta["state"])
}

func TestValidSourceKind(t *testing.T) {
	assert.True(t, ValidSourceKind("strategy"))
	assert.True(t, ValidSourceKind("datasource"))
	assert.True(t, ValidSourceKind("job"), "in-server scheduled jobs (fidelity monitor) heartbeat")
	assert.False(t, ValidSourceKind("server"), "the server does not heartbeat to itself")
	assert.False(t, ValidSourceKind(""))
}
