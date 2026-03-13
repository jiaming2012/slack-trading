package eventservices

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

// PolygonCache provides an in-memory cache for Polygon API responses to avoid
// redundant HTTP calls during backtesting. It is safe for concurrent use.
//
// The cache is scoped to a PolygonOptionsClient instance — when the client is
// garbage-collected, the cache goes with it. For long-running servers the cache
// should be cleared periodically or scoped per-playground; the current
// implementation is intentionally simple (no TTL / eviction).
//
// Phase 3 TODO: embed account state in TickDelta proto to eliminate per-tick GetAccount RPCs
// Phase 4 TODO: add a BatchTick RPC to advance multiple ticks in one call
// Phase 5 TODO: pre-fetch all option chain data at playground creation for backtests
type PolygonCache struct {
	// contractsMu guards the contracts cache
	contractsMu sync.RWMutex
	// contracts caches fetchPolygonBulkHistOptionOhlc results keyed by
	// (symbol, expGTE, expLTE, isExpired)
	contracts map[string]*eventmodels.PolygonBulkResponse

	// stockTickMu guards the stockTick cache
	stockTickMu sync.RWMutex
	// stockTick caches FindClosestStockTickItemDTO results keyed by
	// (symbol, timestamp rounded to minute)
	stockTick map[string]*eventmodels.StockTickItemDTO

	// aggregateBarsMu guards the aggregateBars cache
	aggregateBarsMu sync.RWMutex
	// aggregateBars caches per-contract option aggregate bar fetches keyed by
	// (optionSymbol, startDate, endDate)
	aggregateBars map[string]*eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]
}

// NewPolygonCache creates an empty cache.
func NewPolygonCache() *PolygonCache {
	return &PolygonCache{
		contracts:     make(map[string]*eventmodels.PolygonBulkResponse),
		stockTick:     make(map[string]*eventmodels.StockTickItemDTO),
		aggregateBars: make(map[string]*eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]),
	}
}

// Stats returns the number of entries in each cache bucket (for logging/debugging).
func (c *PolygonCache) Stats() (contracts, stockTicks, aggregateBars int) {
	c.contractsMu.RLock()
	contracts = len(c.contracts)
	c.contractsMu.RUnlock()

	c.stockTickMu.RLock()
	stockTicks = len(c.stockTick)
	c.stockTickMu.RUnlock()

	c.aggregateBarsMu.RLock()
	aggregateBars = len(c.aggregateBars)
	c.aggregateBarsMu.RUnlock()

	return
}

// ---------------------------------------------------------------------------
// Contract list cache (fetchPolygonBulkHistOptionOhlc)
// ---------------------------------------------------------------------------

func contractsCacheKey(symbol eventmodels.StockSymbol, expGTE, expLTE time.Time, isExpired bool) string {
	return fmt.Sprintf("%s|%s|%s|%t", symbol, expGTE.Format("2006-01-02"), expLTE.Format("2006-01-02"), isExpired)
}

// GetContracts returns a cached contract list, or nil if not present.
func (c *PolygonCache) GetContracts(symbol eventmodels.StockSymbol, expGTE, expLTE time.Time, isExpired bool) *eventmodels.PolygonBulkResponse {
	key := contractsCacheKey(symbol, expGTE, expLTE, isExpired)
	c.contractsMu.RLock()
	defer c.contractsMu.RUnlock()
	if v, ok := c.contracts[key]; ok {
		log.Debugf("PolygonCache.GetContracts HIT: %s", key)
		return v
	}
	return nil
}

// SetContracts stores a contract list in the cache.
func (c *PolygonCache) SetContracts(symbol eventmodels.StockSymbol, expGTE, expLTE time.Time, isExpired bool, resp *eventmodels.PolygonBulkResponse) {
	key := contractsCacheKey(symbol, expGTE, expLTE, isExpired)
	c.contractsMu.Lock()
	c.contracts[key] = resp
	c.contractsMu.Unlock()
	log.Debugf("PolygonCache.SetContracts STORE: %s (%d contracts)", key, len(resp.Contracts))
}

// ---------------------------------------------------------------------------
// Stock tick cache (FindClosestStockTickItemDTO)
// ---------------------------------------------------------------------------

func stockTickCacheKey(symbol eventmodels.StockSymbol, at time.Time) string {
	// Round to the minute so nearby calls within the same minute share cache
	rounded := at.Truncate(time.Minute)
	return fmt.Sprintf("%s|%s", symbol, rounded.Format("2006-01-02T15:04"))
}

// GetStockTick returns a cached stock tick, or nil if not present.
func (c *PolygonCache) GetStockTick(symbol eventmodels.StockSymbol, at time.Time) *eventmodels.StockTickItemDTO {
	key := stockTickCacheKey(symbol, at)
	c.stockTickMu.RLock()
	defer c.stockTickMu.RUnlock()
	if v, ok := c.stockTick[key]; ok {
		log.Debugf("PolygonCache.GetStockTick HIT: %s", key)
		return v
	}
	return nil
}

// SetStockTick stores a stock tick in the cache.
func (c *PolygonCache) SetStockTick(symbol eventmodels.StockSymbol, at time.Time, tick *eventmodels.StockTickItemDTO) {
	key := stockTickCacheKey(symbol, at)
	c.stockTickMu.Lock()
	c.stockTick[key] = tick
	c.stockTickMu.Unlock()
	log.Debugf("PolygonCache.SetStockTick STORE: %s", key)
}

// ---------------------------------------------------------------------------
// Aggregate bars cache (per-option-contract minute bars)
// ---------------------------------------------------------------------------

func aggregateBarsCacheKey(optionSymbol eventmodels.OptionSymbol, startDate, endDate time.Time) string {
	return fmt.Sprintf("%s|%s|%s", optionSymbol, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
}

// GetAggregateBars returns cached aggregate bars for a single option contract,
// or nil if not present.
func (c *PolygonCache) GetAggregateBars(optionSymbol eventmodels.OptionSymbol, startDate, endDate time.Time) *eventmodels.AggregateResult[eventmodels.PolygonAggregateBar] {
	key := aggregateBarsCacheKey(optionSymbol, startDate, endDate)
	c.aggregateBarsMu.RLock()
	defer c.aggregateBarsMu.RUnlock()
	if v, ok := c.aggregateBars[key]; ok {
		log.Debugf("PolygonCache.GetAggregateBars HIT: %s", key)
		return v
	}
	return nil
}

// SetAggregateBars stores aggregate bars for a single option contract.
func (c *PolygonCache) SetAggregateBars(optionSymbol eventmodels.OptionSymbol, startDate, endDate time.Time, bars *eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]) {
	key := aggregateBarsCacheKey(optionSymbol, startDate, endDate)
	c.aggregateBarsMu.Lock()
	c.aggregateBars[key] = bars
	c.aggregateBarsMu.Unlock()
	log.Debugf("PolygonCache.SetAggregateBars STORE: %s (%d bars)", key, len(bars.Results))
}
