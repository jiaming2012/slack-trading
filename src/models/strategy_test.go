package models

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestEntryConditionsSatisfied(t *testing.T) {
	name := "test"
	symbol := "symbol"
	balance := 1000.0
	newUpPriceLevels := func() []*PriceLevel {
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

	entrySignal := SignalV2{Name: "entrySignal", IsSatisfied: false}
	exitSignal := SignalV2{Name: "exitSignal", IsSatisfied: false}

	t.Run("entry conditions not satisfied if strategy has no entry conditions", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Up, balance, newUpPriceLevels())
		assert.NoError(t, err)
		assert.False(t, s.EntryConditionsSatisfied())
	})

	t.Run("entry conditions are not satisfied", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Up, balance, newUpPriceLevels())
		assert.NoError(t, err)
		err = s.AddCondition(entrySignal, exitSignal)
		assert.NoError(t, err)
		assert.Len(t, s.Conditions, 1)
		assert.Equal(t, entrySignal.Name, s.Conditions[0].EntrySignal.Name)
		assert.Equal(t, exitSignal.Name, s.Conditions[0].ExitSignal.Name)
		assert.False(t, s.EntryConditionsSatisfied())
	})

	t.Run("entry conditions are satisfied", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Up, balance, newUpPriceLevels())
		assert.NoError(t, err)
		err = s.AddCondition(entrySignal, exitSignal)
		assert.NoError(t, err)
		assert.Len(t, s.Conditions, 1)
		assert.Equal(t, entrySignal.Name, s.Conditions[0].EntrySignal.Name)
		assert.Equal(t, exitSignal.Name, s.Conditions[0].ExitSignal.Name)
		assert.False(t, s.EntryConditionsSatisfied())

		//s.GetPriceLevelByPrice()
		assert.Fail(t, "finish the test")
	})
}

