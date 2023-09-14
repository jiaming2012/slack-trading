package models

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestOpenTrades(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	prc := 2.0
	sl := 1.8
	timeframe := 5
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)

	t.Run("empty trades return zero open trades", func(t *testing.T) {
		trades := &Trades{}
		openTrades := trades.OpenTrades()
		assert.NotNil(t, openTrades)
		assert.Len(t, *openTrades, 0)
	})

	t.Run("single trade", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl)
		tr1.AutoExecute()
		assert.NoError(t, err)

		trades := &Trades{}
		trades.Add(tr1)
		openTrades := trades.OpenTrades()
		assert.Len(t, *openTrades, 1)
		assert.Equal(t, tr1, (*openTrades)[0])
	})

	t.Run("close trade", func(t *testing.T) {
		vol := 1.0
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, vol, sl)
		tr1.AutoExecute()
		assert.NoError(t, err)

		trades := &Trades{}
		trades.Add(tr1)
		openTrades := trades.OpenTrades()
		assert.Len(t, *openTrades, 1)
		assert.Equal(t, tr1, (*openTrades)[0])

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, prc, vol)
		tr2.AutoExecute()
		assert.NoError(t, err)
		trades.Add(tr2)
		openTrades = trades.OpenTrades()
		assert.Len(t, *openTrades, 0)
	})

	t.Run("partial close", func(t *testing.T) {
		vol := 1.0
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, vol, sl)
		tr1.AutoExecute()
		assert.NoError(t, err)

		trades := &Trades{}
		trades.Add(tr1)
		openTrades := trades.OpenTrades()
		assert.Len(t, *openTrades, 1)
		assert.Equal(t, tr1, (*openTrades)[0])

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, prc, vol/2.0)
		tr2.AutoExecute()
		assert.NoError(t, err)
		trades.Add(tr2)
		openTrades = trades.OpenTrades()
		assert.Len(t, *openTrades, 1)
		assert.Equal(t, tr1, (*openTrades)[0])
	})

	t.Run("multiple closes", func(t *testing.T) {
		vol := 1.0
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, vol, sl)
		tr1.AutoExecute()
		assert.NoError(t, err)

		tr2, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, vol, sl)
		tr2.AutoExecute()
		assert.NoError(t, err)

		tr3, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, vol, sl)
		tr3.AutoExecute()
		assert.NoError(t, err)

		trades := &Trades{}
		trades.Add(tr1)
		trades.Add(tr2)
		trades.Add(tr3)
		openTrades := trades.OpenTrades()
		assert.Len(t, *openTrades, 3)
		assert.Equal(t, tr1, (*openTrades)[0])

		// partial close trade 2
		tr4, err := NewCloseTrade(id, []*Trade{tr2}, timeframe, ts, prc, vol/2.0)
		tr4.AutoExecute()
		assert.NoError(t, err)
		trades.Add(tr4)
		openTrades = trades.OpenTrades()
		assert.Len(t, *openTrades, 3)

		// fully close trade 1
		tr5, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, prc, vol)
		tr5.AutoExecute()
		assert.NoError(t, err)
		trades.Add(tr5)
		openTrades = trades.OpenTrades()
		assert.Len(t, *openTrades, 2)
		assert.Equal(t, tr1, (*openTrades)[0])
		assert.Equal(t, tr3, (*openTrades)[1])

		// close the rest of trade 2
		tr6, err := NewCloseTrade(id, []*Trade{tr2}, timeframe, ts, prc, vol/2.0)
		tr6.AutoExecute()
		assert.NoError(t, err)
		trades.Add(tr6)
		openTrades = trades.OpenTrades()
		assert.Len(t, *openTrades, 1)
		assert.Equal(t, tr3, (*openTrades)[0])
	})
}

