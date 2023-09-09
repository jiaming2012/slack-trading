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
	symbol := "symbol"
	ts := time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)

	t.Run("closing trade MUST have an offsetting trade", func(t *testing.T) {
		_, err := NewTradeClose(id, []*Trade{}, ts, closePrc, 0.8)
		assert.ErrorIs(t, err, NoOffsettingTradeErr)
	})

	t.Run("offsetting trades volume can be less than closing trade volume", func(t *testing.T) {
		tr1, err := NewTradeOpen(id, TradeTypeBuy, symbol, ts, prc, 1.0, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		_, err = NewTradeClose(id, []*Trade{tr1}, ts, closePrc, 0.9)
		assert.Nil(t, err)
	})

	t.Run("offsetting trades volume can be equal to closing trade volume", func(t *testing.T) {
		tr1, err := NewTradeOpen(id, TradeTypeBuy, symbol, ts, prc, 1.0, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		_, err = NewTradeClose(id, []*Trade{tr1}, ts, closePrc, 1.0)
		assert.Nil(t, err)
	})

	t.Run("case 1: each offsetting trade volume MUST combine to cover the closing trades volume", func(t *testing.T) {
		tr1, err := NewTradeOpen(id, TradeTypeBuy, symbol, ts, prc, 1.0, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		tr2, err := NewTradeOpen(id, TradeTypeBuy, symbol, ts, prc, 0.5, sl)
		assert.Nil(t, err)
		tr2.AutoExecute()

		_, err = NewTradeClose(id, []*Trade{tr1, tr2}, ts, closePrc, 0.8)
		assert.ErrorIs(t, err, OffsetTradesVolumeExceedsClosingTradeVolumeErr)
	})

	t.Run("case 2: each offsetting trade volume MUST combine to cover the closing trades volume", func(t *testing.T) {
		tr1, err := NewTradeOpen(id, TradeTypeBuy, symbol, ts, prc, 0.5, sl)
		assert.Nil(t, err)
		tr1.AutoExecute()

		tr2, err := NewTradeOpen(id, TradeTypeBuy, symbol, ts, prc, 0.5, sl)
		assert.Nil(t, err)
		tr2.AutoExecute()

		tr3, err := NewTradeOpen(id, TradeTypeBuy, symbol, ts, prc, 0.5, sl)
		assert.Nil(t, err)
		tr3.AutoExecute()

		_, err = NewTradeClose(id, []*Trade{tr1, tr2, tr3}, ts, closePrc, 0.8)
		assert.ErrorIs(t, err, OffsetTradesVolumeExceedsClosingTradeVolumeErr)
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
