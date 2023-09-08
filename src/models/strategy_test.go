package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStrategy(t *testing.T) {
	name := "test"
	symbol := "symbol"
	direction := Direction("up")
	balance := 1000.0
	newPriceLevels := func() []*PriceLevel {
		return []*PriceLevel{
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
