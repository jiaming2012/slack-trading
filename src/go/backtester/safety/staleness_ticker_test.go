package safety

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stalenessCycleFixture installs a registry whose staleness signal is ALWAYS
// over threshold, so any evaluation that is not suppressed would trip the
// controller — the gating tests assert the controller stays clear.
func stalenessCycleFixture(t *testing.T) *HaltController {
	t.Helper()
	c := newTestController(t)
	reg := &GuardRegistry{
		Controller:    c,
		FeedStaleness: NewFeedStalenessGuard(30*time.Second, fakeStaleness{age: time.Hour, ok: true}, c),
	}
	installTestRegistry(t, reg)
	return c
}

func TestStalenessCycle_ClosedMarketNeverEvaluates(t *testing.T) {
	c := stalenessCycleFixture(t)

	evaluated := runStalenessEvaluationCycle(
		func() (bool, error) { return false, nil }, // market closed
		func() bool { return true },
	)

	require.False(t, evaluated)
	require.False(t, c.Status().Engaged, "a closed market must never trip the staleness guard, no matter how stale the feed")
}

func TestStalenessCycle_NoRealtimePlaygroundNeverEvaluates(t *testing.T) {
	c := stalenessCycleFixture(t)

	evaluated := runStalenessEvaluationCycle(
		func() (bool, error) { return true, nil },
		func() bool { return false }, // only Simulations loaded
	)

	require.False(t, evaluated)
	require.False(t, c.Status().Engaged, "with no realtime playground loaded there is no live feed to guard")
}

func TestStalenessCycle_CalendarErrorSkipsCycleWithoutTripping(t *testing.T) {
	c := stalenessCycleFixture(t)

	evaluated := runStalenessEvaluationCycle(
		func() (bool, error) { return false, errors.New("tradier calendar 503") },
		func() bool { return true },
	)

	require.False(t, evaluated)
	require.False(t, c.Status().Engaged, "a calendar API blip must not halt trading (fail-quiet for one cycle)")
}

func TestStalenessCycle_OpenMarketWithRealtimePlaygroundEvaluatesAndTrips(t *testing.T) {
	c := stalenessCycleFixture(t)

	evaluated := runStalenessEvaluationCycle(
		func() (bool, error) { return true, nil },
		func() bool { return true },
	)

	require.True(t, evaluated)
	require.True(t, c.Status().Engaged, "an over-threshold stale feed during market hours with a live playground must trip")
	require.Equal(t, SourceAuto, c.Status().Source)
	require.Contains(t, c.Status().Reason, "feed-staleness guard")
}

func TestStalenessCycle_FreshFeedEvaluatesWithoutTripping(t *testing.T) {
	c := newTestController(t)
	reg := &GuardRegistry{
		Controller:    c,
		FeedStaleness: NewFeedStalenessGuard(30*time.Second, fakeStaleness{age: 2 * time.Second, ok: true}, c),
	}
	installTestRegistry(t, reg)

	evaluated := runStalenessEvaluationCycle(
		func() (bool, error) { return true, nil },
		func() bool { return true },
	)

	require.True(t, evaluated)
	require.False(t, c.Status().Engaged)
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
