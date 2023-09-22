package models

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestRealizedPLForTradeTypeSell(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	openPrc := 4.0
	sl := 6.0
	timeframe := 5
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)
	side := TradeTypeSell

	t.Run("realized pl is zero if no trade is opened", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)
		assert.Equal(t, 0.0, tr1.RealizedPL())
	})

	t.Run("realized pl realized once trade is closed for buy trade", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 3.0, 0.5)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)
		assert.Equal(t, 0.5, tr1.RealizedPL())
	})

	t.Run("realized pl for partial closes", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 3.0, 0.6)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)
		assert.Equal(t, 0.6, tr1.RealizedPL())

		tr3, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 2.0, 0.2)
		assert.NoError(t, err)
		tr3.AutoExecute(nil)
		assert.Equal(t, 1.0, tr1.RealizedPL())

		tr4, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 2.0, 0.2)
		assert.NoError(t, err)
		tr4.AutoExecute(nil)
		assert.Equal(t, 1.4, tr1.RealizedPL())
	})

	t.Run("realized pl for losing trade", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 5.5, 0.1)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)
		assert.InEpsilon(t, -0.15, tr1.RealizedPL(), 0.001)
	})
}

func TestRealizedPLForTradeTypeBuy(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	openPrc := 2.0
	sl := 1.8
	timeframe := 5
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)
	side := TradeTypeBuy

	t.Run("realized pl is zero if no trade is opened", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)
		assert.Equal(t, 0.0, tr1.RealizedPL())
	})

	t.Run("realized pl realized once trade is closed for buy trade", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 3.0, 0.5)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)
		assert.Equal(t, 0.5, tr1.RealizedPL())
	})

	t.Run("realized pl for partial closes", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 3.0, 0.6)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)
		assert.Equal(t, 0.6, tr1.RealizedPL())

		tr3, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 4.0, 0.2)
		assert.NoError(t, err)
		tr3.AutoExecute(nil)
		assert.Equal(t, 1.0, tr1.RealizedPL())

		tr4, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 4.0, 0.2)
		assert.NoError(t, err)
		tr4.AutoExecute(nil)
		assert.Equal(t, 1.4, tr1.RealizedPL())
	})

	t.Run("realized pl for losing trade", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 0.5, 0.1)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)
		assert.InEpsilon(t, -0.15, tr1.RealizedPL(), 0.001)
	})
}

func TestTradeClose(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	prc := 2.0
	sl := 1.8
	closePrc := 3.0
	timeframe := 5
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)

	t.Run("closing trade MUST have an offsetting trade", func(t *testing.T) {
		_, err := NewCloseTrade(id, []*Trade{}, timeframe, ts, closePrc, 0.8)
		assert.ErrorIs(t, err, NoOffsettingTradeErr)
	})

	t.Run("offsetting trades volume can be less than closing trade volume", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)

		_, err = NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, closePrc, 0.9)
		assert.NoError(t, err)
	})

	t.Run("offsetting trades volume can be equal to closing trade volume", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)

		_, err = NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, closePrc, 1.0)
		assert.NoError(t, err)
	})

	t.Run("case 1: each offsetting trade volume MUST combine to cover the closing trades volume", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)

		tr2, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)

		_, err = NewCloseTrade(id, []*Trade{tr1, tr2}, timeframe, ts, closePrc, 0.8)
		assert.ErrorIs(t, err, OffsetTradesVolumeExceedsClosingTradeVolumeErr)
	})

	t.Run("case 2: each offsetting trade volume MUST combine to cover the closing trades volume", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)

		tr2, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)

		tr3, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl)
		assert.NoError(t, err)
		tr3.AutoExecute(nil)

		_, err = NewCloseTrade(id, []*Trade{tr1, tr2, tr3}, timeframe, ts, closePrc, 0.8)
		assert.ErrorIs(t, err, OffsetTradesVolumeExceedsClosingTradeVolumeErr)
	})
}

func TestMaxRisk(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	timestamp := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)
	symbol := "symbol"
	tf := 5
	reqPrice := 1000.0
	sl := 750.0
	reqVol := 2.0

	t.Run("max risk is zero when no trades are open", func(t *testing.T) {
		trades := &Trades{}
		maxRisk, realizedPL := trades.MaxRisk(sl)
		assert.Equal(t, 0.0, maxRisk)
		assert.Equal(t, RealizedPL(0.0), realizedPL)
	})

	t.Run("max risk with one of three open trades", func(t *testing.T) {
		tr, err := NewOpenTrade(id, TradeTypeBuy, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.NoError(t, err)
		tr.AutoExecute(nil)

		trades := Trades{}
		trades.Add(tr)

		maxRisk, realizedPL := trades.MaxRisk(sl)
		assert.Equal(t, 500.0, maxRisk)
		assert.Equal(t, RealizedPL(0.0), realizedPL)
	})

	t.Run("max risk with two of three open trades", func(t *testing.T) {
		sl := 1500.0

		tr1, err := NewOpenTrade(id, TradeTypeSell, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)

		tr2, err := NewOpenTrade(id, TradeTypeSell, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)

		trades := Trades{}
		trades.Add(tr1)
		trades.Add(tr2)

		maxRisk, realizedPL := trades.MaxRisk(sl)
		assert.Equal(t, 2000.0, maxRisk)
		assert.Equal(t, RealizedPL(0.0), realizedPL)
	})

	t.Run("max risk decreases when trade closed at profit", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.NoError(t, err)
		tr1.AutoExecute(nil)

		tr2, err := NewOpenTrade(id, TradeTypeBuy, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.NoError(t, err)
		tr2.AutoExecute(nil)

		trades := Trades{}
		trades.Add(tr1)
		trades.Add(tr2)

		maxRisk, realizedPL := trades.MaxRisk(sl)
		assert.Equal(t, 1000.0, maxRisk)
		assert.Equal(t, RealizedPL(0.0), realizedPL)

		clsTrade, err := NewCloseTrade(id, []*Trade{tr1}, tf, timestamp, reqPrice+500.0, reqVol)
		paritalCloseItemRequests := []*PartialCloseItemRequest{
			{
				Trade: tr1,
				PartialCloseItem: &PartialCloseItem{
					ClosedBy: clsTrade,
					Volume:   tr1.ExecutedVolume * -1,
					Price:    reqPrice + 500.0,
				},
			},
		}
		assert.NoError(t, err)
		clsTrade.AutoExecute(paritalCloseItemRequests)
		trades.Add(clsTrade)

		maxRisk, realizedPL = trades.MaxRisk(sl)
		assert.Equal(t, RealizedPL(1000.0), realizedPL)
		assert.Equal(t, 500.0, maxRisk)
	})
}

func TestTrade(t *testing.T) {
	t.Run("trade side", func(t *testing.T) {
		tr := Trade{RequestedVolume: 1.0}
		assert.Equal(t, TradeTypeBuy, tr.Side())

		tr = Trade{RequestedVolume: -1.0}
		assert.Equal(t, TradeTypeSell, tr.Side())
	})
}