func TestNewCloseTrade(t *testing.T) {
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	name := "test"
	symbol := "symbol"
	tf := 5
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	balance := 1000.0
	newUpPriceLevels := func() []*PriceLevel {
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

	newDownPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price: 1.0,
			},
			{
				Price:             2.0,
				StopLoss:          12.5,
				MaxNoOfTrades:     1,
				AllocationPercent: 0.5,
			},
			{
				Price:             10.0,
				StopLoss:          13.5,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
		}
	}

	t.Run("close the entire buy trade", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Up, balance, newUpPriceLevels())
		assert.NoError(t, err)

		tr1, err := s.NewOpenTrade(id, tf, ts, 1.5)
		assert.NoError(t, err)
		t1Result, err := s.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		priceLevel, err := s.GetPriceLevelByIndex(0)
		assert.NoError(t, err)
		_, vol, _ := priceLevel.Trades.GetTradeStatsItems()
		assert.Greater(t, vol, Volume(0.0))

		tr2, err := s.NewCloseTrades(id, tf, ts, 1.8, t1Result.PriceLevelIndex, 1.0)
		assert.NoError(t, err)
		_, err = s.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		assert.Len(t, tr2.Offsets, 1)
		assert.Equal(t, tr1, tr2.Offsets[0])

		_, vol, _ = priceLevel.Trades.GetTradeStatsItems()
		assert.Equal(t, Volume(0.0), vol)
	})

	t.Run("close partial buy trade", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Up, balance, newUpPriceLevels())
		assert.NoError(t, err)

		tr1, err := s.NewOpenTrade(id, tf, ts, 1.5)
		assert.NoError(t, err)
		t1Result, err := s.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		priceLevel, err := s.GetPriceLevelByIndex(0)
		assert.NoError(t, err)
		_, vol, _ := priceLevel.Trades.GetTradeStatsItems()
		assert.Greater(t, vol, Volume(0.0))

		// partial close
		tr2, err := s.NewCloseTrades(id, tf, ts, 1.8, t1Result.PriceLevelIndex, 0.5)
		assert.NoError(t, err)
		_, err = s.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		assert.Len(t, tr2.Offsets, 1)
		assert.Equal(t, tr1, tr2.Offsets[0])

		// should still have one open trade
		openTrades := priceLevel.Trades.OpenTrades()
		assert.Len(t, *openTrades, 1)

		// close the rest of the trade
		tr3, err := s.NewCloseTrades(id, tf, ts, 1.8, t1Result.PriceLevelIndex, 0.5)
		assert.NoError(t, err)
		_, err = s.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		openTrades = priceLevel.Trades.OpenTrades()
		assert.Len(t, *openTrades, 0)
	})

	t.Run("close the entire sell trade", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Down, balance, newDownPriceLevels())
		assert.NoError(t, err)

		tr1, err := s.NewOpenTrade(id, tf, ts, 2.5)
		assert.NoError(t, err)
		t1Result, err := s.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		priceLevel, err := s.GetPriceLevelByIndex(t1Result.PriceLevelIndex)
		assert.NoError(t, err)

		_, vol, _ := priceLevel.Trades.GetTradeStatsItems()
		assert.Less(t, vol, Volume(0.0))

		tr2, err := s.NewCloseTrades(id, tf, ts, 1.8, t1Result.PriceLevelIndex, 1.0)
		assert.NoError(t, err)
		t2Result, err := s.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		assert.Len(t, tr2.Offsets, 1)
		assert.Equal(t, tr1, tr2.Offsets[0])

		assert.NoError(t, err)
		assert.Equal(t, t2Result.PriceLevelIndex, t1Result.PriceLevelIndex)
		_, vol, _ = priceLevel.Trades.GetTradeStatsItems()
		assert.Equal(t, Volume(0.0), vol)
	})

	t.Run("close partial buy trade", func(t *testing.T) {
		s, err := NewStrategy(name, symbol, Down, balance, newDownPriceLevels())
		assert.NoError(t, err)

		tr1, err := s.NewOpenTrade(id, tf, ts, 1.5)
		assert.NoError(t, err)
		t1Result, err := s.AutoExecuteTrade(tr1)
		assert.NoError(t, err)

		priceLevel, err := s.GetPriceLevelByIndex(t1Result.PriceLevelIndex)
		assert.NoError(t, err)
		_, vol, _ := priceLevel.Trades.GetTradeStatsItems()
		assert.Less(t, vol, Volume(0.0))

		// partial close
		tr2, err := s.NewCloseTrades(id, tf, ts, 1.2, t1Result.PriceLevelIndex, 0.5)
		assert.NoError(t, err)
		_, err = s.AutoExecuteTrade(tr2)
		assert.NoError(t, err)

		assert.Len(t, tr2.Offsets, 1)
		assert.Equal(t, tr1, tr2.Offsets[0])

		// should still have one open trade
		openTrades := priceLevel.Trades.OpenTrades()
		assert.Len(t, *openTrades, 1)

		// close the rest of the trade
		tr3, err := s.NewCloseTrades(id, tf, ts, 1.8, t1Result.PriceLevelIndex, 0.5)
		assert.NoError(t, err)
		_, err = s.AutoExecuteTrade(tr3)
		assert.NoError(t, err)

		openTrades = priceLevel.Trades.OpenTrades()
		assert.Len(t, *openTrades, 0)
	})
}

func TestUpStrategy(t *testing.T) {
	name := "test"
	symbol := "symbol"
	direction := Up
	balance := 1000.0
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	tf := 5
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	newPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price:                1.0,
				StopLoss:             0.5,
				MaxNoOfTrades:        3,
				AllocationPercent:    0.5,
				MinimumTradeDistance: 0.0,
			},
			{
				Price:                2.0,
				StopLoss:             1.5,
				MaxNoOfTrades:        3,
				AllocationPercent:    0.5,
				MinimumTradeDistance: 0.1,
			},
			{
				Price:             10.0,
				MaxNoOfTrades:     0,
				AllocationPercent: 0,
			},
		}
	}

	t.Run("second trade is with minimum trade distance", func(t *testing.T) {
		strategy, err := NewStrategy(name, symbol, direction, balance, newPriceLevels())
		assert.NoError(t, err)

		t1, err := strategy.NewOpenTrade(id, tf, ts, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t1)
		assert.NoError(t, err)

		t2, err := strategy.NewOpenTrade(id, tf, ts, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t2)
		assert.NoError(t, err)
	})

	t.Run("second trade is not with minimum trade distance", func(t *testing.T) {
		strategy, err := NewStrategy(name, symbol, direction, balance, newPriceLevels())
		assert.NoError(t, err)

		t1, err := strategy.NewOpenTrade(id, tf, ts, 2.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t1)
		assert.NoError(t, err)

		t2, err := strategy.NewOpenTrade(id, tf, ts, 2.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t2)
		assert.ErrorIs(t, err, PriceLevelMinimumDistanceNotSatisfiedError)

		t3, err := strategy.NewOpenTrade(id, tf, ts, 2.09)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t3)
		assert.ErrorIs(t, err, PriceLevelMinimumDistanceNotSatisfiedError)

		t4, err := strategy.NewOpenTrade(id, tf, ts, 2.1)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t4)
		assert.NoError(t, err)
	})
}

