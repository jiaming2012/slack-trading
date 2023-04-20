package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// todo
// 1. place algo trade on rsi cross < 30 || > 70 if net exposure  <> 0
// 2. should see a slack alert
// 3. add 1 BTC on each trade. close all on opposite signal

func TestAccount(t *testing.T) {
	balance := 10000.00
	maxLossPerc := 0.05

	t.Run("the last price level must have an allocation of zero", func(t *testing.T) {
		priceLevels := PriceLevels{
			Values: []*PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        3,
					AllocationPercent: 0.5,
				},
				{
					Price:             2.0,
					NoOfTrades:        1,
					AllocationPercent: 0.5,
				},
			},
		}

		_, err := NewAccount(1.0, 0.5, priceLevels)

		assert.ErrorIs(t, err, PriceLevelsLastAllocationErr)

		priceLevels.Values = append(priceLevels.Values, &PriceLevel{
			Price:             6.0,
			NoOfTrades:        0,
			AllocationPercent: 0,
		})

		_, err = NewAccount(1.0, 0.5, priceLevels)

		assert.Nil(t, err)
	})

	t.Run("fails if no levels are set", func(t *testing.T) {
		_, err := NewAccount(1.0, 0.5, PriceLevels{})
		assert.ErrorIs(t, err, LevelsNotSetErr)
	})

	t.Run("errors if maxLossPercentage is invalid", func(t *testing.T) {
		_, err := NewAccount(1.0, -1, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0}, {Price: 2.0}},
		})
		assert.ErrorIs(t, err, MaxLossPercentErr)

		_, err = NewAccount(1.0, 1.1, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0}, {Price: 2.0}},
		})
		assert.NotNil(t, err, MaxLossPercentErr)
	})

	t.Run("errors if price levels are not sorted", func(t *testing.T) {
		_, err := NewAccount(1.0, 1.0, PriceLevels{
			Values: []*PriceLevel{{Price: 1.0, AllocationPercent: 1, NoOfTrades: 1}, {Price: 3.0}, {Price: 2.0}},
		})
		assert.ErrorIs(t, err, PriceLevelsNotSortedErr)
	})

	t.Run("num of trade > 0 if allocation is > 0", func(t *testing.T) {
		_priceLevels := PriceLevels{
			Values: []*PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        3,
					AllocationPercent: 0.5,
				},
				{
					Price:             2.0,
					AllocationPercent: 0.5,
				},
				{
					Price: 3.0,
				},
			},
		}

		_, err := NewAccount(balance, maxLossPerc, _priceLevels)
		assert.ErrorIs(t, err, NoOfTradeMustBeNonzeroErr)
	})

	t.Run("num of trades must be zero if allocation is zero", func(t *testing.T) {
		_priceLevels1 := PriceLevels{
			Values: []*PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        3,
					AllocationPercent: 0.5,
				},
				{
					Price:             2.0,
					AllocationPercent: 0,
				},
				{
					Price:             3.0,
					NoOfTrades:        1,
					AllocationPercent: 0.5,
				},
				{
					Price:             10.0,
					NoOfTrades:        0,
					AllocationPercent: 0,
				},
			},
		}

		_, err := NewAccount(balance, maxLossPerc, _priceLevels1)
		assert.Nil(t, err)

		_priceLevels2 := PriceLevels{
			Values: []*PriceLevel{
				{
					Price:             1.0,
					NoOfTrades:        3,
					AllocationPercent: 0.5,
				},
				{
					Price:             2.0,
					NoOfTrades:        3,
					AllocationPercent: 0,
				},
				{
					Price:             3.0,
					NoOfTrades:        1,
					AllocationPercent: 0.5,
				},
				{
					Price:             10.0,
					NoOfTrades:        0,
					AllocationPercent: 0,
				},
			},
		}

		_, err = NewAccount(balance, maxLossPerc, _priceLevels2)
		assert.ErrorIs(t, err, NoOfTradesMustBeZeroErr)
	})
}
