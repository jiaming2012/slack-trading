package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestSymbol(t *testing.T) {
	t.Run("returns the symbol", func(t *testing.T) {
		symbol := eventmodels.StockSymbol("AAPL")

		repo := NewBacktesterCandleRepository(symbol, nil)

		assert.Equal(t, symbol, repo.GetSymbol())
	})
}

func TestNext(t *testing.T) {
	symbol := eventmodels.StockSymbol("AAPL")

	candles := []*eventmodels.PolygonAggregateBarV2{
		{
			Timestamp: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Timestamp: time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC),
		},
		{
			Timestamp: time.Date(2021, 1, 1, 0, 2, 0, 0, time.UTC),
		},
	}

	t.Run("returns the current candle", func(t *testing.T) {
		repo := NewBacktesterCandleRepository(symbol, candles)

		tstamp := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		candle := repo.GetCurrentCandle()

		assert.Equal(t, tstamp, candle.Timestamp)
	})

	t.Run("returns the next candle", func(t *testing.T) {
		repo := NewBacktesterCandleRepository(symbol, candles)

		tstamp := time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)

		c, err := repo.Update(tstamp)

		assert.NoError(t, err)

		assert.NotNil(t, c)

		candle := repo.GetCurrentCandle()

		assert.Equal(t, tstamp, candle.Timestamp)
	})

	t.Run("returns last candle if there are no more candles", func(t *testing.T) {
		repo := NewBacktesterCandleRepository(symbol, candles)

		tstamp := time.Date(2021, 1, 1, 0, 3, 0, 0, time.UTC)

		_, err := repo.Update(tstamp)

		assert.NoError(t, err)

		candle := repo.GetCurrentCandle()

		assert.Equal(t, candles[len(candles)-1].Timestamp, candle.Timestamp)
	})
}
