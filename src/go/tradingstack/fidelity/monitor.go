package fidelity

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// RunOutcome classifies one scheduled fidelity run. Every run completes as
// exactly one of these.
type RunOutcome string

const (
	// RunOK: results were produced and every strategy is within tolerance.
	RunOK RunOutcome = "ok"
	// RunBreach: results were produced and at least one strategy breached.
	RunBreach RunOutcome = "breach"
	// RunNoData: the source yielded no live trades — a clean non-outcome. No
	// fidelity rows are persisted and no alert state changes.
	RunNoData RunOutcome = "no_data"
	// RunError: the source, engine, or persistence failed. Logged at Warn;
	// the loop continues.
	RunError RunOutcome = "error"
)

// RunReport is the record of one completed run, handed to OnRunComplete.
type RunReport struct {
	// At is the evaluation time (the end of the evaluated window).
	At time.Time
	// Period is the trailing window the run evaluated: [At − period, At].
	Period Period
	// Outcome classifies the run.
	Outcome RunOutcome
	// Results holds the per-strategy fidelity results. Non-nil only for
	// result-producing runs (RunOK / RunBreach) — no_data and error runs must
	// not feed downstream alert state (design D3).
	Results []Result
	// Err carries the failure for RunError outcomes; nil otherwise.
	Err error
}

// MonitorOptions configures a Monitor. Source is required; everything else
// defaults sensibly.
type MonitorOptions struct {
	// Source supplies each run's sim-vs-live trade sets (required).
	Source TradeSource
	// DB, when non-nil, receives one upserted simulator_fidelity row per
	// strategy on every result-producing run (via Persist). Nil disables
	// persistence (pure in-memory operation for tests).
	DB *gorm.DB
	// Scoring holds the deterministic scoring parameters; zero value is
	// replaced by DefaultConfig().
	Scoring FidelityConfig
	// Interval between evaluations. Defaults to MonitorInterval().
	Interval time.Duration
	// Period is the trailing window each evaluation covers, ending at the
	// evaluation time. Defaults to MonitorPeriod().
	Period time.Duration
	// KeepaliveInterval is the fast idle tick proving the goroutine is alive
	// between runs (the run interval is far longer than the heartbeat
	// staleness threshold). Defaults to 60s.
	KeepaliveInterval time.Duration
	// Now supplies the clock; defaults to UTC wall time. Injectable for tests.
	Now func() time.Time
	// OnRunComplete, when set, observes every completed run (any outcome).
	// Production wires telemetry here: the run counter, per-strategy gauges,
	// the AlertEngine fidelity snapshot, and the job heartbeat.
	OnRunComplete func(RunReport)
	// OnKeepalive, when set, observes every idle keepalive tick. Production
	// wires the job heartbeat's idle beat here.
	OnKeepalive func(at time.Time)
}

// Monitor runs the fidelity checker on a schedule inside the trading server
// (design D1): every Interval it evaluates the trailing Period ending at the
// evaluation time, persists result-producing runs, and reports every run
// through OnRunComplete. It is strictly read-only with respect to trading —
// it never places, modifies, or cancels an order, and never mutates any
// playground state; its only write is the simulator_fidelity upsert.
type Monitor struct {
	opts MonitorOptions

	// Injectable tick channels for tests. When nil, Start creates real
	// tickers from the configured intervals.
	runTicks       <-chan time.Time
	keepaliveTicks <-chan time.Time
}

