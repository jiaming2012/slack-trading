package telemetry

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Kind distinguishes how a series accumulates values.
type Kind string

var (
	KindCounter Kind = "counter"
	KindGauge   Kind = "gauge"
)

// Label is a single key/value dimension on a series.
type Label struct {
	Key   string
	Value string
}

// Point is a point-in-time reading of one series, as returned by Snapshot.
type Point struct {
	Name   string
	Labels map[string]string
	Kind   Kind
	Value  float64
	At     time.Time
}

type series struct {
	labels map[string]string
	kind   Kind
	value  float64
}

// Registry is the in-process metric store. Recording is a synchronous
// function call under a mutex — there is no batching or export pipeline.
type Registry struct {
	mu     sync.Mutex
	series map[string]*series
}

func NewRegistry() *Registry {
	return &Registry{series: make(map[string]*series)}
}

// seriesKey builds a deterministic map key from a metric name and its labels.
func seriesKey(name string, labels []Label) string {
	if len(labels) == 0 {
		return name
	}
	sorted := make([]Label, len(labels))
	copy(sorted, labels)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	var b strings.Builder
	b.WriteString(name)
	for _, l := range sorted {
		b.WriteByte('|')
		b.WriteString(l.Key)
		b.WriteByte('=')
		b.WriteString(l.Value)
	}
	return b.String()
}

func (r *Registry) record(name string, kind Kind, labels []Label, apply func(prev float64) float64) {
	key := seriesKey(name, labels)

	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.series[key]
	if !ok {
		labelMap := make(map[string]string, len(labels))
		for _, l := range labels {
			labelMap[l.Key] = l.Value
		}
		s = &series{labels: labelMap, kind: kind}
		r.series[key] = s
	}
	s.value = apply(s.value)
}

// Snapshot returns a point-in-time copy of every live series.
func (r *Registry) Snapshot() []Point {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	points := make([]Point, 0, len(r.series))
	for key, s := range r.series {
		name := key
		if idx := strings.IndexByte(key, '|'); idx >= 0 {
			name = key[:idx]
		}
		labelsCopy := make(map[string]string, len(s.labels))
		for k, v := range s.labels {
			labelsCopy[k] = v
		}
		points = append(points, Point{
			Name:   name,
			Labels: labelsCopy,
			Kind:   s.kind,
			Value:  s.value,
			At:     now,
		})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Name < points[j].Name })
	return points
}

// Counter is a monotonically increasing instrument.
type Counter struct {
	name string
	reg  *Registry
}

func (r *Registry) Counter(name string) *Counter {
	return &Counter{name: name, reg: r}
}

// Add increments the counter's series for the given labels. Negative deltas
// are ignored — counters are monotonic.
func (c *Counter) Add(delta int64, labels ...Label) {
	if c == nil || delta <= 0 {
		return
	}
	c.reg.record(c.name, KindCounter, labels, func(prev float64) float64 {
		return prev + float64(delta)
	})
}

// Gauge is a last-value instrument.
type Gauge struct {
	name string
	reg  *Registry
}

func (r *Registry) Gauge(name string) *Gauge {
	return &Gauge{name: name, reg: r}
}

// Set records the gauge's current value for the given labels.
func (g *Gauge) Set(value float64, labels ...Label) {
	if g == nil {
		return
	}
	g.reg.record(g.name, KindGauge, labels, func(prev float64) float64 {
		return value
	})
}
