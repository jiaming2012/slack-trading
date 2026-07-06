package fidelity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedSource records every requested period and returns the scripted
// responses in order (the last response repeats once the script is exhausted).
type scriptedSource struct {
	periods   []Period
	responses []struct {
		set TradeSet
		err error
	}
	calls int
}

func (s *scriptedSource) FetchTradeSet(_ context.Context, period Period) (TradeSet, error) {
	s.periods = append(s.periods, period)
	idx := s.calls
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	s.calls++
	r := s.responses[idx]
	return r.set, r.err
}

func scripted(responses ...struct {
	set TradeSet
	err error
}) *scriptedSource {
	return &scriptedSource{responses: responses}
}

func resp(set TradeSet, err error) struct {
	set TradeSet
	err error
} {
	return struct {
		set TradeSet
		err error
	}{set: set, err: err}
}

func TestMonitor_RunOnceEvaluatesTrailingWindow(t *testing.T) {
	src := scripted(resp(ZeroDriftSet(), nil))
	m, err := NewMonitor(MonitorOptions{Source: src, Period: 168 * time.Hour})
	require.NoError(t, err)

	at := time.Date(2026, time.July, 5, 12, 0, 0, 0, time.UTC)
	report := m.RunOnce(context.Background(), at)

	require.Len(t, src.periods, 1)
	assert.True(t, src.periods[0].End.Equal(at), "window ends at the evaluation time")
	assert.True(t, src.periods[0].Start.Equal(at.Add(-168*time.Hour)), "window starts period earlier")
	assert.Equal(t, src.periods[0], report.Period)
}

func TestMonitor_OutcomeClassification(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, time.July, 5, 12, 0, 0, 0, time.UTC)

	t.Run("all within tolerance is ok", func(t *testing.T) {
		m, err := NewMonitor(MonitorOptions{Source: StaticTradeSource{Set: ZeroDriftSet()}})
		require.NoError(t, err)
		report := m.RunOnce(ctx, at)
		assert.Equal(t, RunOK, report.Outcome)
		require.Len(t, report.Results, 1)
		assert.NoError(t, report.Err)
	})

	t.Run("any breaching strategy is breach", func(t *testing.T) {
		m, err := NewMonitor(MonitorOptions{Source: SyntheticSource()})
		require.NoError(t, err)
		report := m.RunOnce(ctx, at)
		assert.Equal(t, RunBreach, report.Outcome)
		require.Len(t, report.Results, 5)
	})

	t.Run("no live trades is a clean no_data with no results", func(t *testing.T) {
		m, err := NewMonitor(MonitorOptions{Source: NoLiveTradesSource{}})
		require.NoError(t, err)
		report := m.RunOnce(ctx, at)
		assert.Equal(t, RunNoData, report.Outcome)
		assert.Nil(t, report.Results, "no_data runs must not feed downstream alert state")
		assert.NoError(t, report.Err)
	})

	t.Run("source failure is error", func(t *testing.T) {
		boom := errors.New("feed down")
		m, err := NewMonitor(MonitorOptions{Source: StaticTradeSource{Err: boom}})
		require.NoError(t, err)
		report := m.RunOnce(ctx, at)
		assert.Equal(t, RunError, report.Outcome)
		assert.ErrorIs(t, report.Err, boom)
		assert.Nil(t, report.Results)
	})
}

func TestMonitor_StartRunsImmediatelyThenPerTick(t *testing.T) {
	src := scripted(resp(ZeroDriftSet(), nil))
	reports := make(chan RunReport, 8)

	base := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)
	clock := base
	m, err := NewMonitor(MonitorOptions{
		Source:        src,
		Period:        168 * time.Hour,
		Now:           func() time.Time { return clock },
		OnRunComplete: func(r RunReport) { reports <- r },
	})
	require.NoError(t, err)

	ticks := make(chan time.Time)
	m.runTicks = ticks
	m.keepaliveTicks = make(chan time.Time) // never fires

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { m.Start(ctx); close(done) }()

	// The immediate boot run.
	first := <-reports
	assert.True(t, first.At.Equal(base))
	assert.True(t, first.Period.End.Equal(base))

	// A tick 24h later evaluates the trailing window ending at the new now.
	clock = base.Add(24 * time.Hour)
	ticks <- clock
	second := <-reports
	assert.True(t, second.At.Equal(clock))
	assert.True(t, second.Period.Start.Equal(clock.Add(-168*time.Hour)))
	assert.True(t, second.Period.End.Equal(clock))

	cancel()
	<-done
}

func TestMonitor_FailingRunDoesNotStopTheLoop(t *testing.T) {
	src := scripted(
		resp(TradeSet{}, errors.New("transient source failure")),
		resp(ZeroDriftSet(), nil),
	)
	reports := make(chan RunReport, 8)

	m, err := NewMonitor(MonitorOptions{
		Source:        src,
		OnRunComplete: func(r RunReport) { reports <- r },
	})
	require.NoError(t, err)

	ticks := make(chan time.Time)
	m.runTicks = ticks
	m.keepaliveTicks = make(chan time.Time)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { m.Start(ctx); close(done) }()

	first := <-reports // immediate boot run: the scripted failure
	assert.Equal(t, RunError, first.Outcome)

	ticks <- time.Now().UTC() // the next tick still evaluates normally
	second := <-reports
	assert.Equal(t, RunOK, second.Outcome)

	cancel()
	<-done
}

func TestMonitor_KeepaliveTicksReachTheHook(t *testing.T) {
	beats := make(chan time.Time, 8)
	m, err := NewMonitor(MonitorOptions{
		Source:      NoLiveTradesSource{},
		OnKeepalive: func(at time.Time) { beats <- at },
	})
	require.NoError(t, err)

	m.runTicks = make(chan time.Time)
	keepalives := make(chan time.Time)
	m.keepaliveTicks = keepalives

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { m.Start(ctx); close(done) }()

	keepalives <- time.Now().UTC()
	select {
	case <-beats:
	case <-time.After(5 * time.Second):
		t.Fatal("keepalive tick never reached the hook")
	}

	cancel()
	<-done
}

func TestNewMonitor_RequiresSourceAndAppliesDefaults(t *testing.T) {
	_, err := NewMonitor(MonitorOptions{})
	require.Error(t, err)

	m, err := NewMonitor(MonitorOptions{Source: NoLiveTradesSource{}})
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, m.Interval())
	assert.Equal(t, 168*time.Hour, m.Period())
}

func TestMonitorConfig_EnvKnobs(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		assert.True(t, MonitorEnabled())
		assert.Equal(t, 24*time.Hour, MonitorInterval())
		assert.Equal(t, 168*time.Hour, MonitorPeriod())
	})

	t.Run("disabled flag", func(t *testing.T) {
		t.Setenv("FIDELITY_MONITOR_ENABLED", "false")
		assert.False(t, MonitorEnabled())
	})

	t.Run("invalid values fall back to defaults", func(t *testing.T) {
		t.Setenv("FIDELITY_MONITOR_ENABLED", "banana")
		t.Setenv("FIDELITY_CHECK_INTERVAL", "-3h")
		t.Setenv("FIDELITY_PERIOD", "soon")
		assert.True(t, MonitorEnabled())
		assert.Equal(t, 24*time.Hour, MonitorInterval())
		assert.Equal(t, 168*time.Hour, MonitorPeriod())
	})

	t.Run("overrides parse", func(t *testing.T) {
		t.Setenv("FIDELITY_CHECK_INTERVAL", "6h")
		t.Setenv("FIDELITY_PERIOD", "72h")
		assert.Equal(t, 6*time.Hour, MonitorInterval())
		assert.Equal(t, 72*time.Hour, MonitorPeriod())
	})
}
