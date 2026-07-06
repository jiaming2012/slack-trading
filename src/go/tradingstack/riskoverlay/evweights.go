package riskoverlay

import (
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/tradingstack"
)

// DefaultLookupCacheTTL is the scan-cycle-scale freshness window for the
// cached EV-weight / sector / crowding lookups. EV weights, sectors, and
// crowding views change once per scan cycle (~minutes), so re-reading them per
// evaluated order would be pure PlaceOrder latency for no fidelity gain.
const DefaultLookupCacheTTL = 60 * time.Second

// EvWeightLookup resolves the current EV-weight set: the most recent
// strategy_ev_weights row per strategy_id, as strategyID -> ev_weight. The
// production implementation reads the strategy_ev_weights table; the test fake
// is in-memory. An EMPTY returned map pins the strategy-allocation family
// inactive (see PortfolioState.EvWeights); a lookup ERROR surfaces as a
// snapshot-build error and is handled fail-permissive (observed via the
// degradation counter) by the gate.
type EvWeightLookup interface {
	Latest() (map[string]float64, error)
}

// GormEvWeightLookup is the production EvWeightLookup. It reads the most
// recent strategy_ev_weights row per strategy_id (by computed_at, NULLS LAST)
// and never writes. Rows with a NULL strategy_id or NULL ev_weight are
// skipped: a strategy whose latest row carries no weight is simply absent from
// the returned set (and therefore zero-capped when the set is non-empty).
type GormEvWeightLookup struct {
	db *gorm.DB
}

// NewGormEvWeightLookup constructs a GormEvWeightLookup over an already
// migrated *gorm.DB (see tradingstack.MigrateTradingStack).
func NewGormEvWeightLookup(db *gorm.DB) *GormEvWeightLookup {
	return &GormEvWeightLookup{db: db}
}

// Latest returns the most recent ev_weight per strategy_id.
func (g *GormEvWeightLookup) Latest() (map[string]float64, error) {
	var rows []tradingstack.StrategyEvWeight
	err := g.db.
		Raw(`SELECT DISTINCT ON (strategy_id) *
		     FROM strategy_ev_weights
		     WHERE strategy_id IS NOT NULL
		     ORDER BY strategy_id, computed_at DESC NULLS LAST`).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("GormEvWeightLookup.Latest: load strategy_ev_weights failed: %w", err)
	}

	weights := make(map[string]float64, len(rows))
	for _, row := range rows {
		if row.StrategyID == nil || row.EvWeight == nil {
			continue
		}
		weights[*row.StrategyID] = *row.EvWeight
	}
	return weights, nil
}

// FakeEvWeightLookup is an in-memory, seedable EvWeightLookup for unit tests.
// Set Err to simulate a lookup outage; Calls counts Latest invocations so
// tests can assert the reduction-bypass and cache behavior.
type FakeEvWeightLookup struct {
	Weights map[string]float64
	Err     error
	Calls   int
}

// NewFakeEvWeightLookup constructs a fake seeded with the given weights.
func NewFakeEvWeightLookup(weights map[string]float64) *FakeEvWeightLookup {
	return &FakeEvWeightLookup{Weights: weights}
}

// Latest returns a copy of the seeded weights (or the seeded error).
func (f *FakeEvWeightLookup) Latest() (map[string]float64, error) {
	f.Calls++
	if f.Err != nil {
		return nil, f.Err
	}
	out := make(map[string]float64, len(f.Weights))
	for k, v := range f.Weights {
		out[k] = v
	}
	return out, nil
}

// CachedEvWeightLookup memoizes an inner EvWeightLookup for a TTL so the
// per-order evaluation path stays flat: at most one DB read per scan-cycle-
// scale window. Errors are never cached — the next call retries the inner
// lookup. Safe for concurrent use.
type CachedEvWeightLookup struct {
	inner EvWeightLookup
	ttl   time.Duration
	now   func() time.Time

	mu       sync.Mutex
	cached   map[string]float64
	cachedAt time.Time
	valid    bool
}

// NewCachedEvWeightLookup wraps inner with a TTL cache. A non-positive ttl
// falls back to DefaultLookupCacheTTL.
func NewCachedEvWeightLookup(inner EvWeightLookup, ttl time.Duration) *CachedEvWeightLookup {
	if ttl <= 0 {
		ttl = DefaultLookupCacheTTL
	}
	return &CachedEvWeightLookup{inner: inner, ttl: ttl, now: time.Now}
}

// Latest returns the cached weight set when fresh, otherwise consults the
// inner lookup. Callers receive a copy, so mutating the result cannot poison
// the cache.
func (c *CachedEvWeightLookup) Latest() (map[string]float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.valid || c.now().Sub(c.cachedAt) >= c.ttl {
		fresh, err := c.inner.Latest()
		if err != nil {
			return nil, err
		}
		c.cached = fresh
		c.cachedAt = c.now()
		c.valid = true
	}

	out := make(map[string]float64, len(c.cached))
	for k, v := range c.cached {
		out[k] = v
	}
	return out, nil
}
