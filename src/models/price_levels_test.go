package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewPriceLevels(t *testing.T) {
	t.Run("overlapping price bands throws error", func(t *testing.T) {
		levels := []*PriceLevel{
			{
				MaxNoOfTrades:     1,
				Price:             1.0,
				StopLoss:          0.5,
				AllocationPercent: 0.5,
			},
			{
				MaxNoOfTrades:     1,
				Price:             1.5,
				StopLoss:          0.5,
				AllocationPercent: 0.3,
			},
			{
				MaxNoOfTrades:     1,
				Price:             1.3,
				StopLoss:          0.5,
				AllocationPercent: 0.2,
			},
			{
				Price:             5.0,
				AllocationPercent: 0.0,
			},
		}

		_, err := NewPriceLevels(levels, Up)
		assert.ErrorIs(t, err, PriceLevelsNotSortedErr)
	})

	t.Run("stop loss must be outside of price band", func(t *testing.T) {
		levels := []*PriceLevel{
			{
				Price: 1.0,
			},
			{
				MaxNoOfTrades:     1,
				Price:             1.5,
				StopLoss:          2.5,
				AllocationPercent: 0.5,
			},
			{
				MaxNoOfTrades:     1,
				Price:             5.0,
				StopLoss:          4.5,
				AllocationPercent: 0.5,
			},
		}

		_, err := NewPriceLevels(levels, Down)
		assert.ErrorIs(t, err, PriceLevelStopLossMustBeOutsideLowerAndUpperRange)
	})

	t.Run("up price levels with gaps", func(t *testing.T) {

	})

	t.Run("down price levels with gaps", func(t *testing.T) {

	})
}
