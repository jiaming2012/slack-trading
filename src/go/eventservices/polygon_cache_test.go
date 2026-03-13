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
	if got := cache.GetAggregateBars(sym, start, end); got != nil {
		t.Fatal("expected nil on cache miss")
	}

	bars := &eventmodels.AggregateResult[eventmodels.PolygonAggregateBar]{
		ResultsCount: 2,
		Results: []eventmodels.PolygonAggregateBar{
			{Open: 5.0, Close: 5.1},
			{Open: 5.1, Close: 5.2},
		},
	}
	cache.SetAggregateBars(sym, start, end, bars)

	got := cache.GetAggregateBars(sym, start, end)
	if got == nil {
		t.Fatal("expected cache hit")
	}
	if got.ResultsCount != 2 {
		t.Fatalf("expected 2 results, got %d", got.ResultsCount)
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