func TestProfit(t *testing.T) {
	t.Run("profitable trades", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: 1.0,
				ExecutedPrice:   1000.0,
			},
			{
				RequestedVolume: -0.5,
				ExecutedPrice:   1100.0,
			},
		})

		stats, err := trades.GetTradeStats(Tick{Bid: 1300.0, Ask: 1300.0})
		assert.NoError(t, err)

		assert.Equal(t, 50.0, stats.Realized)
		assert.Equal(t, 150.0, stats.Floating)
	})

	t.Run("losing trades", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: 1.0,
				ExecutedPrice:   1000.0,
			},
			{
				RequestedVolume: -0.5,
				ExecutedPrice:   500.0,
			},
		})

		stats, err := trades.GetTradeStats(Tick{Bid: 400.0, Ask: 400.0})
		assert.NoError(t, err)

		assert.Equal(t, -250.0, stats.Realized)
		assert.Equal(t, -300.0, stats.Floating)

		trades.Add(&Trade{
			RequestedVolume: -0.5,
			ExecutedPrice:   400.0,
		})

		stats, err = trades.GetTradeStats(Tick{Bid: 400.0, Ask: 400.0})
		assert.NoError(t, err)

		assert.Equal(t, -550.0, stats.Realized)
		assert.Equal(t, 0.0, stats.Floating)
	})

	t.Run("losing -> winning trades", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: 1.0,
				ExecutedPrice:   1000.0,
			},
			{
				RequestedVolume: -0.5,
				ExecutedPrice:   500.0,
			},
		})

		stats, err := trades.GetTradeStats(Tick{Bid: 400.0, Ask: 400.0})
		assert.NoError(t, err)

		assert.Equal(t, -250.0, stats.Realized)
		assert.Equal(t, -300.0, stats.Floating)

		trades.Add(&Trade{
			RequestedVolume: -0.3,
			ExecutedPrice:   5000.0,
		})

		stats, err = trades.GetTradeStats(Tick{Bid: 5000.0, Ask: 5000.0})
		assert.NoError(t, err)

		assert.Equal(t, -250.0+1200.0, stats.Realized)
		assert.Equal(t, 800.0, stats.Floating)
	})

	t.Run("no trades", func(t *testing.T) {
		trades := Trades([]*Trade{})
		stats, err := trades.GetTradeStats(Tick{Bid: 1000.0, Ask: 1000.0})
		assert.NoError(t, err)

		assert.Equal(t, 0.0, stats.Realized)
		assert.Equal(t, 0.0, stats.Floating)
	})

	t.Run("close an open trade", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: 1.0,
				ExecutedPrice:   1000.0,
			},
		})

		stats, err := trades.GetTradeStats(Tick{Bid: 2000.0, Ask: 2000.0})
		assert.NoError(t, err)

		assert.Equal(t, 0.0, stats.Realized)
		assert.Equal(t, 1000.0, stats.Floating)

		trades.Add(&Trade{
			RequestedVolume: -1.0,
			ExecutedPrice:   2000.0,
		})

		stats, err = trades.GetTradeStats(Tick{Bid: 2000.0, Ask: 2000.0})
		assert.NoError(t, err)

		assert.Equal(t, 1000.0, stats.Realized)
		assert.Equal(t, 0.0, stats.Floating)
	})

	t.Run("floating profit long", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: 1.0,
				ExecutedPrice:   1000.0,
			},
			{
				RequestedVolume: 1.0,
				ExecutedPrice:   2000.0,
			},
		})
		stats, err := trades.GetTradeStats(Tick{Bid: 3000.0, Ask: 3000.0})
		assert.NoError(t, err)

		assert.Equal(t, 0.0, stats.Realized)
		assert.Equal(t, 3000.0, stats.Floating)
	})

	t.Run("floating profit short", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: -1.0,
				ExecutedPrice:   1000.0,
			},
			{
				RequestedVolume: -1.0,
				ExecutedPrice:   2000.0,
			},
		})
		stats, err := trades.GetTradeStats(Tick{Bid: 3000.0, Ask: 3000.0})
		assert.NoError(t, err)

		assert.Equal(t, 0.0, stats.Realized)
		assert.Equal(t, -3000.0, stats.Floating)
	})
}

func TestVwap(t *testing.T) {
	t.Run("long and short trades", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: 1.0,
				ExecutedPrice:   1000.0,
			},
			{
				RequestedVolume: -0.5,
				ExecutedPrice:   1100.0,
			},
		})

		vwap, volume, realizedPL := trades.GetTradeStatsItems()

		assert.Equal(t, Volume(0.5), volume)
		assert.Equal(t, Vwap(1000.0), vwap)
		assert.Equal(t, RealizedPL(50.0), realizedPL)
	})

	t.Run("no trades", func(t *testing.T) {
		trades := Trades([]*Trade{})

		vwap, volume, realizedPL := trades.GetTradeStatsItems()

		assert.Equal(t, Volume(0.0), volume)
		assert.Equal(t, Vwap(0.0), vwap)
		assert.Equal(t, RealizedPL(0.0), realizedPL)
	})

	t.Run("switch volume direction: long -> short", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: 1.0,
				ExecutedPrice:   1000.0,
			},
			{
				RequestedVolume: -0.5,
				ExecutedPrice:   1100.0,
			},
			{
				RequestedVolume: -0.7,
				ExecutedPrice:   1500.0,
			},
		})

		vwap, volume, realizedPL := trades.GetTradeStatsItems()

		assert.LessOrEqual(t, Volume(-0.2)-volume, 0.01)
		assert.Equal(t, Vwap(1500.0), vwap)
		assert.Equal(t, RealizedPL(300.0), realizedPL)
	})

	t.Run("switch volume direction: short -> long", func(t *testing.T) {
		trades := Trades([]*Trade{
			{
				RequestedVolume: -1.0,
				ExecutedPrice:   1000.0,
			},
			{
				RequestedVolume: 1.7,
				ExecutedPrice:   1300.0,
			},
			{
				RequestedVolume: -0.5,
				ExecutedPrice:   1100.0,
			},
		})

		vwap, volume, realizedPL := trades.GetTradeStatsItems()

		assert.LessOrEqual(t, Volume(-0.2)-volume, 0.001)
		assert.Equal(t, Vwap(1300.0), vwap)
		assert.Equal(t, RealizedPL(-400.0), realizedPL)
	})
}
