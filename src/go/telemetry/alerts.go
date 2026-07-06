package telemetry

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Alert rule names.
const (
	RuleStaleHeartbeat      = "stale_heartbeat"
	RuleErrorRate           = "error_rate"
	RuleAutoHalt            = "auto_halt"
	RuleDeferredAutoClose   = "deferred_auto_close"
	RuleUnprotectedPosition = "unprotected_position"

	// wire-risk-overlay-state: sustained fail-permissive degradation of the
	// portfolio risk overlay (the gate is permitting orders it could not
	// evaluate), and an enabled gate whose EV allocation family is pinned
	// inactive (an entire limit family silently missing).
	RuleRiskOverlayDegraded = "riskoverlay_degraded"
	RuleRiskOverlayEvPinned = "riskoverlay_ev_pinned"

	// continuous-fidelity-monitoring: a strategy whose last-known scheduled
	// fidelity result breached tolerance — the simulator has drifted from
	// reality and optimizers must not consume that strategy's simulator data.
	RuleFidelityDrift = "fidelity_drift"
)

// FidelityStrategyStatus is one strategy's last-known fidelity verdict, as
// reported by the fidelity monitor after a result-producing run. It is a
// telemetry-owned struct (the monitor maps fidelity.Result into it) so this
// package never imports the fidelity package.
type FidelityStrategyStatus struct {
	StrategyID      string
	DriftScore      float64
	WithinTolerance bool
}

// UnprotectedPosition describes a live position whose broker-held companion
// stop failed to place (wire-companion-stops): the position has NO protective
// exit at the broker until the operator intervenes. The safety package records
// these; the alert engine fires one alert per position until acknowledged.
type UnprotectedPosition struct {
	EntryOrderID uint
	Symbol       string
	Quantity     int
	Reason       string
}

// UnprotectedPositionsFunc reports the currently known unprotected positions.
// Wired at startup from the safety package's companion-stop hook.
type UnprotectedPositionsFunc func() []UnprotectedPosition

// Ack channels.
const (
	AckViaSlack = "slack"
	AckViaCLI   = "cli"
)

// deferredCommittedSuffix marks a sticky deferred-auto-close alert whose
// deferrals have since committed: it keeps firing until acknowledged.
const deferredCommittedSuffix = " — the deferred close(s) have since committed; acknowledge to clear this alert"

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

	staleAfter        time.Duration
	errorWindow       time.Duration
	errorThreshold    int
	renotify          time.Duration
	degradedWindow    time.Duration
	degradedThreshold int

	mu              sync.Mutex
	active          map[string]*activeAlert
	errSamples      []errSample
	degradedSamples []errSample

	// haltStatus, when set, feeds the auto_halt rule: it reports whether the
	// kill switch is engaged, its source ("auto"/"manual"), and the recorded
	// reason (which names the tripping guard). Nil = rule inactive.
	haltStatus HaltStatusFunc

	// unprotected, when set, feeds the unprotected_position rule with the
	// positions whose companion stops failed to place. Nil = rule inactive.
	unprotected UnprotectedPositionsFunc

	// deferredCount, when set, is the AUTHORITATIVE count of outstanding
	// deferred auto-closes (backed by the persisted deferral set, rehydrated
	// into playground memory at load). When nil the rule falls back to the
	// internal-registry gauge.
	deferredCount DeferredAutoCloseCountFunc

	// fidelity is the last-known per-strategy fidelity snapshot, replaced
	// wholesale by ReportFidelity on every result-producing monitor run and
	// deliberately left standing across no_data/error runs (design D3: a
	// strategy known to be drifting stays alerted until contradicted by data).
	fidelity []FidelityStrategyStatus
}

// DeferredAutoCloseCountFunc reports how many option auto-closes are currently
// deferred by an engaged halt, summed across playgrounds. Wired at startup
// from the database service's in-memory (DB-rehydrated) deferral sets.
type DeferredAutoCloseCountFunc func() int

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

// SetUnprotectedPositions installs the provider consulted by the
// unprotected_position rule. Safe to call while the engine is running.
func (e *AlertEngine) SetUnprotectedPositions(fn UnprotectedPositionsFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.unprotected = fn
}

// SetDeferredAutoCloses installs the authoritative deferred-auto-close count
// provider. Safe to call while the engine is running.
func (e *AlertEngine) SetDeferredAutoCloses(fn DeferredAutoCloseCountFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.deferredCount = fn
}

