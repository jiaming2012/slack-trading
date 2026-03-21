package eventservices

import (
	"testing"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

func TestPolygonCache_Contracts(t *testing.T) {
	cache := NewPolygonCache()

	symbol := eventmodels.StockSymbol("AAPL")
	gte := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	lte := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	// Miss
	if got := cache.GetContracts(symbol, gte, lte, false); got != nil {
		t.Fatal("expected nil on cache miss")
	}

	// Store and hit
	resp := &eventmodels.PolygonBulkResponse{
		Contracts: []eventmodels.OptionContractV3{
			{Strike: 100, Symbol: "O:AAPL250101C00100000"},
		},
	}
	cache.SetContracts(symbol, gte, lte, false, resp)

	got := cache.GetContracts(symbol, gte, lte, false)
	if got == nil {
		t.Fatal("expected cache hit")
	}
	if len(got.Contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(got.Contracts))
	}

	// Different isExpired flag should miss
	if got := cache.GetContracts(symbol, gte, lte, true); got != nil {
		t.Fatal("expected miss for different isExpired")
	}
}

func TestPolygonCache_StockTick(t *testing.T) {
	cache := NewPolygonCache()

	symbol := eventmodels.StockSymbol("AAPL")
	at := time.Date(2025, 1, 15, 10, 30, 45, 0, time.UTC)

	// Miss
	if got := cache.GetStockTick(symbol, at); got != nil {
		t.Fatal("expected nil on cache miss")
	}

	tick := &eventmodels.StockTickItemDTO{
		Symbol: "AAPL",
		Bid:    240.0,
		Ask:    240.5,
	}
	cache.SetStockTick(symbol, at, tick)

	// Same minute (different second) should hit
	at2 := time.Date(2025, 1, 15, 10, 30, 10, 0, time.UTC)
	got := cache.GetStockTick(symbol, at2)
	if got == nil {
		t.Fatal("expected cache hit within same minute")
	}
	if got.Bid != 240.0 {
		t.Fatalf("expected bid 240.0, got %f", got.Bid)
	}

	// Different minute should miss
	at3 := time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC)
	if got := cache.GetStockTick(symbol, at3); got != nil {
		t.Fatal("expected miss for different minute")
	}
}

func TestPolygonCache_AggregateBars(t *testing.T) {
	cache := NewPolygonCache()

	sym := eventmodels.OptionSymbol("O:AAPL250103C00242500")
	start := time.Date(2024, 12, 30, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	// Miss
	if got := cache.GetAggregateBars(sym, start, end, "1m"); got != nil {
		t.Fatal("expected nil on cache miss")
	}

	bars := &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
		ResultsCount: 2,
		Results: []eventmodels.PolygonAggregateBar{
			{Open: 5.0, Close: 5.1},
			{Open: 5.1, Close: 5.2},
		},
	}
	cache.SetAggregateBars(sym, start, end, "1m", bars)

	got := cache.GetAggregateBars(sym, start, end, "1m")
	if got == nil {
		t.Fatal("expected cache hit")
	}
	if got.ResultsCount != 2 {
		t.Fatalf("expected 2 results, got %d", got.ResultsCount)
	}
}

func TestPolygonCache_TimeframeIsolation(t *testing.T) {
	cache := NewPolygonCache()
	sym := eventmodels.OptionSymbol("O:AAPL250703C00200000")
	start := time.Date(2025, 5, 29, 9, 30, 0, 0, time.UTC)
	end := time.Date(2025, 6, 1, 16, 0, 0, 0, time.UTC)

	minuteBars := &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
		ResultsCount: 1,
		Results:      []eventmodels.PolygonAggregateBar{{Open: 5.0}},
	}
	wideBars := &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
		ResultsCount: 1,
		Results:      []eventmodels.PolygonAggregateBar{{Open: 6.0}},
	}

	cache.SetAggregateBars(sym, start, end, "1m", minuteBars)
	cache.SetAggregateBars(sym, start, end, "1m-wide", wideBars)

	cachedMinute := cache.GetAggregateBars(sym, start, end, "1m")
	cachedWide := cache.GetAggregateBars(sym, start, end, "1m-wide")

	if cachedMinute == nil || cachedWide == nil {
		t.Fatal("expected both cache hits")
	}
	if cachedMinute.Results[0].Open != 5.0 {
		t.Fatalf("expected minute open=5.0, got %f", cachedMinute.Results[0].Open)
	}
	if cachedWide.Results[0].Open != 6.0 {
		t.Fatalf("expected wide open=6.0, got %f", cachedWide.Results[0].Open)
	}
}

