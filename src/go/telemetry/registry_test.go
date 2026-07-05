package telemetry

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func snapshotValue(t *testing.T, points []Point, name string, labels map[string]string) float64 {
	t.Helper()
	for _, p := range points {
		if p.Name != name {
			continue
		}
		match := len(p.Labels) == len(labels)
		for k, v := range labels {
			if p.Labels[k] != v {
				match = false
				break
			}
		}
		if match {
			return p.Value
		}
	}
	t.Fatalf("no series %s with labels %v in snapshot", name, labels)
	return 0
}

func TestCounterIncrementsSynchronously(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("test.counter")

	c.Add(1, Label{Key: "mode", Value: "live"})
	c.Add(2, Label{Key: "mode", Value: "live"})

	got := snapshotValue(t, r.Snapshot(), "test.counter", map[string]string{"mode": "live"})
	assert.Equal(t, float64(3), got, "counter reflects increments immediately")
}

func TestCounterIgnoresNonPositiveDeltas(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("test.counter")

	c.Add(5)
	c.Add(-3)
	c.Add(0)

	got := snapshotValue(t, r.Snapshot(), "test.counter", nil)
	assert.Equal(t, float64(5), got, "counters are monotonic")
}

func TestGaugeRecordsLastValuePerLabelSet(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("test.gauge")

	g.Set(10, Label{Key: "mode", Value: "live"})
	g.Set(20, Label{Key: "mode", Value: "live"})
	g.Set(7, Label{Key: "mode", Value: "simulator"})

	points := r.Snapshot()
	assert.Equal(t, float64(20), snapshotValue(t, points, "test.gauge", map[string]string{"mode": "live"}))
	assert.Equal(t, float64(7), snapshotValue(t, points, "test.gauge", map[string]string{"mode": "simulator"}))
}

func TestSeriesKeyIsLabelOrderInsensitive(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("test.counter")

	c.Add(1, Label{Key: "a", Value: "1"}, Label{Key: "b", Value: "2"})
	c.Add(1, Label{Key: "b", Value: "2"}, Label{Key: "a", Value: "1"})

	points := r.Snapshot()
	require.Len(t, points, 1, "same labels in any order are one series")
	assert.Equal(t, float64(2), points[0].Value)
}

func TestSnapshotContainsOneEntryPerSeries(t *testing.T) {
	r := NewRegistry()
	r.Counter("a.counter").Add(1)
	r.Gauge("b.gauge").Set(2, Label{Key: "host", Value: "x"})

	points := r.Snapshot()
	require.Len(t, points, 2)
	assert.Equal(t, "a.counter", points[0].Name)
	assert.Equal(t, KindCounter, points[0].Kind)
	assert.Equal(t, "b.gauge", points[1].Name)
	assert.Equal(t, KindGauge, points[1].Kind)
	assert.False(t, points[0].At.IsZero())
}

func TestSnapshotReturnsCopies(t *testing.T) {
	r := NewRegistry()
	r.Gauge("g").Set(1, Label{Key: "k", Value: "v"})

	points := r.Snapshot()
	points[0].Labels["k"] = "mutated"

	fresh := r.Snapshot()
	assert.Equal(t, "v", fresh[0].Labels["k"], "snapshot mutation must not leak into the registry")
}

func TestNilInstrumentsAreSafe(t *testing.T) {
	var c *Counter
	var g *Gauge

	assert.NotPanics(t, func() {
		c.Add(1)
		g.Set(2)
	})
}

func TestConcurrentRecording(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("test.concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Add(1, Label{Key: "mode", Value: "live"})
			}
		}()
	}
	wg.Wait()

	got := snapshotValue(t, r.Snapshot(), "test.concurrent", map[string]string{"mode": "live"})
	assert.Equal(t, float64(5000), got)
}