func TestDownStrategy(t *testing.T) {
	name := "test"
	symbol := "symbol"
	direction := Down
	balance := 1000.0
	id := uuid.MustParse("69359037-9599-48e7-b8f2-48393c019135")
	tf := 5
	ts := time.Date(2023, 01, 01, 12, 0, 0, 0, time.UTC)
	newDownPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
			{
				Price: 1.0,
			},
			{
				Price:                2.0,
				StopLoss:             2.5,
				MaxNoOfTrades:        3,
				AllocationPercent:    0.5,
				MinimumTradeDistance: 0,
			},
			{
				Price:                10.0,
				StopLoss:             11.5,
				MaxNoOfTrades:        3,
				AllocationPercent:    0.5,
				MinimumTradeDistance: 0.1,
			},
		}
	}

	t.Run("second trade is with minimum trade distance", func(t *testing.T) {
		strategy, err := NewStrategy(name, symbol, direction, balance, newDownPriceLevels())
		assert.NoError(t, err)

		t1, err := strategy.NewOpenTrade(id, tf, ts, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t1)
		assert.NoError(t, err)

		t2, err := strategy.NewOpenTrade(id, tf, ts, 1.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t2)
		assert.NoError(t, err)
	})

	t.Run("second trade is not with minimum trade distance", func(t *testing.T) {
		strategy, err := NewStrategy(name, symbol, direction, balance, newDownPriceLevels())
		assert.NoError(t, err)

		t1, err := strategy.NewOpenTrade(id, tf, ts, 9.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t1)
		assert.NoError(t, err)

		t2, err := strategy.NewOpenTrade(id, tf, ts, 9.0)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t2)
		assert.ErrorIs(t, err, PriceLevelMinimumDistanceNotSatisfiedError)

		t3, err := strategy.NewOpenTrade(id, tf, ts, 9.09)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t3)
		assert.ErrorIs(t, err, PriceLevelMinimumDistanceNotSatisfiedError)

		t4, err := strategy.NewOpenTrade(id, tf, ts, 9.1)
		assert.NoError(t, err)
		_, err = strategy.AutoExecuteTrade(t4)
		assert.NoError(t, err)
	})
}

func TestStrategy(t *testing.T) {
	name := "test"
	symbol := "symbol"
	direction := Up
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
		}

		_, err := NewStrategy(name, symbol, direction, balance, priceLevels)

		assert.ErrorIs(t, err, PriceLevelsLastAllocationErr)

		priceLevels = append(priceLevels, &PriceLevel{
			Price:             6.0,
			MaxNoOfTrades:     0,
			AllocationPercent: 0,
		})

		_, err = NewStrategy(name, symbol, direction, balance, priceLevels)

		assert.NoError(t, err)
	})

	t.Run("fails if no levels are set", func(t *testing.T) {
		_, err := NewStrategy(name, symbol, direction, balance, []*PriceLevel{})
		assert.ErrorIs(t, err, MinimumNumberOfPriceLevelsNotMetErr)
	})

	t.Run("errors if price levels are not sorted", func(t *testing.T) {
		_, err := NewStrategy(name, symbol, direction, balance, []*PriceLevel{
			{Price: 1.0, StopLoss: 0.5, AllocationPercent: 1, MaxNoOfTrades: 1},
			{Price: 3.0, StopLoss: 2.5},
			{Price: 2.0, StopLoss: 1.8},
		})

		assert.ErrorIs(t, err, PriceLevelsNotSortedErr)
	})

	t.Run("num of trade > 0 if allocation is > 0", func(t *testing.T) {
		_priceLevels := []*PriceLevel{
			{
				Price:             1.0,
				StopLoss:          0.5,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				StopLoss:          1.5,
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
				StopLoss:          0.5,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				StopLoss:          1.8,
				AllocationPercent: 0,
			},
			{
				Price:             3.0,
				StopLoss:          2.0,
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
		assert.NoError(t, err)

		_priceLevels2 := []*PriceLevel{
			{
				Price:             1.0,
				StopLoss:          0.5,
				MaxNoOfTrades:     3,
				AllocationPercent: 0.5,
			},
			{
				Price:             2.0,
				StopLoss:          1.8,
				MaxNoOfTrades:     3,
				AllocationPercent: 0,
			},
			{
				Price:             3.0,
				StopLoss:          2.7,
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
