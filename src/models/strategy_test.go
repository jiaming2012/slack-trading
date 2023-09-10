package models

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewCloseTrade(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	name := "test"
	symbol := "symbol"
	tf := 5
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	balance := 1000.0
	newPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price:             1.0,
				StopLoss:          0.5,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				StopLoss:          1.5,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}
	}

	newPriceLevels2 := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price:             1.0,
				StopLoss:          3.5,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				StopLoss:          12.5,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}
	}

	t.Run("close the entire buy trade", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Up, balance, newPriceLevels())
		assert.Nil(t, err)

		tr1, err := s.NewOpenTrade(id, tf, ts, 1.5)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		priceLevel, err := s.GetPriceLevelByIndex(0)
		assert.Nil(t, err)
		_, vol, _ := priceLevel.Trades.Vwap()
		assert.Greater(t, vol, Volume(0.0))

		tr2, err := s.NewCloseTrade(id, tf, ts, 1.8, 1.0)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr2)
		assert.Nil(t, err)

		assert.Len(t, tr2.Offsets, 1)
		assert.Equal(t, tr1, tr2.Offsets[0])

		_, vol, _ = priceLevel.Trades.Vwap()
		assert.Equal(t, Volume(0.0), vol)
	})

	t.Run("close partial buy trade", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Up, balance, newPriceLevels())
		assert.Nil(t, err)

		tr1, err := s.NewOpenTrade(id, tf, ts, 1.5)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		priceLevel, err := s.GetPriceLevelByIndex(0)
		assert.Nil(t, err)
		_, vol, _ := priceLevel.Trades.Vwap()
		assert.Greater(t, vol, Volume(0.0))

		// partial close
		tr2, err := s.NewCloseTrade(id, tf, ts, 1.8, 0.5)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr2)
		assert.Nil(t, err)

		assert.Len(t, tr2.Offsets, 1)
		assert.Equal(t, tr1, tr2.Offsets[0])

		// should still have one open trade
		openTrades := priceLevel.Trades.OpenTrades()
		assert.Len(t, *openTrades, 1)

		// close the rest of the trade
		tr3, err := s.NewCloseTrade(id, tf, ts, 1.8, 0.5)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr3)
		assert.Nil(t, err)

		openTrades = priceLevel.Trades.OpenTrades()
		assert.Len(t, *openTrades, 0)
	})

	t.Run("close the entire sell trade", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Down, balance, newPriceLevels2())
		assert.Nil(t, err)

		tr1, err := s.NewOpenTrade(id, tf, ts, 1.5)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		priceLevel, err := s.GetPriceLevelByIndex(0)
		assert.Nil(t, err)
		_, vol, _ := priceLevel.Trades.Vwap()
		assert.Less(t, vol, Volume(0.0))

		tr2, err := s.NewCloseTrade(id, tf, ts, 1.8, 1.0)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr2)
		assert.Nil(t, err)

		assert.Len(t, tr2.Offsets, 1)
		assert.Equal(t, tr1, tr2.Offsets[0])

		_, vol, _ = priceLevel.Trades.Vwap()
		assert.Equal(t, Volume(0.0), vol)
	})

	t.Run("close partial buy trade", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Down, balance, newPriceLevels2())
		assert.Nil(t, err)

		tr1, err := s.NewOpenTrade(id, tf, ts, 1.5)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr1)
		assert.Nil(t, err)

		priceLevel, err := s.GetPriceLevelByIndex(0)
		assert.Nil(t, err)
		_, vol, _ := priceLevel.Trades.Vwap()
		assert.Less(t, vol, Volume(0.0))

		// partial close
		tr2, err := s.NewCloseTrade(id, tf, ts, 1.2, 0.5)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr2)
		assert.Nil(t, err)

		assert.Len(t, tr2.Offsets, 1)
		assert.Equal(t, tr1, tr2.Offsets[0])

		// should still have one open trade
		openTrades := priceLevel.Trades.OpenTrades()
		assert.Len(t, *openTrades, 1)

		// close the rest of the trade
		tr3, err := s.NewCloseTrade(id, tf, ts, 1.8, 0.5)
		assert.Nil(t, err)
		err = s.AutoExecuteTrade(tr3)
		assert.Nil(t, err)

		openTrades = priceLevel.Trades.OpenTrades()
		assert.Len(t, *openTrades, 0)
	})
}

func TestStrategy(t *testing.T) {
	name := "test"
	symbol := "symbol"
	direction := Direction("up")
	balance := 1000.0
	newPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price:             1.0,
				StopLoss:          0.5,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				StopLoss:          1.0,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}
	}

	t.Run("strategy balance must be greater than zero", func(t *testing.T) {
		_, err := NewStrategy(name, symbol, direction, 0.0, newPriceLevels())
		assert.ErrorIs(t, err, BalanceGreaterThanZeroErr)
	})

	t.Run("the last price level must have an allocation of zero", func(t *testing.T) {
		priceLevels := []*PriceLevel{
			{
				Price:             1.0,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
		}

		_, err := NewStrategy(name, symbol, direction, balance, priceLevels)

		assert.ErrorIs(t, err, PriceLevelsLastAllocationErr)

		priceLevels = append(priceLevels, &PriceLevel{
			Price:             6.0,
			MaxNoOfTrades:     0,
			AllocationPercent: 0,
		})

		_, err = NewStrategy(name, symbol, direction, balance, priceLevels)

		assert.Nil(t, err)
	})

	t.Run("fails if no levels are set", func(t *testing.T) {
		_, err := NewStrategy(name, symbol, direction, balance, []*PriceLevel{})
		assert.ErrorIs(t, err, LevelsNotSetErr)
	})

	t.Run("errors if price levels are not sorted", func(t *testing.T) {
		_, err := NewStrategy(name, symbol, direction, balance, []*PriceLevel{
			{Price: 1.0, AllocationPercent: 1, MaxNoOfTrades: 1},
			{Price: 3.0},
			{Price: 2.0},
		})

		assert.ErrorIs(t, err, PriceLevelsNotSortedErr)
	})

	t.Run("num of trade > 0 if allocation is > 0", func(t *testing.T) {
		_priceLevels := []*PriceLevel{
			{
				Price:             1.0,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				AllocationPercent: 0.5,
			},
			{
				Price: 3.0,
			},
		}

		_, err := NewStrategy(name, symbol, direction, balance, _priceLevels)
		assert.ErrorIs(t, err, NoOfTradeMustBeNonzeroErr)
	})

	t.Run("num of trades must be zero if allocation is zero", func(t *testing.T) {
		_priceLevels1 := []*PriceLevel{
			{
				Price:             1.0,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				AllocationPercent: 0,
			},
			{
				Price:             3.0,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}

		_, err := NewStrategy(name, symbol, direction, balance, _priceLevels1)
		assert.Nil(t, err)

		_priceLevels2 := []*PriceLevel{
			{
				Price:             1.0,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				MaxNoOfTrades:     3,
				AllocationPercent: 0,
			},
			{
				Price:             3.0,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}

		_, err = NewStrategy(name, symbol, direction, balance, _priceLevels2)
		assert.ErrorIs(t, err, NoOfTradesMustBeZeroErr)
	})
}