// ReportFidelity replaces the engine's per-strategy fidelity snapshot with
// the given result-producing run's statuses. The fidelity monitor calls it
// after every run that produced results — and does NOT call it on no_data or
// error runs, so the last-known state stands until contradicted by data. Safe
// to call while the engine is running.
func (e *AlertEngine) ReportFidelity(statuses []FidelityStrategyStatus) {
	snapshot := make([]FidelityStrategyStatus, len(statuses))
	copy(snapshot, statuses)

	e.mu.Lock()
	defer e.mu.Unlock()
	e.fidelity = snapshot
}

// NewAlertEngine wires the engine with the operator-tunable thresholds from
// the environment (design D6).
func NewAlertEngine(db *gorm.DB, tracker *HeartbeatTracker, reg *Registry, notifier Notifier) *AlertEngine {
	return &AlertEngine{
		db:                db,
		tracker:           tracker,
		reg:               reg,
		notifier:          notifier,
		staleAfter:        HeartbeatStaleAfter(),
		errorWindow:       ErrorWindow(),
		errorThreshold:    ErrorThreshold(),
		renotify:          RenotifyInterval(),
		degradedWindow:    RiskOverlayDegradedWindow(),
		degradedThreshold: RiskOverlayDegradedThreshold(),
		active:            make(map[string]*activeAlert),
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

// metricTotal reads the sum of a named series (across all label sets) from
// the registry snapshot.
func (e *AlertEngine) metricTotal(name string) float64 {
	var total float64
	for _, p := range e.reg.Snapshot() {
		if p.Name == name {
			total += p.Value
		}
	}
	return total
}

// errorsTotal reads the cumulative error counter from the registry snapshot.
func (e *AlertEngine) errorsTotal() float64 {
	return e.metricTotal("grodt.errors.total")
}

// errorCountInWindow updates the sample history with the current cumulative
// total and returns how many errors were logged within the rolling window.
// Callers must hold e.mu.
func (e *AlertEngine) errorCountInWindow(now time.Time, total float64) int {
	return countInWindow(&e.errSamples, e.errorWindow, now, total)
}

// degradedCountInWindow is the riskoverlay analogue of errorCountInWindow,
// tracking the cumulative grodt.riskoverlay.degraded total over its own
// rolling window. Callers must hold e.mu.
func (e *AlertEngine) degradedCountInWindow(now time.Time, total float64) int {
	return countInWindow(&e.degradedSamples, e.degradedWindow, now, total)
}

// countInWindow appends the current cumulative total to the sample history,
// prunes samples older than the window (keeping one older sample as the
// baseline), and returns the count accumulated within the rolling window.
func countInWindow(samples *[]errSample, window time.Duration, now time.Time, total float64) int {
	*samples = append(*samples, errSample{at: now, total: total})

	// Baseline = the newest sample at or before the window boundary; keep one
	// sample older than the window so the baseline stays available.
	boundary := now.Add(-window)
	baselineIdx := 0
	for i, s := range *samples {
		if s.at.After(boundary) {
			break
		}
		baselineIdx = i
	}
	*samples = (*samples)[baselineIdx:]

	return int(total - (*samples)[0].total)
}

// metricValue reads the sum of a named series from the registry snapshot and
// reports whether the series exists at all — a gauge that has never been set
// must not be conflated with a gauge reading zero.
func (e *AlertEngine) metricValue(name string) (float64, bool) {
	var total float64
	found := false
	for _, p := range e.reg.Snapshot() {
		if p.Name == name {
			total += p.Value
			found = true
		}
	}
	return total, found
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

	// Riskoverlay degradation rule (wire-risk-overlay-state): sustained
	// fail-permissive permits mean the risk overlay is letting orders through
	// that it could NOT evaluate — the gate is blind but permitting, and the
	// operator must know without Grafana.
	if count := e.degradedCountInWindow(now, e.metricTotal(MetricRiskOverlayDegraded)); count > e.degradedThreshold {
		want[RuleRiskOverlayDegraded+"|risk-overlay"] = desired{
			rule:    RuleRiskOverlayDegraded,
			subject: "risk-overlay",
			message: fmt.Sprintf("risk overlay DEGRADED: %d fail-permissive/blind evaluations in the last %s (threshold %d) — orders are being permitted without full risk evaluation; check DB health and the grodt.riskoverlay.degraded reasons", count, e.degradedWindow, e.degradedThreshold),
		}
	}

	// Riskoverlay EV-pin rule (wire-risk-overlay-state): an ENABLED gate whose
	// strategy-allocation family is pinned inactive (empty EV-weight set) is
	// silently missing an entire limit family. Both gauges must exist — a gate
	// that never evaluated (or was never installed) stays silent.
	if enabled, okE := e.metricValue(MetricRiskOverlayEnabled); okE && enabled >= 1 {
		if evActive, okV := e.metricValue(MetricRiskOverlayEvFamilyActive); okV && evActive == 0 {
			want[RuleRiskOverlayEvPinned+"|risk-overlay"] = desired{
				rule:    RuleRiskOverlayEvPinned,
				subject: "risk-overlay",
				message: "risk overlay EV allocation family PINNED INACTIVE: the gate is enabled but strategy_ev_weights is empty, so per-strategy allocation caps are not enforced — populate EV weights (task ev pipeline) or acknowledge if expected",
			}
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

	// Unprotected-position rule (wire-companion-stops): a companion-stop
	// placement failure left a live position with NO broker-held protective
	// exit. One alert per position, firing (and re-notifying) until the
	// operator acknowledges; it resolves only if the safety hook reports the
	// position protected or gone.
	if e.unprotected != nil {
		for _, u := range e.unprotected() {
			subject := fmt.Sprintf("%s/entry-%d", u.Symbol, u.EntryOrderID)
			want[RuleUnprotectedPosition+"|"+subject] = desired{
				rule:    RuleUnprotectedPosition,
				subject: subject,
				message: fmt.Sprintf("position UNPROTECTED: companion stop failed for %s qty %d (entry order %d): %s — the position has NO broker-held exit; place a protective stop manually", u.Symbol, u.Quantity, u.EntryOrderID, u.Reason),
			}
		}
	}

	// Fidelity-drift rule (continuous-fidelity-monitoring): one alert per
	// strategy whose last-known scheduled fidelity result breached tolerance.
	// The condition comes from the explicit ReportFidelity snapshot, never
	// from the fidelity gauges (design D3) — the snapshot is replaced
	// wholesale per result-producing run and stands across no_data runs, so
	// the alert resolves only when a later run shows the strategy within
	// tolerance, not on mere absence of data.
	for _, s := range e.fidelity {
		if s.WithinTolerance {
			continue
		}
		subject := "strategy/" + s.StrategyID
		want[RuleFidelityDrift+"|"+subject] = desired{
			rule:    RuleFidelityDrift,
			subject: subject,
			message: fmt.Sprintf("simulator fidelity DRIFT for strategy %s: drift_score %.4f breached tolerance — the simulator has drifted from live behavior; optimizers must not consume this strategy's simulator data until fidelity recovers (task fidelity:status)", s.StrategyID, s.DriftScore),
		}
	}

	// Deferred-auto-close rule (wire-companion-stops): option auto-closes
	// deferred by an engaged halt are open exposure the operator must know
	// about. The condition comes from the authoritative provider (backed by
	// the persisted deferral set) when wired, falling back to the
	// internal-registry gauge otherwise — a reset gauge alone must never
	// signal an all-clear the persisted set contradicts.
	deferred := 0
	if e.deferredCount != nil {
		deferred = e.deferredCount()
	} else {
		deferred = int(e.metricTotal(MetricDeferredAutoCloses))
	}
	if deferred > 0 {
		want[RuleDeferredAutoClose+"|auto-closes"] = desired{
			rule:    RuleDeferredAutoClose,
			subject: "auto-closes",
			message: fmt.Sprintf("%d option auto-close(s) DEFERRED by the engaged kill switch — this is OPEN EXPOSURE that will not close until the halt is released (task kill-switch:release)", deferred),
		}
	}

	// The deferred-auto-close rule is STICKY: exposure existed, so the alert
	// must reach the operator — it never auto-resolves just because the
	// condition cleared. An unacked alert whose deferrals have since
	// committed keeps firing (with an updated message) until acknowledged;
	// acknowledged alerts resolve on the next cycle with the condition clear.
	for key, a := range e.active {
		if a.rule != RuleDeferredAutoClose || a.acked {
			continue
		}
		if _, still := want[key]; still {
			continue
		}
		msg := a.message
		if !strings.Contains(msg, deferredCommittedSuffix) {
			msg += deferredCommittedSuffix
		}
		want[key] = desired{rule: a.rule, subject: a.subject, message: msg}
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
