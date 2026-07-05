package eventservices

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
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
	contracts map[string]*models.PolygonBulkResponse

	// stockTickMu guards the stockTick cache
	stockTickMu sync.RWMutex
	// stockTick caches FindClosestStockTickItemDTO results keyed by
	// (symbol, timestamp rounded to minute)
	stockTick map[string]*models.StockTickItemDTO

	// aggregateBarsMu guards the aggregateBars cache
	aggregateBarsMu sync.RWMutex
	// aggregateBars caches per-contract option aggregate bar fetches keyed by
	// (optionSymbol, startDate, endDate)
	aggregateBars map[string]*models.AggregateResult[models.PolygonAggregateBar]

	// cacheDir is the directory for disk persistence (empty = no persistence)
	cacheDir string
	// dirty tracks whether aggregate bars have been modified since last flush
	dirty bool
}

// NewPolygonCache creates an empty cache. If cacheDir is non-empty, loads
// persisted aggregate bars from disk.
func NewPolygonCache(cacheDir ...string) *PolygonCache {
	c := &PolygonCache{
		contracts:     make(map[string]*models.PolygonBulkResponse),
		stockTick:     make(map[string]*models.StockTickItemDTO),
		aggregateBars: make(map[string]*models.AggregateResult[models.PolygonAggregateBar]),
	}

	if len(cacheDir) > 0 && cacheDir[0] != "" {
		c.cacheDir = cacheDir[0]
		if err := c.loadAggregateBarsFromDisk(); err != nil {
			log.Warnf("PolygonCache: failed to load disk cache: %v", err)
		}
	}

	return c
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

func contractsCacheKey(symbol models.StockSymbol, expGTE, expLTE time.Time, isExpired bool) string {
	return fmt.Sprintf("%s|%s|%s|%t", symbol, expGTE.Format("2006-01-02"), expLTE.Format("2006-01-02"), isExpired)
}

// GetContracts returns a cached contract list, or nil if not present.
func (c *PolygonCache) GetContracts(symbol models.StockSymbol, expGTE, expLTE time.Time, isExpired bool) *models.PolygonBulkResponse {
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
func (c *PolygonCache) SetContracts(symbol models.StockSymbol, expGTE, expLTE time.Time, isExpired bool, resp *models.PolygonBulkResponse) {
	key := contractsCacheKey(symbol, expGTE, expLTE, isExpired)
	c.contractsMu.Lock()
	c.contracts[key] = resp
	c.contractsMu.Unlock()
	log.Debugf("PolygonCache.SetContracts STORE: %s (%d contracts)", key, len(resp.Contracts))
}

// ---------------------------------------------------------------------------
// Stock tick cache (FindClosestStockTickItemDTO)
// ---------------------------------------------------------------------------

func stockTickCacheKey(symbol models.StockSymbol, at time.Time) string {
	// Round to the minute so nearby calls within the same minute share cache
	rounded := at.Truncate(time.Minute)
	return fmt.Sprintf("%s|%s", symbol, rounded.Format("2006-01-02T15:04"))
}

// GetStockTick returns a cached stock tick, or nil if not present.
func (c *PolygonCache) GetStockTick(symbol models.StockSymbol, at time.Time) *models.StockTickItemDTO {
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
func (c *PolygonCache) SetStockTick(symbol models.StockSymbol, at time.Time, tick *models.StockTickItemDTO) {
	key := stockTickCacheKey(symbol, at)
	c.stockTickMu.Lock()
	c.stockTick[key] = tick
	c.stockTickMu.Unlock()
	log.Debugf("PolygonCache.SetStockTick STORE: %s", key)
}

// ---------------------------------------------------------------------------
// Aggregate bars cache (per-option-contract minute bars)
// ---------------------------------------------------------------------------

func aggregateBarsCacheKey(optionSymbol models.OptionSymbol, startDate, endDate time.Time, timeframe string) string {
	return fmt.Sprintf("%s|%s|%s|%s", optionSymbol, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), timeframe)
}

// GetAggregateBars returns cached aggregate bars for a single option contract,
// or nil if not present.
func (c *PolygonCache) GetAggregateBars(optionSymbol models.OptionSymbol, startDate, endDate time.Time, timeframe string) *models.AggregateResult[models.PolygonAggregateBar] {
	key := aggregateBarsCacheKey(optionSymbol, startDate, endDate, timeframe)
	c.aggregateBarsMu.RLock()
	defer c.aggregateBarsMu.RUnlock()
	if v, ok := c.aggregateBars[key]; ok {
		log.Debugf("PolygonCache.GetAggregateBars HIT: %s", key)
		return v
	}
	return nil
}

// SetAggregateBars stores aggregate bars for a single option contract.
// If disk persistence is enabled, flushes to disk periodically.
func (c *PolygonCache) SetAggregateBars(optionSymbol models.OptionSymbol, startDate, endDate time.Time, timeframe string, bars *models.AggregateResult[models.PolygonAggregateBar]) {
	key := aggregateBarsCacheKey(optionSymbol, startDate, endDate, timeframe)
	c.aggregateBarsMu.Lock()
	c.aggregateBars[key] = bars
	c.dirty = true
	size := len(c.aggregateBars)
	c.aggregateBarsMu.Unlock()
	log.Debugf("PolygonCache.SetAggregateBars STORE: %s (%d bars)", key, len(bars.Results))

	// Auto-flush every 50 new entries
	if c.cacheDir != "" && size%50 == 0 {
		go func() {
			if err := c.FlushAggregateBars(); err != nil {
				log.Warnf("PolygonCache: background flush failed: %v", err)
			}
		}()
	}
}

// FlushAggregateBars writes the aggregate bars cache to disk.
func (c *PolygonCache) FlushAggregateBars() error {
	if c.cacheDir == "" {
		return nil
	}

	c.aggregateBarsMu.RLock()
	if !c.dirty {
		c.aggregateBarsMu.RUnlock()
		return nil
	}

	// Snapshot the data under read lock
	data := make(map[string]*models.AggregateResult[models.PolygonAggregateBar], len(c.aggregateBars))
	for k, v := range c.aggregateBars {
		data[k] = v
	}
	c.aggregateBarsMu.RUnlock()

	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	path := filepath.Join(c.cacheDir, "aggregate_bars.json")
	f, err := os.CreateTemp(c.cacheDir, "aggregate_bars_*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(f.Name()) // clean up on error

	enc := json.NewEncoder(f)
	if err := enc.Encode(data); err != nil {
		f.Close()
		return fmt.Errorf("encode: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	c.aggregateBarsMu.Lock()
	c.dirty = false
	c.aggregateBarsMu.Unlock()

	log.Infof("PolygonCache: flushed %d aggregate bar entries to %s", len(data), path)
	return nil
}

// loadAggregateBarsFromDisk loads persisted aggregate bars into memory.
func (c *PolygonCache) loadAggregateBarsFromDisk() error {
	path := filepath.Join(c.cacheDir, "aggregate_bars.json")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no cache file yet
		}
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	data := make(map[string]*models.AggregateResult[models.PolygonAggregateBar])
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	c.aggregateBarsMu.Lock()
	for k, v := range data {
		c.aggregateBars[k] = v
	}
	c.aggregateBarsMu.Unlock()

	log.Infof("PolygonCache: loaded %d aggregate bar entries from %s", len(data), path)
	return nil
}
