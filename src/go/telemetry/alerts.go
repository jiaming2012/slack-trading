package telemetry

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Alert rule names.
const (
	RuleStaleHeartbeat = "stale_heartbeat"
	RuleErrorRate      = "error_rate"
	RuleAutoHalt       = "auto_halt"
)

// Ack channels.
const (
	AckViaSlack = "slack"
	AckViaCLI   = "cli"
)

// Notifier delivers an alert message to the operator. Satisfied by the
// existing workers.SlackNotifierClient.
type Notifier interface {
	SendMessage(msg string) error
}

// activeAlert is the in-memory mirror of a firing telemetry_alerts row.
type activeAlert struct {
	rowID        uint // 0 until the row insert succeeds; retried each cycle
	rule         string
	subject      string
	message      string
	firedAt      time.Time
	lastNotified *time.Time
	acked        bool
}

type errSample struct {
	at    time.Time
	total float64
}

// AlertEngine evaluates alert rules over in-memory telemetry state (never the
// database, so evaluation survives DB slowness) and drives the
// firing→(acked)→resolved lifecycle with re-notification until acknowledged.
type AlertEngine struct {
	db       *gorm.DB
	tracker  *HeartbeatTracker
	reg      *Registry
	notifier Notifier

	staleAfter     time.Duration
	errorWindow    time.Duration
	errorThreshold int
	renotify       time.Duration

	mu         sync.Mutex
	active     map[string]*activeAlert
	errSamples []errSample

	// haltStatus, when set, feeds the auto_halt rule: it reports whether the
	// kill switch is engaged, its source ("auto"/"manual"), and the recorded
	// reason (which names the tripping guard). Nil = rule inactive.
	haltStatus HaltStatusFunc
}

// HaltStatusFunc reports the kill-switch halt state for the auto_halt alert
// rule. Wired at startup from the shared safety.HaltController's Status.
type HaltStatusFunc func() (engaged bool, source string, reason string)

// SetHaltStatus installs the halt-state provider consulted by the auto_halt
// rule. Safe to call while the engine is running.
func (e *AlertEngine) SetHaltStatus(fn HaltStatusFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.haltStatus = fn
}

// NewAlertEngine wires the engine with the operator-tunable thresholds from
// the environment (design D6).
func NewAlertEngine(db *gorm.DB, tracker *HeartbeatTracker, reg *Registry, notifier Notifier) *AlertEngine {
	return &AlertEngine{
		db:             db,
		tracker:        tracker,
		reg:            reg,
		notifier:       notifier,
		staleAfter:     HeartbeatStaleAfter(),
		errorWindow:    ErrorWindow(),
		errorThreshold: ErrorThreshold(),
		renotify:       RenotifyInterval(),
		active:         make(map[string]*activeAlert),
	}
}

// Start runs the evaluation loop until ctx is done.
func (e *AlertEngine) Start(ctx context.Context) {
	ticker := time.NewTicker(defaultEvaluationInterval)
	defer ticker.Stop()

	log.Infof("telemetry: alert engine started (evaluate every %s, stale after %s, renotify every %s)",
		defaultEvaluationInterval, e.staleAfter, e.renotify)
	for {
		select {
		case <-ctx.Done():
			log.Info("telemetry: alert engine stopping")
			return
		case <-ticker.C:
			e.Evaluate(time.Now().UTC())
		}
	}
}

// errorsTotal reads the cumulative error counter from the registry snapshot.
func (e *AlertEngine) errorsTotal() float64 {
	var total float64
	for _, p := range e.reg.Snapshot() {
		if p.Name == "grodt.errors.total" {
			total += p.Value
		}
	}
	return total
}

// errorCountInWindow updates the sample history with the current cumulative
// total and returns how many errors were logged within the rolling window.
// Callers must hold e.mu.
func (e *AlertEngine) errorCountInWindow(now time.Time, total float64) int {
	e.errSamples = append(e.errSamples, errSample{at: now, total: total})

	// Baseline = the newest sample at or before the window boundary; keep one
	// sample older than the window so the baseline stays available.
	boundary := now.Add(-e.errorWindow)
	baselineIdx := 0
	for i, s := range e.errSamples {
		if s.at.After(boundary) {
			break
		}
		baselineIdx = i
	}
	e.errSamples = e.errSamples[baselineIdx:]

	return int(total - e.errSamples[0].total)
}

