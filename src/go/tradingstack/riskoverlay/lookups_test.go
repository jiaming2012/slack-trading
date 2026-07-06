package riskoverlay

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The fakes round-trip seeded values and simulate outages.
func TestFakeEvWeightLookup(t *testing.T) {
	fake := NewFakeEvWeightLookup(map[string]float64{"cc-v7": 0.75, "wheel": 0.25})

	weights, err := fake.Latest()
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"cc-v7": 0.75, "wheel": 0.25}, weights)
	require.Equal(t, 1, fake.Calls)

	// Mutating the returned map must not poison the fake's seed.
	weights["cc-v7"] = 999
	again, err := fake.Latest()
	require.NoError(t, err)
	require.Equal(t, 0.75, again["cc-v7"])

	fake.Err = errors.New("boom")
	_, err = fake.Latest()
	require.Error(t, err)
}

func TestFakeSectorLookup(t *testing.T) {
	fake := NewFakeSectorLookup(map[string]string{"AAPL": "technology"})

	sector, err := fake.SectorOf("AAPL")
	require.NoError(t, err)
	require.Equal(t, "technology", sector)

	unknown, err := fake.SectorOf("ZZZQ")
	require.NoError(t, err)
	require.Equal(t, "", unknown, "unknown ticker resolves to the empty sector with no error")
	require.Equal(t, 2, fake.Calls)

	fake.Err = errors.New("boom")
	_, err = fake.SectorOf("AAPL")
	require.Error(t, err)
}

// The EV cache consults the inner lookup once per TTL window, never caches
// errors, and hands out defensive copies.
func TestCachedEvWeightLookup(t *testing.T) {
	inner := NewFakeEvWeightLookup(map[string]float64{"A": 1.0})
	cached := NewCachedEvWeightLookup(inner, time.Minute)

	clock := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	cached.now = func() time.Time { return clock }

	first, err := cached.Latest()
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"A": 1.0}, first)
	require.Equal(t, 1, inner.Calls)

	// Within the TTL: served from cache, inner not consulted again.
	clock = clock.Add(30 * time.Second)
	_, err = cached.Latest()
	require.NoError(t, err)
	require.Equal(t, 1, inner.Calls)

	// Mutating the returned map must not poison the cache.
	first["A"] = 999
	fromCache, err := cached.Latest()
	require.NoError(t, err)
	require.Equal(t, 1.0, fromCache["A"])

	// Past the TTL: refreshed from the inner lookup.
	clock = clock.Add(time.Minute)
	inner.Weights = map[string]float64{"A": 2.0}
	refreshed, err := cached.Latest()
	require.NoError(t, err)
	require.Equal(t, 2.0, refreshed["A"])
	require.Equal(t, 2, inner.Calls)
}

// An inner error is surfaced, never cached, and a later success repopulates.
func TestCachedEvWeightLookup_ErrorNotCached(t *testing.T) {
	inner := NewFakeEvWeightLookup(map[string]float64{"A": 1.0})
	inner.Err = errors.New("db down")
	cached := NewCachedEvWeightLookup(inner, time.Minute)

	_, err := cached.Latest()
	require.Error(t, err)

	inner.Err = nil
	weights, err := cached.Latest()
	require.NoError(t, err)
	require.Equal(t, 1.0, weights["A"])
}

// The sector cache memoizes per ticker, including the negative "no sector"
// result, and refreshes past the TTL.
func TestCachedSectorLookup(t *testing.T) {
	inner := NewFakeSectorLookup(map[string]string{"AAPL": "technology"})
	cached := NewCachedSectorLookup(inner, time.Minute)

	clock := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	cached.now = func() time.Time { return clock }

	sector, err := cached.SectorOf("AAPL")
	require.NoError(t, err)
	require.Equal(t, "technology", sector)
	require.Equal(t, 1, inner.Calls)

	// Negative result cached too: an OCC option symbol never gains a sector,
	// and hammering the DB for it per order would defeat the cache.
	empty, err := cached.SectorOf("AAPL250718C00200000")
	require.NoError(t, err)
	require.Equal(t, "", empty)
	require.Equal(t, 2, inner.Calls)

	clock = clock.Add(30 * time.Second)
	_, _ = cached.SectorOf("AAPL")
	_, _ = cached.SectorOf("AAPL250718C00200000")
	require.Equal(t, 2, inner.Calls, "both positive and negative entries served from cache within the TTL")

	clock = clock.Add(time.Minute)
	_, _ = cached.SectorOf("AAPL")
	require.Equal(t, 3, inner.Calls, "TTL expiry refreshes from the inner lookup")
}

func TestCachedSectorLookup_ErrorNotCached(t *testing.T) {
	inner := NewFakeSectorLookup(map[string]string{"AAPL": "technology"})
	inner.Err = errors.New("db down")
	cached := NewCachedSectorLookup(inner, time.Minute)

	_, err := cached.SectorOf("AAPL")
	require.Error(t, err)

	inner.Err = nil
	sector, err := cached.SectorOf("AAPL")
	require.NoError(t, err)
	require.Equal(t, "technology", sector)
}

// The crowding cache serves a fresh view without re-consulting the inner
// lookup and refreshes past the TTL.
func TestCachedCrowdingLookup(t *testing.T) {
	scannedAt := time.Date(2026, 7, 5, 14, 0, 0, 0, time.UTC)
	inner := &countingCrowdingLookup{inner: NewFakeCrowdingLookup()}
	inner.inner.Seed(scannedAt, true, []string{"NVDA"})

	cached := NewCachedCrowdingLookup(inner, time.Minute)
	clock := scannedAt
	cached.now = func() time.Time { return clock }

	view, err := cached.ViewForScanCycle(scannedAt)
	require.NoError(t, err)
	require.True(t, view.IsFlagged("NVDA"))
	require.Equal(t, 1, inner.calls)

	clock = clock.Add(30 * time.Second)
	_, err = cached.ViewForScanCycle(scannedAt)
	require.NoError(t, err)
	require.Equal(t, 1, inner.calls)

	clock = clock.Add(time.Minute)
	_, err = cached.ViewForScanCycle(scannedAt)
	require.NoError(t, err)
	require.Equal(t, 2, inner.calls)
}

func TestCachedCrowdingLookup_ErrorNotCached(t *testing.T) {
	cached := NewCachedCrowdingLookup(erroringCrowdingLookup{}, time.Minute)
	_, err := cached.ViewForScanCycle(time.Now())
	require.Error(t, err)
}

// countingCrowdingLookup counts pass-through calls to a FakeCrowdingLookup.
type countingCrowdingLookup struct {
	inner *FakeCrowdingLookup
	calls int
}

func (c *countingCrowdingLookup) ViewForScanCycle(scannedAt time.Time) (CrowdingView, error) {
	c.calls++
	return c.inner.ViewForScanCycle(scannedAt)
}
