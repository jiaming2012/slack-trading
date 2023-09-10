package models

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

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
		assert.Nil(t, err)
		tr1.AutoExecute()

		_, err = NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, closePrc, 0.9)
		assert.Nil(t, err)
	})

	t.Run("offsetting trades volume can be equal to closing trade volume", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		_, err = NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, closePrc, 1.0)
		assert.Nil(t, err)
	})

	t.Run("case 1: each offsetting trade volume MUST combine to cover the closing trades volume", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		tr2, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl)
		assert.Nil(t, err)
		tr2.AutoExecute()

		_, err = NewCloseTrade(id, []*Trade{tr1, tr2}, timeframe, ts, closePrc, 0.8)
		assert.ErrorIs(t, err, OffsetTradesVolumeExceedsClosingTradeVolumeErr)
	})

	t.Run("case 2: each offsetting trade volume MUST combine to cover the closing trades volume", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		tr2, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl)
		assert.Nil(t, err)
		tr2.AutoExecute()

		tr3, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl)
		assert.Nil(t, err)
		tr3.AutoExecute()

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
		assert.Nil(t, err)
		tr.AutoExecute()

		trades := Trades{}
		trades.Add(tr)

		maxRisk, realizedPL := trades.MaxRisk(sl)
		assert.Equal(t, 500.0, maxRisk)
		assert.Equal(t, RealizedPL(0.0), realizedPL)
	})

	t.Run("max risk with two of three open trades", func(t *testing.T) {
		sl := 1500.0

		tr1, err := NewOpenTrade(id, TradeTypeSell, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		tr2, err := NewOpenTrade(id, TradeTypeSell, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.Nil(t, err)
		tr2.AutoExecute()

		trades := Trades{}
		trades.Add(tr1)
		trades.Add(tr2)

		maxRisk, realizedPL := trades.MaxRisk(sl)
		assert.Equal(t, 2000.0, maxRisk)
		assert.Equal(t, RealizedPL(0.0), realizedPL)
	})

	t.Run("max risk decreases when trade closed at profit", func(t *testing.T) {
		tr1, err := NewOpenTrade(id, TradeTypeBuy, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		tr2, err := NewOpenTrade(id, TradeTypeBuy, symbol, tf, timestamp, reqPrice, reqVol, sl)
		assert.Nil(t, err)
		tr2.AutoExecute()

		trades := Trades{}
		trades.Add(tr1)
		trades.Add(tr2)

		maxRisk, realizedPL := trades.MaxRisk(sl)
		assert.Equal(t, 1000.0, maxRisk)
		assert.Equal(t, RealizedPL(0.0), realizedPL)

		clsTrade, err := NewCloseTrade(id, []*Trade{tr1}, tf, timestamp, reqPrice+500.0, reqVol)
		assert.Nil(t, err)
		clsTrade.AutoExecute()
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

	t.Run("validate volume is non zero", func(t *testing.T) {
		tr := Trade{RequestedVolume: 0.0, StopLoss: 1.5}
		err := tr.Validate()
		assert.ErrorIs(t, err, TradeVolumeIsZeroErr)
	})

	t.Run("validate stop loss", func(t *testing.T) {
		t.Run("passes validation", func(t *testing.T) {
			tr := Trade{RequestedVolume: 1.0, RequestedPrice: 1.0, StopLoss: 0.5}
			err := tr.Validate()
			assert.Nil(t, err)
		})

		t.Run("errors if trade has no stop loss", func(t *testing.T) {
			tr := Trade{RequestedVolume: 1.0}
			err := tr.Validate()
			assert.ErrorIs(t, err, NoStopLossErr)
		})

		t.Run("errors if buy order has stop loss above current price", func(t *testing.T) {
			tr := Trade{RequestedVolume: 1.0, RequestedPrice: 1.0, StopLoss: 1.5}
			err := tr.Validate()
			assert.ErrorContains(t, err, InvalidStopLossErr.Error())
		})

		t.Run("errors if sell order has stop loss below current price", func(t *testing.T) {
			tr := Trade{RequestedVolume: -1.0, RequestedPrice: 1.0, StopLoss: 0.5}
			err := tr.Validate()
			assert.ErrorContains(t, err, InvalidStopLossErr.Error())
		})
	})
}
