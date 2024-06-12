package eventmodels

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestTrade_IsStopLossTriggered(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	openPrc := 4.0
	timeframe := new(int)
	*timeframe = 5
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)

	t.Run("buy stop loss triggered", func(t *testing.T) {
		side := TradeTypeBuy
		sl := 2.0
		tr, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)

		// should not trigger when Bid > sl
		closeTradeReq, err := tr.IsStopLossTriggered(Tick{Price: 2.1})
		assert.NoError(t, err)
		assert.Nil(t, closeTradeReq)

		// trigger
		closeTradeReq, err = tr.IsStopLossTriggered(Tick{Price: 2.0})
		assert.NoError(t, err)
		assert.Equal(t, tr, closeTradeReq.Trade)
		assert.Nil(t, closeTradeReq.Timeframe)
		assert.Equal(t, 1.0, closeTradeReq.Percent)
		assert.Equal(t, "sl", closeTradeReq.Reason)
	})

	t.Run("sell stop loss triggered", func(t *testing.T) {
		side := TradeTypeSell
		sl := 7.0
		tr, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)

		// should not trigger when Ask < sl
		closeTradeReq, err := tr.IsStopLossTriggered(Tick{Price: 6.98})
		assert.NoError(t, err)
		assert.Nil(t, closeTradeReq)

		// trigger
		closeTradeReq, err = tr.IsStopLossTriggered(Tick{Price: 7.5})
		assert.NoError(t, err)
		assert.Equal(t, tr, closeTradeReq.Trade)
		assert.Nil(t, closeTradeReq.Timeframe)
		assert.Equal(t, 1.0, closeTradeReq.Percent)
		assert.Equal(t, "sl", closeTradeReq.Reason)
	})
}

func TestRealizedPLForTradeTypeSell(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	openPrc := 4.0
	sl := 6.0
	timeframe := new(int)
	*timeframe = 5
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)
	side := TradeTypeSell

	t.Run("realized pl is zero if no trade is opened", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Equal(t, 0.0, tr1.RealizedPL())
	})

	t.Run("realized pl realized once trade is closed for buy trade", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 3.0, 0.5, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()
		assert.Equal(t, 0.5, tr1.RealizedPL())
	})

	t.Run("realized pl for partial closes", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 3.0, 0.6, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()
		assert.Equal(t, 0.6, tr1.RealizedPL())

		tr3, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 2.0, 0.2, nil)
		assert.NoError(t, err)
		tr3.AutoExecute()
		assert.Equal(t, 1.0, tr1.RealizedPL())

		tr4, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 2.0, 0.2, nil)
		assert.NoError(t, err)
		tr4.AutoExecute()
		assert.Equal(t, 1.4, tr1.RealizedPL())
	})

	t.Run("realized pl for losing trade", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 5.5, 0.1, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()
		assert.InEpsilon(t, -0.15, tr1.RealizedPL(), 0.001)
	})
}

func TestRealizedPLForTradeTypeBuy(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	openPrc := 2.0
	sl := 1.8
	timeframe := new(int)
	*timeframe = 5
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)
	side := TradeTypeBuy

	t.Run("realized pl is zero if no trade is opened", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Equal(t, 0.0, tr1.RealizedPL())
	})

	t.Run("realized pl realized once trade is closed for buy trade", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 3.0, 0.5, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()
		assert.Equal(t, 0.5, tr1.RealizedPL())
	})

	t.Run("realized pl for partial closes", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 3.0, 0.6, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()
		assert.Equal(t, 0.6, tr1.RealizedPL())

		tr3, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 4.0, 0.2, nil)
		assert.NoError(t, err)
		tr3.AutoExecute()
		assert.Equal(t, 1.0, tr1.RealizedPL())

		tr4, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 4.0, 0.2, nil)
		assert.NoError(t, err)
		tr4.AutoExecute()
		assert.Equal(t, 1.4, tr1.RealizedPL())
	})

	t.Run("realized pl for losing trade", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, side, symbol, timeframe, ts, openPrc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Equal(t, 0.0, tr1.RealizedPL())

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, 0.5, 0.1, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()
		assert.InEpsilon(t, -0.15, tr1.RealizedPL(), 0.001)
	})
}

