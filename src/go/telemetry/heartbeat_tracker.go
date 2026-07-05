package telemetry

import (
	"sort"
	"sync"
	"time"
)

// Heartbeat source kinds, per the CONTEXT.md glossary: each strategy and each
// datasource heartbeats. The server does not heartbeat to itself (ADR-0005).
const (
	SourceKindStrategy   = "strategy"
	SourceKindDatasource = "datasource"
)

// ValidSourceKind reports whether kind names a heartbeat source we accept.
func ValidSourceKind(kind string) bool {
	return kind == SourceKindStrategy || kind == SourceKindDatasource
}

// HeartbeatSource is the tracked liveness state of one strategy or datasource.
type HeartbeatSource struct {
	Kind      string
	Name      string
	Meta      map[string]string
	LastSeen  time.Time
	BeatCount int64
}

// Stale reports whether the source has been silent past the threshold.
func (s HeartbeatSource) Stale(threshold time.Duration, now time.Time) bool {
	return now.Sub(s.LastSeen) > threshold
}

// HeartbeatTracker keeps per-source last-seen state in memory so the alert
// rules can evaluate staleness without touching the database.
type HeartbeatTracker struct {
	mu      sync.Mutex
	sources map[string]*HeartbeatSource
}

// Heartbeats is the process-wide tracker, non-nil after Init().
var Heartbeats *HeartbeatTracker

func NewHeartbeatTracker() *HeartbeatTracker {
	return &HeartbeatTracker{sources: make(map[string]*HeartbeatSource)}
}

// Beat records a liveness signal from a source.
func (t *HeartbeatTracker) Beat(kind, name string, meta map[string]string, at time.Time) {
	key := kind + "/" + name

	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.sources[key]
	if !ok {
		s = &HeartbeatSource{Kind: kind, Name: name}
		t.sources[key] = s
	}
	s.Meta = meta
	s.LastSeen = at
	s.BeatCount++
}

// Sources returns a copy of all tracked sources, ordered by kind then name.
func (t *HeartbeatTracker) Sources() []HeartbeatSource {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]HeartbeatSource, 0, len(t.sources))
	for _, s := range t.sources {
		metaCopy := make(map[string]string, len(s.Meta))
		for k, v := range s.Meta {
			metaCopy[k] = v
		}
		copied := *s
		copied.Meta = metaCopy
		out = append(out, copied)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// StaleSources returns the sources silent past threshold as of now.
func (t *HeartbeatTracker) StaleSources(threshold time.Duration, now time.Time) []HeartbeatSource {
	var stale []HeartbeatSource
	for _, s := range t.Sources() {
		if s.Stale(threshold, now) {
			stale = append(stale, s)
		}
	}
	return stale
}