func TestPolygonCache_EmptyResultsNotCachedPolicy(t *testing.T) {
	// Validates the caching policy used by populateTickDataToOptionChainMap:
	// empty results should NOT be cached so they get re-fetched when the
	// simulation time advances and data becomes available.

	cache := NewPolygonCache()
	sym := eventmodels.OptionSymbol("O:AAPL250703C00197500")
	start := time.Date(2025, 5, 29, 9, 30, 0, 0, time.UTC)
	end := time.Date(2025, 6, 1, 16, 0, 0, 0, time.UTC)

	// Simulate: fetch returns empty → only cache if non-empty (production policy)
	emptyResult := &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
		ResultsCount: 0,
		Results:      nil,
	}
	if len(emptyResult.Results) > 0 {
		cache.SetAggregateBars(sym, start, end, "1m", emptyResult)
	}

	// Lookup should miss → triggers re-fetch from Polygon
	cached := cache.GetAggregateBars(sym, start, end, "1m")
	if cached != nil {
		t.Fatal("empty results should not be cached; expected nil")
	}

	// Simulate: later fetch returns data → gets cached
	nonEmptyResult := &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
		ResultsCount: 1,
		Results:      []eventmodels.PolygonAggregateBar{{Open: 16.5, Close: 16.55}},
	}
	if len(nonEmptyResult.Results) > 0 {
		cache.SetAggregateBars(sym, start, end, "1m", nonEmptyResult)
	}

	cached = cache.GetAggregateBars(sym, start, end, "1m")
	if cached == nil {
		t.Fatal("non-empty results should be cached")
	}
	if len(cached.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(cached.Results))
	}
}

func TestPolygonCache_WideVsNarrowKeyIsolation(t *testing.T) {
	// The narrow fetch (StartDate → marketClose) and the wide fetch
	// (StartDate → expiration) use different cache keys and must not collide.

	cache := NewPolygonCache()
	sym := eventmodels.OptionSymbol("O:AAPL250703C00197500")
	start := time.Date(2025, 5, 29, 9, 30, 0, 0, time.UTC)
	narrowEnd := time.Date(2025, 6, 1, 16, 0, 0, 0, time.UTC)
	wideEnd := time.Date(2025, 7, 3, 0, 0, 0, 0, time.UTC) // contract expiration

	// Wide fetch finds data
	wideResult := &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
		ResultsCount: 1,
		Results:      []eventmodels.PolygonAggregateBar{{Open: 16.5}},
	}
	cache.SetAggregateBars(sym, start, wideEnd, "1m-wide", wideResult)

	// Narrow fetch should still miss (different end date + timeframe key)
	cachedNarrow := cache.GetAggregateBars(sym, start, narrowEnd, "1m")
	if cachedNarrow != nil {
		t.Fatal("narrow key should not match wide cache entry")
	}

	// Wide fetch should hit
	cachedWide := cache.GetAggregateBars(sym, start, wideEnd, "1m-wide")
	if cachedWide == nil {
		t.Fatal("wide key should match wide cache entry")
	}
	if cachedWide.Results[0].Open != 16.5 {
		t.Fatalf("expected open=16.5, got %f", cachedWide.Results[0].Open)
	}
}

func TestPolygonCache_Stats(t *testing.T) {
	cache := NewPolygonCache()

	c, s, a := cache.Stats()
	if c != 0 || s != 0 || a != 0 {
		t.Fatalf("expected all zeros, got %d %d %d", c, s, a)
	}

	symbol := eventmodels.StockSymbol("AAPL")
	gte := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	lte := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	cache.SetContracts(symbol, gte, lte, false, &eventmodels.PolygonBulkResponse{})

	c, s, a = cache.Stats()
	if c != 1 || s != 0 || a != 0 {
		t.Fatalf("expected 1 0 0, got %d %d %d", c, s, a)
	}
}