// NewMonitor validates the options and applies defaults.
func NewMonitor(opts MonitorOptions) (*Monitor, error) {
	if opts.Source == nil {
		return nil, fmt.Errorf("fidelity: NewMonitor: a TradeSource is required")
	}
	if opts.Scoring == (FidelityConfig{}) {
		opts.Scoring = DefaultConfig()
	}
	if opts.Interval <= 0 {
		opts.Interval = MonitorInterval()
	}
	if opts.Period <= 0 {
		opts.Period = MonitorPeriod()
	}
	if opts.KeepaliveInterval <= 0 {
		opts.KeepaliveInterval = defaultMonitorKeepalive
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Monitor{opts: opts}, nil
}

// Interval returns the effective evaluation interval.
func (m *Monitor) Interval() time.Duration { return m.opts.Interval }

// Period returns the effective trailing evaluation window.
func (m *Monitor) Period() time.Duration { return m.opts.Period }

// RunOnce evaluates the trailing window ending at the given time, classifies
// the outcome, persists result-producing runs when a DB is configured, and
// hands the report to OnRunComplete. A failed run is logged at Warn severity
// and never panics or propagates — the caller's loop continues.
func (m *Monitor) RunOnce(ctx context.Context, at time.Time) RunReport {
	report := m.evaluate(ctx, at)
	if m.opts.OnRunComplete != nil {
		m.opts.OnRunComplete(report)
	}
	return report
}

func (m *Monitor) evaluate(ctx context.Context, at time.Time) RunReport {
	period := Period{Start: at.Add(-m.opts.Period), End: at}
	report := RunReport{At: at, Period: period}

	set, err := m.opts.Source.FetchTradeSet(ctx, period)
	if err != nil {
		report.Outcome = RunError
		report.Err = err
		log.Warnf("fidelity monitor: trade source failed for period %s..%s: %v",
			period.Start.Format(time.RFC3339), period.End.Format(time.RFC3339), err)
		return report
	}

	results, _, err := RunFidelityCheck(ctx, set, period, m.opts.Scoring, nil)
	if errors.Is(err, ErrNoLiveTrades) {
		report.Outcome = RunNoData
		log.Infof("fidelity monitor: no live trades for period %s..%s; nothing to compare",
			period.Start.Format(time.RFC3339), period.End.Format(time.RFC3339))
		return report
	}
	if err != nil {
		report.Outcome = RunError
		report.Err = err
		log.Warnf("fidelity monitor: fidelity check failed for period %s..%s: %v",
			period.Start.Format(time.RFC3339), period.End.Format(time.RFC3339), err)
		return report
	}

	if m.opts.DB != nil {
		if err := Persist(m.opts.DB, results, at); err != nil {
			// A persistence failure is a run failure: the results did not
			// reach history, and downstream alert state must not move on a
			// run that left no durable trace.
			report.Outcome = RunError
			report.Err = err
			log.Warnf("fidelity monitor: persist failed for period %s..%s: %v",
				period.Start.Format(time.RFC3339), period.End.Format(time.RFC3339), err)
			return report
		}
	}

	report.Outcome = RunOK
	for _, r := range results {
		if !r.WithinTolerance {
			report.Outcome = RunBreach
			break
		}
	}
	report.Results = results
	return report
}

// Start runs the monitor loop until ctx is done: one evaluation immediately
// (so a fresh boot reports its state without waiting a full interval — same
// shape as telemetry.StartPrune), then one per Interval, with keepalive ticks
// in between. Run it in its own goroutine.
func (m *Monitor) Start(ctx context.Context) {
	runTicks := m.runTicks
	if runTicks == nil {
		t := time.NewTicker(m.opts.Interval)
		defer t.Stop()
		runTicks = t.C
	}
	keepaliveTicks := m.keepaliveTicks
	if keepaliveTicks == nil {
		t := time.NewTicker(m.opts.KeepaliveInterval)
		defer t.Stop()
		keepaliveTicks = t.C
	}

	m.RunOnce(ctx, m.opts.Now())

	for {
		select {
		case <-ctx.Done():
			log.Info("fidelity monitor: stopping")
			return
		case <-runTicks:
			m.RunOnce(ctx, m.opts.Now())
		case <-keepaliveTicks:
			if m.opts.OnKeepalive != nil {
				m.opts.OnKeepalive(m.opts.Now())
			}
		}
	}
}