// Evaluate runs one rule-evaluation cycle at the given time. Exported for
// tests; production calls it from Start's ticker.
func (e *AlertEngine) Evaluate(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Desired firing set for this cycle.
	type desired struct{ rule, subject, message string }
	want := make(map[string]desired)

	for _, s := range e.tracker.StaleSources(e.staleAfter, now) {
		subject := s.Kind + "/" + s.Name
		silence := now.Sub(s.LastSeen).Round(time.Second)
		want[RuleStaleHeartbeat+"|"+subject] = desired{
			rule:    RuleStaleHeartbeat,
			subject: subject,
			message: fmt.Sprintf("%s heartbeat stale: last seen %s ago (threshold %s)", subject, silence, e.staleAfter),
		}
	}

	if count := e.errorCountInWindow(now, e.errorsTotal()); count > e.errorThreshold {
		want[RuleErrorRate+"|server"] = desired{
			rule:    RuleErrorRate,
			subject: "server",
			message: fmt.Sprintf("error rate spike: %d errors logged in the last %s (threshold %d)", count, e.errorWindow, e.errorThreshold),
		}
	}

	// Auto-halt rule: an engaged automatic halt (a guard tripped the kill
	// switch) fires until acknowledged and resolves when the halt clears, so
	// the operator hears about an auto-halt without polling status.
	if e.haltStatus != nil {
		if engaged, source, reason := e.haltStatus(); engaged && source == "auto" {
			want[RuleAutoHalt+"|kill-switch"] = desired{
				rule:    RuleAutoHalt,
				subject: "kill-switch",
				message: fmt.Sprintf("kill switch AUTO-HALT engaged: %s — order submission is blocked until acknowledge + release (task kill-switch:acknowledge / kill-switch:release)", reason),
			}
		}
	}

	// Resolve alerts whose condition cleared — notified whether or not acked.
	for key, a := range e.active {
		if _, still := want[key]; still {
			continue
		}
		delete(e.active, key)

		if a.rowID != 0 {
			if err := e.db.Model(&AlertRow{}).Where("id = ?", a.rowID).
				Update("resolved_at", now).Error; err != nil {
				// Warn, not Error: the engine must never feed the error
				// counter it alerts on.
				log.Warnf("telemetry: failed to persist resolution of alert #%d: %v", a.rowID, err)
			}
		}
		e.deliver(fmt.Sprintf("✅ grodt alert #%d RESOLVED [%s] %s", a.rowID, a.rule, a.subject), nil, now)
	}

	// Fire new alerts and (re-)notify existing ones.
	for key, d := range want {
		a, ok := e.active[key]
		if !ok {
			a = &activeAlert{rule: d.rule, subject: d.subject, message: d.message, firedAt: now}
			e.active[key] = a
		}
		a.message = d.message

		// Persist the row first — the record exists independently of Slack.
		if a.rowID == 0 {
			row := AlertRow{Rule: a.rule, Subject: a.subject, Message: a.message, FiredAt: a.firedAt}
			if err := e.db.Create(&row).Error; err != nil {
				log.Warnf("telemetry: failed to persist alert row for %s (will retry next cycle): %v", key, err)
			} else {
				a.rowID = row.ID
			}
		}

		// Notify on firing, then every renotify interval until acked or
		// resolved. A failed post leaves lastNotified unset/stale, so the
		// next cycle retries — delivery retry falls out of the same loop.
		if a.acked {
			continue
		}
		if a.lastNotified != nil && now.Sub(*a.lastNotified) < e.renotify {
			continue
		}
		msg := fmt.Sprintf("🚨 grodt alert #%d FIRING [%s] %s — ack: task alert:ack ID=%d (Slack: ack %d)",
			a.rowID, a.rule, a.message, a.rowID, a.rowID)
		e.deliver(msg, a, now)
	}
}

// deliver posts to Slack; on success it stamps the alert's lastNotified state
// in memory and in the row. Resolution messages pass a nil alert (best-effort,
// no retry loop — the row already records the resolution).
func (e *AlertEngine) deliver(msg string, a *activeAlert, now time.Time) {
	if err := e.notifier.SendMessage(msg); err != nil {
		// Warn, not Error: a Slack outage must not increment the error
		// counter and re-trigger the error-rate rule (feedback loop).
		log.Warnf("telemetry: slack delivery failed (will retry next cycle): %v", err)
		return
	}
	if a == nil {
		return
	}
	t := now
	a.lastNotified = &t
	if a.rowID != 0 {
		if err := e.db.Model(&AlertRow{}).Where("id = ?", a.rowID).
			Update("last_notified_at", now).Error; err != nil {
			log.Warnf("telemetry: failed to persist last_notified_at for alert #%d: %v", a.rowID, err)
		}
	}
}

// Ack acknowledges a firing alert by row id, silencing re-notification while
// its condition persists. Unknown or already-resolved ids are an error with
// no side effects.
func (e *AlertEngine) Ack(id uint, via string, now time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, a := range e.active {
		if a.rowID != id || a.rowID == 0 {
			continue
		}
		if err := e.db.Model(&AlertRow{}).Where("id = ?", id).Updates(map[string]interface{}{
			"acked_at": now, "acked_via": via,
		}).Error; err != nil {
			return fmt.Errorf("Ack: failed to persist acknowledgement of alert #%d: %w", id, err)
		}
		a.acked = true
		log.Infof("telemetry: alert #%d acknowledged via %s", id, via)
		return nil
	}
	return fmt.Errorf("Ack: alert #%d is not firing (unknown or already resolved)", id)
}