func TestTradeClose(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	prc := 2.0
	sl := 1.8
	closePrc := 3.0
	timeframe := new(int)
	*timeframe = 5
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)

	t.Run("test calculate profit", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 2.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Nil(t, tr1.ConvertToTradeDTO().ProfitLoss)

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, closePrc, 2.0, nil)
		assert.Nil(t, err)
		tr2.AutoExecute()

		pl := tr1.ConvertToTradeDTO().ProfitLoss
		assert.NotNil(t, pl)

		assert.Equal(t, 2.0, *pl)
	})

	t.Run("test calculate loss", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, TradeTypeSell, symbol, timeframe, ts, prc, 2.0, prc+sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()
		assert.Nil(t, tr1.ConvertToTradeDTO().ProfitLoss)

		tr2, _, err := NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, closePrc, 2.0, nil)
		assert.Nil(t, err)
		tr2.AutoExecute()

		pl := tr1.ConvertToTradeDTO().ProfitLoss
		assert.NotNil(t, pl)

		assert.Equal(t, -2.0, *pl)
	})

	t.Run("closing trade MUST have an offsetting trade", func(t *testing.T) {
		_, _, err := NewCloseTrade(id, []*Trade{}, timeframe, ts, closePrc, 0.8, nil)
		assert.ErrorIs(t, err, NoOffsettingTradeErr)
	})

	t.Run("offsetting trades volume can be less than closing trade volume", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()

		_, _, err = NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, closePrc, 0.9, nil)
		assert.NoError(t, err)
	})

	t.Run("offsetting trades volume can be equal to closing trade volume", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()

		_, _, err = NewCloseTrade(id, []*Trade{tr1}, timeframe, ts, closePrc, 1.0, nil)
		assert.NoError(t, err)
	})

	t.Run("case 1: each offsetting trade volume MUST combine to cover the closing trades volume", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 1.0, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()

		tr2, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()

		_, _, err = NewCloseTrade(id, []*Trade{tr1, tr2}, timeframe, ts, closePrc, 0.8, nil)
		assert.ErrorIs(t, err, OffsetTradesVolumeExceedsClosingTradeVolumeErr)
	})

	t.Run("case 2: each offsetting trade volume MUST combine to cover the closing trades volume", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()

		tr2, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()

		tr3, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, timeframe, ts, prc, 0.5, sl, nil)
		assert.NoError(t, err)
		tr3.AutoExecute()

		_, _, err = NewCloseTrade(id, []*Trade{tr1, tr2, tr3}, timeframe, ts, closePrc, 0.8, nil)
		assert.ErrorIs(t, err, OffsetTradesVolumeExceedsClosingTradeVolumeErr)
	})
}

func TestMaxRisk(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	timestamp := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)
	symbol := "symbol"
	tf := new(int)
	*tf = 5
	reqPrice := 1000.0
	sl := 750.0
	reqVol := 2.0

	t.Run("max risk is zero when no trades are open", func(t *testing.T) {
		trades := &Trades{}
		maxRisk := trades.CurrentRisk(sl)
		assert.Equal(t, 0.0, maxRisk)
	})

	t.Run("max risk with one of three open trades", func(t *testing.T) {
		tr, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, tf, timestamp, reqPrice, reqVol, sl, nil)
		assert.NoError(t, err)
		tr.AutoExecute()

		trades := Trades{}
		trades.Add(tr)

		maxRisk := trades.CurrentRisk(sl)
		assert.Equal(t, 500.0, maxRisk)
	})

	t.Run("max risk with two of three open trades", func(t *testing.T) {
		sl := 1500.0

		tr1, _, err := NewOpenTrade(id, TradeTypeSell, symbol, tf, timestamp, reqPrice, reqVol, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()

		tr2, _, err := NewOpenTrade(id, TradeTypeSell, symbol, tf, timestamp, reqPrice, reqVol, sl, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()

		trades := Trades{}
		trades.Add(tr1)
		trades.Add(tr2)

		maxRisk := trades.CurrentRisk(sl)
		assert.Equal(t, 2000.0, maxRisk)
	})

	t.Run("max risk decreases when trade closed at profit", func(t *testing.T) {
		tr1, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, tf, timestamp, reqPrice, reqVol, sl, nil)
		assert.NoError(t, err)
		tr1.AutoExecute()

		tr2, _, err := NewOpenTrade(id, TradeTypeBuy, symbol, tf, timestamp, reqPrice, reqVol, sl, nil)
		assert.NoError(t, err)
		tr2.AutoExecute()

		trades := Trades{}
		trades.Add(tr1)
		trades.Add(tr2)

		maxLoss := trades.CurrentRisk(sl)
		assert.Equal(t, 1000.0, maxLoss)

		clsTrade, _, err := NewCloseTrade(id, []*Trade{tr1}, tf, timestamp, reqPrice+500.0, reqVol, nil)
		assert.NoError(t, err)
		clsTrade.AutoExecute()
		trades.Add(clsTrade)

		maxLoss = trades.CurrentRisk(sl)
		assert.Equal(t, 500.0, maxLoss)
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
