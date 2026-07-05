package telemetry

import (
	"errors"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeNotifier struct {
	mu   sync.Mutex
	sent []string
	fail bool
}

func (f *fakeNotifier) SendMessage(msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("slack unreachable")
	}
	f.sent = append(f.sent, msg)
	return nil
}

func (f *fakeNotifier) messages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	copy(out, f.sent)
	return out
}

func newTestEngine(db *gorm.DB, notifier Notifier) (*AlertEngine, *HeartbeatTracker, *Registry) {
	tracker := NewHeartbeatTracker()
	reg := NewRegistry()
	e := &AlertEngine{
		db:             db,
		tracker:        tracker,
		reg:            reg,
		notifier:       notifier,
		staleAfter:     90 * time.Second,
		errorWindow:    5 * time.Minute,
		errorThreshold: 10,
		renotify:       30 * time.Minute,
		active:         make(map[string]*activeAlert),
	}
	return e, tracker, reg
}

func TestAlertEngine(t *testing.T) {
	db := newTestDB(t)

	t.Run("stale heartbeat fires, re-notifies until acked, resolves on recovery", func(t *testing.T) {
		notifier := &fakeNotifier{}
		e, tracker, _ := newTestEngine(db, notifier)

		start := time.Now().UTC()
		tracker.Beat(SourceKindStrategy, "cc-lifecycle", nil, start)

		// Fresh: no alert.
		e.Evaluate(start.Add(30 * time.Second))
		assert.Empty(t, notifier.messages())

		// Past threshold: fires once, row persisted, message carries the id.
		e.Evaluate(start.Add(2 * time.Minute))
		msgs := notifier.messages()
		require.Len(t, msgs, 1)
		assert.Contains(t, msgs[0], "FIRING")
		assert.Contains(t, msgs[0], "stale_heartbeat")
		assert.Contains(t, msgs[0], "strategy/cc-lifecycle")

		var row AlertRow
		require.NoError(t, db.Where("rule = ? AND subject = ?", RuleStaleHeartbeat, "strategy/cc-lifecycle").
			Order("id desc").First(&row).Error)
		assert.Nil(t, row.ResolvedAt)
		require.NotNil(t, row.LastNotifiedAt)

		// Within the renotify interval: silent.
		e.Evaluate(start.Add(10 * time.Minute))
		assert.Len(t, notifier.messages(), 1)

		// Past the renotify interval, unacked: nags again.
		e.Evaluate(start.Add(33 * time.Minute))
		require.Len(t, notifier.messages(), 2)

		// Ack silences while still firing.
		require.NoError(t, e.Ack(row.ID, AckViaCLI, start.Add(34*time.Minute)))
		e.Evaluate(start.Add(70 * time.Minute))
		assert.Len(t, notifier.messages(), 2, "acked alert stops re-notifying")

		var acked AlertRow
		require.NoError(t, db.First(&acked, row.ID).Error)
		require.NotNil(t, acked.AckedAt)
		assert.Equal(t, AckViaCLI, acked.AckedVia)

		// Recovery resolves and sends the resolution notification.
		tracker.Beat(SourceKindStrategy, "cc-lifecycle", nil, start.Add(71*time.Minute))
		e.Evaluate(start.Add(71*time.Minute+time.Second))
		msgs = notifier.messages()
		require.Len(t, msgs, 3)
		assert.Contains(t, msgs[2], "RESOLVED")

		var resolved AlertRow
		require.NoError(t, db.First(&resolved, row.ID).Error)
		assert.NotNil(t, resolved.ResolvedAt)
	})

	t.Run("resolution closes an unacked alert with notification", func(t *testing.T) {
		notifier := &fakeNotifier{}
		e, tracker, _ := newTestEngine(db, notifier)

		start := time.Now().UTC()
		tracker.Beat(SourceKindDatasource, "ds-unacked", nil, start)

		e.Evaluate(start.Add(2 * time.Minute)) // fires
		tracker.Beat(SourceKindDatasource, "ds-unacked", nil, start.Add(3*time.Minute))
		e.Evaluate(start.Add(3*time.Minute+time.Second)) // resolves

		msgs := notifier.messages()
		require.Len(t, msgs, 2)
		assert.Contains(t, msgs[1], "RESOLVED")
	})

	t.Run("error burst fires the error-rate rule", func(t *testing.T) {
		notifier := &fakeNotifier{}
		e, _, reg := newTestEngine(db, notifier)

		start := time.Now().UTC()
		e.Evaluate(start) // baseline sample: zero errors

		errCounter := reg.Counter("grodt.errors.total")
		errCounter.Add(11)
		e.Evaluate(start.Add(30 * time.Second))

		msgs := notifier.messages()
		require.Len(t, msgs, 1)
		assert.Contains(t, msgs[0], "error_rate")
		assert.Contains(t, msgs[0], "11 errors")

		// Window slides past the burst: resolves.
		e.Evaluate(start.Add(6 * time.Minute))
		msgs = notifier.messages()
		require.Len(t, msgs, 2)
		assert.Contains(t, msgs[1], "RESOLVED")
	})

	t.Run("errors below threshold stay silent", func(t *testing.T) {
		notifier := &fakeNotifier{}
		e, _, reg := newTestEngine(db, notifier)

		start := time.Now().UTC()
		e.Evaluate(start)
		reg.Counter("grodt.errors.total").Add(5)
		e.Evaluate(start.Add(30 * time.Second))

		assert.Empty(t, notifier.messages())
	})

	t.Run("slack outage: row persists immediately, delivery retries next cycle", func(t *testing.T) {
		notifier := &fakeNotifier{fail: true}
		e, tracker, _ := newTestEngine(db, notifier)

		start := time.Now().UTC()
		tracker.Beat(SourceKindStrategy, "cc-outage", nil, start)

		e.Evaluate(start.Add(2 * time.Minute)) // fires; delivery fails

		var row AlertRow
		require.NoError(t, db.Where("subject = ?", "strategy/cc-outage").Order("id desc").First(&row).Error)
		assert.Nil(t, row.LastNotifiedAt, "row written even though Slack failed")
		assert.Empty(t, notifier.messages())

		// Slack comes back: the very next cycle delivers.
		notifier.mu.Lock()
		notifier.fail = false
		notifier.mu.Unlock()

		e.Evaluate(start.Add(2*time.Minute + 30*time.Second))
		require.Len(t, notifier.messages(), 1)

		require.NoError(t, db.First(&row, row.ID).Error)
		assert.NotNil(t, row.LastNotifiedAt)
	})

	t.Run("ack of unknown or resolved alert fails without side effects", func(t *testing.T) {
		notifier := &fakeNotifier{}
		e, _, _ := newTestEngine(db, notifier)

		err := e.Ack(999999, AckViaSlack, time.Now().UTC())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not firing")
	})

	t.Run("no self-watching rule: no sources and no errors never fires", func(t *testing.T) {
		notifier := &fakeNotifier{}
		e, _, _ := newTestEngine(db, notifier)

		start := time.Now().UTC()
		for i := 0; i < 10; i++ {
			e.Evaluate(start.Add(time.Duration(i) * time.Minute))
		}
		assert.Empty(t, notifier.messages(), "the server does not alert on its own liveness")
	})

	t.Run("auto-halt fires on automatic engage, names the guard, resolves on release", func(t *testing.T) {
		notifier := &fakeNotifier{}
		e, _, _ := newTestEngine(db, notifier)

		// Fake halt state stands in for safety.HaltController.Status().
		var mu sync.Mutex
		engaged, source, reason := false, "", ""
		e.SetHaltStatus(func() (bool, string, string) {
			mu.Lock()
			defer mu.Unlock()
			return engaged, source, reason
		})

		start := time.Now().UTC()

		// Clear: silent.
		e.Evaluate(start)
		assert.Empty(t, notifier.messages())

		// A guard trips: automatic halt engages, alert fires naming the guard.
		mu.Lock()
		engaged, source, reason = true, "auto", "rejection-rate guard: rejection rate 75% over 4 orders exceeds threshold 50%"
		mu.Unlock()

		e.Evaluate(start.Add(30 * time.Second))
		msgs := notifier.messages()
		require.Len(t, msgs, 1)
		assert.Contains(t, msgs[0], "FIRING")
		assert.Contains(t, msgs[0], RuleAutoHalt)
		assert.Contains(t, msgs[0], "rejection-rate guard")

		var row AlertRow
		require.NoError(t, db.Where("rule = ?", RuleAutoHalt).Order("id desc").First(&row).Error)
		assert.Equal(t, "kill-switch", row.Subject)
		assert.Nil(t, row.ResolvedAt)

		// A manual halt does not fire the auto rule; the auto alert resolves.
		mu.Lock()
		engaged, source, reason = true, "manual", "operator halt"
		mu.Unlock()

		e.Evaluate(start.Add(time.Minute))
		msgs = notifier.messages()
		require.Len(t, msgs, 2)
		assert.Contains(t, msgs[1], "RESOLVED")

		// Released entirely: still silent.
		mu.Lock()
		engaged, source, reason = false, "", ""
		mu.Unlock()

		e.Evaluate(start.Add(2 * time.Minute))
		assert.Len(t, notifier.messages(), 2)
	})
}

func TestErrorCounterHook(t *testing.T) {
	Init()
	hook := NewErrorCounterHook()

	logger := log.New()
	logger.SetOutput(nullWriter{})
	logger.AddHook(hook)

	before := errorsTotalValue()
	logger.Error("boom")
	logger.Warn("not counted")
	logger.Info("not counted")
	logger.Error("boom again")

	assert.Equal(t, before+2, errorsTotalValue(), "only Error-and-above increments")
}

func errorsTotalValue() float64 {
	var total float64
	for _, p := range Default.Snapshot() {
		if p.Name == "grodt.errors.total" {
			total += p.Value
		}
	}
	return total
}

type nullWriter struct{}

func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }
