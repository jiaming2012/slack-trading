package safety

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mutableStaleness is a staleness signal the tests can retarget mid-scenario
// (feed starts ticking, feed dies) — fakeStaleness is a fixed value.
type mutableStaleness struct {
	age time.Duration
	ok  bool
}

func (m *mutableStaleness) LastTickAge() (time.Duration, bool) { return m.age, m.ok }

const testOpenGrace = 5 * time.Minute

// stalenessCycleFixture installs a registry over the given signal and returns
// the shared controller plus a ticker state carrying the open-boundary grace.
func stalenessCycleFixture(t *testing.T, signal StalenessSignal) (*HaltController, *stalenessTickerState) {
	t.Helper()
	c := newTestController(t)
	reg := &GuardRegistry{
		Controller:    c,
		FeedStaleness: NewFeedStalenessGuard(30*time.Second, signal, c),
	}
	installTestRegistry(t, reg)
	return c, &stalenessTickerState{openGrace: testOpenGrace}
}

// primePastGrace puts the state into "market has been open well past the
// grace window" so tests of the other gates are not confounded by it.
func primePastGrace(s *stalenessTickerState, now time.Time) {
	s.wasOpen = true
	s.openedAt = now.Add(-2 * s.openGrace)
}

var (
	marketOpen   = func() (bool, error) { return true, nil }
	marketClosed = func() (bool, error) { return false, nil }
	havePlay     = func() bool { return true }
	noPlay       = func() bool { return false }
)

func TestStalenessCycle_ClosedMarketNeverEvaluates(t *testing.T) {
	c, s := stalenessCycleFixture(t, fakeStaleness{age: time.Hour, ok: true})
	now := time.Now()

	evaluated := s.runCycle(now, marketClosed, havePlay)

	require.False(t, evaluated)
	require.False(t, c.Status().Engaged, "a closed market must never trip the staleness guard, no matter how stale the feed")
}

func TestStalenessCycle_NoRealtimePlaygroundNeverEvaluates(t *testing.T) {
	c, s := stalenessCycleFixture(t, fakeStaleness{age: time.Hour, ok: true})
	now := time.Now()
	primePastGrace(s, now)

	evaluated := s.runCycle(now, marketOpen, noPlay)

	require.False(t, evaluated)
	require.False(t, c.Status().Engaged, "with no realtime playground loaded there is no live feed to guard")
}

func TestStalenessCycle_CalendarErrorSkipsCycleWithoutTripping(t *testing.T) {
	c, s := stalenessCycleFixture(t, fakeStaleness{age: time.Hour, ok: true})
	now := time.Now()
	primePastGrace(s, now)
	openedAtBefore := s.openedAt

	evaluated := s.runCycle(now, func() (bool, error) { return false, errors.New("tradier calendar 503") }, havePlay)

	require.False(t, evaluated)
	require.False(t, c.Status().Engaged, "a calendar API blip must not halt trading (fail-quiet for one cycle)")
	// A blip must not fake a close: the next open cycle must NOT restart the
	// grace window.
	require.True(t, s.wasOpen, "a calendar error must leave the open/closed transition state untouched")
	require.Equal(t, openedAtBefore, s.openedAt)
}

func TestStalenessCycle_OpenMarketWithRealtimePlaygroundEvaluatesAndTrips(t *testing.T) {
	c, s := stalenessCycleFixture(t, fakeStaleness{age: time.Hour, ok: true})
	now := time.Now()
	primePastGrace(s, now)

	evaluated := s.runCycle(now, marketOpen, havePlay)

	require.True(t, evaluated)
	require.True(t, c.Status().Engaged, "an over-threshold stale feed during market hours with a live playground must trip")
	require.Equal(t, SourceAuto, c.Status().Source)
	require.Contains(t, c.Status().Reason, "feed-staleness guard")
}

func TestStalenessCycle_FreshFeedEvaluatesWithoutTripping(t *testing.T) {
	c, s := stalenessCycleFixture(t, fakeStaleness{age: 2 * time.Second, ok: true})
	now := time.Now()
	primePastGrace(s, now)

	evaluated := s.runCycle(now, marketOpen, havePlay)

	require.True(t, evaluated)
	require.False(t, c.Status().Engaged)
}

// --- Market-open boundary (the overnight-gap false-halt repro) ---

// The BLOCKER repro: server up overnight with a live playground; at the open
// the last-Tick age is ~17.5h. The first cycles of the session fall inside
// the grace window and MUST NOT evaluate; once the feed delivers its first
// bars and the grace elapses, evaluation resumes and a fresh feed does not
// trip.
func TestStalenessCycle_OvernightGapAtOpenDoesNotTrip(t *testing.T) {
	signal := &mutableStaleness{age: 17*time.Hour + 30*time.Minute, ok: true}
	c, s := stalenessCycleFixture(t, signal)

	openBell := time.Date(2026, 7, 6, 9, 30, 0, 0, time.UTC)

	// Overnight: market closed, nothing evaluates.
	require.False(t, s.runCycle(openBell.Add(-time.Hour), marketClosed, havePlay))

	// 9:30:00 — closed→open transition with a 17.5h-old last Tick: the grace
	// window suppresses evaluation on this and every in-grace cycle.
	require.False(t, s.runCycle(openBell, marketOpen, havePlay), "the opening-bell cycle must not evaluate the overnight gap")
	require.False(t, c.Status().Engaged, "the overnight gap must not auto-halt at the open")

	for _, dt := range []time.Duration{30 * time.Second, 95 * time.Second, testOpenGrace - time.Second} {
		require.False(t, s.runCycle(openBell.Add(dt), marketOpen, havePlay), "cycle at open+%s is inside the grace window", dt)
	}
	require.False(t, c.Status().Engaged)

	// First bars of the session arrive (heartbeat recorded); after the grace
	// window evaluation resumes against the FRESH age (under the 30s guard
	// threshold) and does not trip.
	signal.age = 10 * time.Second
	require.True(t, s.runCycle(openBell.Add(testOpenGrace), marketOpen, havePlay), "evaluation must resume once the grace window elapses")
	require.False(t, c.Status().Engaged, "a feed that ticked after the open must not trip")
}

// After the open, a heartbeat arrives and then the feed genuinely dies: the
// guard must still trip — the boundary grace must not blind it for the rest
// of the session.
func TestStalenessCycle_HeartbeatAfterOpenThenGenuineStalenessStillTrips(t *testing.T) {
	signal := &mutableStaleness{age: 17 * time.Hour, ok: true}
	c, s := stalenessCycleFixture(t, signal)

	openBell := time.Date(2026, 7, 6, 9, 30, 0, 0, time.UTC)
	require.False(t, s.runCycle(openBell, marketOpen, havePlay))

	// Feed ticks at ~9:31; evaluations after the grace stay clear.
	signal.age = 30 * time.Second
	require.True(t, s.runCycle(openBell.Add(testOpenGrace+30*time.Second), marketOpen, havePlay))
	require.False(t, c.Status().Engaged)

	// The feed dies mid-session: age grows past the 30s guard threshold.
	signal.age = 3 * time.Minute
	require.True(t, s.runCycle(openBell.Add(testOpenGrace+5*time.Minute), marketOpen, havePlay))
	require.True(t, c.Status().Engaged, "genuine staleness after the open-boundary grace must still trip")
	require.Equal(t, SourceAuto, c.Status().Source)
	require.Contains(t, c.Status().Reason, "feed-staleness guard")
}

// A feed that is dead FROM the open (no bar ever arrives) trips once the
// grace window elapses — the grace delays the first evaluation of the
// session, it does not require a heartbeat that may never come.
func TestStalenessCycle_DeadFromOpenTripsAfterGrace(t *testing.T) {
	signal := &mutableStaleness{age: 17 * time.Hour, ok: true}
	c, s := stalenessCycleFixture(t, signal)

	openBell := time.Date(2026, 7, 6, 9, 30, 0, 0, time.UTC)
	require.False(t, s.runCycle(openBell, marketOpen, havePlay))
	require.False(t, c.Status().Engaged)

	require.True(t, s.runCycle(openBell.Add(testOpenGrace), marketOpen, havePlay))
	require.True(t, c.Status().Engaged, "a feed that never delivers a bar after the open is a real outage and must trip after the grace")
}

// Every closed→open transition restarts the grace window (half-days, weekend
// restarts): the gap accrued while closed can never trip at the next open.
func TestStalenessCycle_ReopenRestartsGraceWindow(t *testing.T) {
	signal := &mutableStaleness{age: 30 * time.Second, ok: true}
	c, s := stalenessCycleFixture(t, signal)

	day1Open := time.Date(2026, 7, 6, 9, 30, 0, 0, time.UTC)
	require.False(t, s.runCycle(day1Open, marketOpen, havePlay))
	require.True(t, s.runCycle(day1Open.Add(testOpenGrace), marketOpen, havePlay))
	require.False(t, c.Status().Engaged)

	// Market closes; the overnight gap accrues.
	require.False(t, s.runCycle(day1Open.Add(7*time.Hour), marketClosed, havePlay))
	signal.age = 17 * time.Hour

	// Next open: the grace window applies again.
	day2Open := day1Open.Add(24 * time.Hour)
	require.False(t, s.runCycle(day2Open, marketOpen, havePlay), "the reopen cycle must restart the grace window")
	require.False(t, s.runCycle(day2Open.Add(time.Minute), marketOpen, havePlay))
	require.False(t, c.Status().Engaged, "the second morning's overnight gap must not trip either")
}

func TestCompositeStalenessSignal_MinAvailableAgeSemantics(t *testing.T) {
	// No signals or no observations: unavailable (guard stays inactive).
	empty := NewCompositeStalenessSignal()
	_, ok := empty.LastTickAge()
	require.False(t, ok)

	none := NewCompositeStalenessSignal(fakeStaleness{ok: false}, nil, fakeStaleness{ok: false})
	_, ok = none.LastTickAge()
	require.False(t, ok)

	// The most recent tick across feeds wins (minimum age): a fresh equity
	// feed keeps the composite fresh even when options are quiet...
	mixed := NewCompositeStalenessSignal(
		fakeStaleness{age: 5 * time.Second, ok: true},  // equity
		fakeStaleness{age: 20 * time.Minute, ok: true}, // option
	)
	age, ok := mixed.LastTickAge()
	require.True(t, ok)
	require.Equal(t, 5*time.Second, age)

	// ...and a never-observed class cannot mask real staleness of the one
	// class that IS wired.
	oneStale := NewCompositeStalenessSignal(
		fakeStaleness{age: 20 * time.Minute, ok: true},
		fakeStaleness{ok: false},
	)
	age, ok = oneStale.LastTickAge()
	require.True(t, ok)
	require.Equal(t, 20*time.Minute, age)
}
