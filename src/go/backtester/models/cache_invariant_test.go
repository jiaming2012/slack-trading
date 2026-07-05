package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/go/models"
)

// -------------------------------------------------------
// Helpers
// -------------------------------------------------------

type cacheTestEnv struct {
	pg          *Playground
	nextOrderID uint
}

func newCacheTestEnv(t *testing.T, optionSyms []models.OptionSymbol, optionPrices [][]float64) *cacheTestEnv {
	t.Helper()

	tz, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	start := time.Date(2025, time.September, 3, 9, 30, 0, 0, tz)
	end := time.Date(2025, time.September, 6, 16, 0, 0, 0, tz)
	period := time.Minute
	source := models.CandleRepositorySource{Type: "test"}
	stockSym := models.StockSymbol("AAPL")

	n := 30 // enough candles so clock doesn't expire
	var stockCandles []*models.PolygonAggregateBarV2
	for i := 0; i < n; i++ {
		stockCandles = append(stockCandles, &models.PolygonAggregateBarV2{
			Timestamp: start.Add(time.Duration(i) * period),
			Close:     210.0,
		})
	}
	stockRepo, err := NewCandleRepository(stockSym, period, stockCandles, []string{}, nil, 0, source)
	require.NoError(t, err)

	repos := []*CandleRepository{stockRepo}
	brokerData := make(map[models.OptionSymbol][]*models.AggregateBarWithIndicators)

	for idx, sym := range optionSyms {
		prices := optionPrices[idx]
		var candles []*models.PolygonAggregateBarV2
		var bars []*models.AggregateBarWithIndicators
		for i, p := range prices {
			ts := start.Add(time.Duration(i) * period)
			candles = append(candles, &models.PolygonAggregateBarV2{Timestamp: ts, Close: p})
			bars = append(bars, &models.AggregateBarWithIndicators{
				Timestamp: ts, Open: p, High: p, Low: p, Close: p,
			})
		}
		repo, err := NewCandleRepository(sym, period, candles, []string{}, nil, 0, source)
		require.NoError(t, err)
		repos = append(repos, repo)
		brokerData[sym] = bars
	}

	clock := NewClock(start, end, nil)
	pg, err := NewPlayground(PlaygroundConfig{
		Balance: 100_000,
		Clock:   clock,
		Env:     PlaygroundEnvironmentSimulator,
		Now:     start,
		Feeds:   repos,
	})
	require.NoError(t, err)
	pg.OptionsBroker = &MockOptionsBroker{data: brokerData}

	return &cacheTestEnv{pg: pg, nextOrderID: 1}
}

func rep(n int, v float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func (e *cacheTestEnv) placeAndFill(t *testing.T, symbol string, class OrderRecordClass, side TradierOrderSide, qty float64, price float64) {
	t.Helper()
	e.place(t, symbol, class, side, qty, price)
	delta, err := e.pg.Tick(time.Minute, false, nil)
	require.NoError(t, err)
	require.Empty(t, delta.InvalidOrders, "order %d (%s %s qty=%.0f) rejected", e.nextOrderID-1, side, symbol, qty)
}

func (e *cacheTestEnv) place(t *testing.T, symbol string, class OrderRecordClass, side TradierOrderSide, qty float64, price float64) {
	t.Helper()
	id := e.nextOrderID
	e.nextOrderID++
	order, err := NewOrderRecord(id, nil, nil, uuid.Nil, class, LiveAccountTypeMock, e.pg.GetCurrentTime(), symbol, side, qty, Market, Day, price, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
	require.NoError(t, err)
	changes, err := e.pg.PlaceOrder(order)
	require.NoError(t, err)
	for _, c := range changes {
		require.NoError(t, c.Commit())
	}
}

func (e *cacheTestEnv) tickNoRejects(t *testing.T) {
	t.Helper()
	delta, err := e.pg.Tick(time.Minute, false, nil)
	require.NoError(t, err)
	require.Empty(t, delta.InvalidOrders)
}

func (e *cacheTestEnv) assertCache(t *testing.T) {
	t.Helper()
	err := e.pg.validateCache(e.pg.openOrdersCache, e.pg.positionCache)
	require.NoError(t, err, "cache invariant violated")
}

// -------------------------------------------------------
// Tests
// -------------------------------------------------------

func TestCacheInvariant(t *testing.T) {
	optC230 := models.OptionSymbol("AAPL250905C230000")
	optC235 := models.OptionSymbol("AAPL250905C235000")

	t.Run("credit spread full close in one tick", func(t *testing.T) {
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230, optC235},
			[][]float64{rep(30, 5.0), rep(30, 3.0)},
		)

		// Open spread: sell C230, buy C235
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 5.0)
		env.placeAndFill(t, string(optC235), OrderRecordClassOption, TradierOrderSideBuyToOpen, 1, 3.0)
		require.Equal(t, 2, env.pg.openOrdersCache.Len())
		require.Equal(t, 2, env.pg.positionCache.Len())
		env.assertCache(t)

		// Close both legs in one tick
		env.place(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 5.0)
		env.place(t, string(optC235), OrderRecordClassOption, TradierOrderSideSellToClose, 1, 3.0)
		env.tickNoRejects(t)

		require.Equal(t, 0, env.pg.openOrdersCache.Len())
		require.Equal(t, 0, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("multiple closes same instrument one tick", func(t *testing.T) {
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230},
			[][]float64{rep(30, 5.0)},
		)

		// Open qty=2, close with two qty=1 orders in one tick
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 2, 5.0)
		env.assertCache(t)

		env.place(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 5.0)
		env.place(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 5.0)
		env.tickNoRejects(t)

		require.Equal(t, 0, env.pg.openOrdersCache.Len())
		require.Equal(t, 0, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("distributed close across multiple opens", func(t *testing.T) {
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230},
			[][]float64{rep(30, 5.0)},
		)

		// 3 small sell_to_open, 1 large buy_to_close
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 5.0)
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 5.0)
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 5.0)
		pos := env.pg.positionCache.Get(string(optC230))
		require.Equal(t, -3.0, pos.Quantity)
		env.assertCache(t)

		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 3, 5.0)
		require.Equal(t, 0, env.pg.openOrdersCache.Len())
		require.Equal(t, 0, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("equity buy then sell", func(t *testing.T) {
		env := newCacheTestEnv(t, nil, nil)

		env.placeAndFill(t, "AAPL", OrderRecordClassEquity, TradierOrderSideBuy, 10, 100.0)
		require.Equal(t, 1, env.pg.openOrdersCache.Len())
		env.assertCache(t)

		env.placeAndFill(t, "AAPL", OrderRecordClassEquity, TradierOrderSideSell, 10, 100.0)
		require.Equal(t, 0, env.pg.openOrdersCache.Len())
		require.Equal(t, 0, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("equity short sell then cover", func(t *testing.T) {
		env := newCacheTestEnv(t, nil, nil)

		env.placeAndFill(t, "AAPL", OrderRecordClassEquity, TradierOrderSideSellShort, 10, 100.0)
		require.Equal(t, 1, env.pg.openOrdersCache.Len())
		env.assertCache(t)

		env.placeAndFill(t, "AAPL", OrderRecordClassEquity, TradierOrderSideBuyToCover, 10, 100.0)
		require.Equal(t, 0, env.pg.openOrdersCache.Len())
		require.Equal(t, 0, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("close rejected when no matching open side", func(t *testing.T) {
		// sell_to_close when only sell_to_open exists should be rejected at PlaceOrder time.
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230},
			[][]float64{rep(30, 5.0)},
		)

		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 5.0)
		env.assertCache(t)

		// sell_to_close needs buy_to_open to close, but only sell_to_open exists
		// PlaceOrder should reject this immediately
		id := env.nextOrderID
		env.nextOrderID++
		order, err := NewOrderRecord(id, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, env.pg.GetCurrentTime(), string(optC230), TradierOrderSideSellToClose, 1, Market, Day, 5.0, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
		require.NoError(t, err)
		_, err = env.pg.PlaceOrder(order)
		require.Error(t, err, "sell_to_close should be rejected when position is short")
		require.Contains(t, err.Error(), "sell to close")

		// Cache should remain consistent
		require.Equal(t, 1, env.pg.openOrdersCache.Len())
		require.Equal(t, 1, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("partial close preserves cache", func(t *testing.T) {
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230},
			[][]float64{rep(30, 5.0)},
		)

		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 3, 5.0)
		env.assertCache(t)

		// Close 1 of 3
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 5.0)
		pos := env.pg.positionCache.Get(string(optC230))
		require.Equal(t, -2.0, pos.Quantity)
		require.Equal(t, 1, env.pg.openOrdersCache.Len())
		require.Equal(t, 1, env.pg.positionCache.Len())
		env.assertCache(t)

		// Close another 1
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 5.0)
		pos = env.pg.positionCache.Get(string(optC230))
		require.Equal(t, -1.0, pos.Quantity)
		env.assertCache(t)

		// Close last 1
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 5.0)
		require.Equal(t, 0, env.pg.openOrdersCache.Len())
		require.Equal(t, 0, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("two independent spreads open and close separately", func(t *testing.T) {
		// Two spreads on different strikes, close one at a time.
		optC240 := models.OptionSymbol("AAPL250905C240000")
		optC245 := models.OptionSymbol("AAPL250905C245000")
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230, optC235, optC240, optC245},
			[][]float64{rep(30, 5.0), rep(30, 3.0), rep(30, 2.0), rep(30, 1.0)},
		)

		// Spread A: sell C230, buy C235
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 5.0)
		env.placeAndFill(t, string(optC235), OrderRecordClassOption, TradierOrderSideBuyToOpen, 1, 3.0)

		// Spread B: sell C240, buy C245
		env.placeAndFill(t, string(optC240), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 2.0)
		env.placeAndFill(t, string(optC245), OrderRecordClassOption, TradierOrderSideBuyToOpen, 1, 1.0)

		require.Equal(t, 4, env.pg.openOrdersCache.Len())
		require.Equal(t, 4, env.pg.positionCache.Len())
		env.assertCache(t)

		// Close spread A in one tick
		env.place(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 5.0)
		env.place(t, string(optC235), OrderRecordClassOption, TradierOrderSideSellToClose, 1, 3.0)
		env.tickNoRejects(t)

		require.Equal(t, 2, env.pg.openOrdersCache.Len())
		require.Equal(t, 2, env.pg.positionCache.Len())
		env.assertCache(t)

		// Close spread B
		env.place(t, string(optC240), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 2.0)
		env.place(t, string(optC245), OrderRecordClassOption, TradierOrderSideSellToClose, 1, 1.0)
		env.tickNoRejects(t)

		require.Equal(t, 0, env.pg.openOrdersCache.Len())
		require.Equal(t, 0, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("two independent spreads close simultaneously", func(t *testing.T) {
		// Same as above but close all 4 legs in one tick.
		optC240 := models.OptionSymbol("AAPL250905C240000")
		optC245 := models.OptionSymbol("AAPL250905C245000")
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230, optC235, optC240, optC245},
			[][]float64{rep(30, 5.0), rep(30, 3.0), rep(30, 2.0), rep(30, 1.0)},
		)

		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 5.0)
		env.placeAndFill(t, string(optC235), OrderRecordClassOption, TradierOrderSideBuyToOpen, 1, 3.0)
		env.placeAndFill(t, string(optC240), OrderRecordClassOption, TradierOrderSideSellToOpen, 1, 2.0)
		env.placeAndFill(t, string(optC245), OrderRecordClassOption, TradierOrderSideBuyToOpen, 1, 1.0)
		env.assertCache(t)

		// Close all 4 legs in one tick
		env.place(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 5.0)
		env.place(t, string(optC235), OrderRecordClassOption, TradierOrderSideSellToClose, 1, 3.0)
		env.place(t, string(optC240), OrderRecordClassOption, TradierOrderSideBuyToClose, 1, 2.0)
		env.place(t, string(optC245), OrderRecordClassOption, TradierOrderSideSellToClose, 1, 1.0)
		env.tickNoRejects(t)

		require.Equal(t, 0, env.pg.openOrdersCache.Len())
		require.Equal(t, 0, env.pg.positionCache.Len())
		env.assertCache(t)
	})

	t.Run("sell_to_open on existing long position is rejected", func(t *testing.T) {
		// sell_to_open when there's already a long position should be rejected.
		// The correct action is sell_to_close to reduce the position.
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230},
			[][]float64{rep(30, 5.0)},
		)

		// Open long position
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideBuyToOpen, 5, 5.0)
		pos := env.pg.positionCache.Get(string(optC230))
		require.Equal(t, 5.0, pos.Quantity)
		env.assertCache(t)

		// sell_to_open on same symbol — should be REJECTED
		id := env.nextOrderID
		env.nextOrderID++
		order, err := NewOrderRecord(id, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, env.pg.GetCurrentTime(), string(optC230), TradierOrderSideSellToOpen, 5, Market, Day, 5.0, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
		require.NoError(t, err)
		_, err = env.pg.PlaceOrder(order)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot sell to open when long position")

		// Position should be unchanged
		pos = env.pg.positionCache.Get(string(optC230))
		require.Equal(t, 5.0, pos.Quantity)
		env.assertCache(t)
	})

	t.Run("buy_to_open on existing short position is rejected", func(t *testing.T) {
		// buy_to_open when there's already a short position should be rejected.
		// The correct action is buy_to_close to reduce the position.
		env := newCacheTestEnv(t,
			[]models.OptionSymbol{optC230},
			[][]float64{rep(30, 5.0)},
		)

		// Open short position
		env.placeAndFill(t, string(optC230), OrderRecordClassOption, TradierOrderSideSellToOpen, 5, 5.0)
		pos := env.pg.positionCache.Get(string(optC230))
		require.Equal(t, -5.0, pos.Quantity)
		env.assertCache(t)

		// buy_to_open on same symbol — should be REJECTED
		id := env.nextOrderID
		env.nextOrderID++
		order, err := NewOrderRecord(id, nil, nil, uuid.Nil, OrderRecordClassOption, LiveAccountTypeMock, env.pg.GetCurrentTime(), string(optC230), TradierOrderSideBuyToOpen, 5, Market, Day, 5.0, nil, nil, OrderRecordStatusPending, "", nil, false, nil, nil)
		require.NoError(t, err)
		_, err = env.pg.PlaceOrder(order)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot buy to open when short position")

		// Position should be unchanged
		pos = env.pg.positionCache.Get(string(optC230))
		require.Equal(t, -5.0, pos.Quantity)
		env.assertCache(t)
	})
}
