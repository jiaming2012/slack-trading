package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPriceLevel(t *testing.T) {
	t.Run("test trades remaining", func(t *testing.T) {
		priceLevel := PriceLevel{
			MaxNoOfTrades: 5,
			Trades: &Trades{
				{
					RequestedVolume: 1.0,
				},
				{
					RequestedVolume: -1.0,
				},
			},
		}

		tradesRemaining, side := priceLevel.NewTradesRemaining()
		assert.Equal(t, 5, tradesRemaining)
		assert.Equal(t, side, TradeTypeBuy)

		priceLevel.Trades.Add(&Trade{
			RequestedVolume: -1.0,
		})

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		assert.Equal(t, 4, tradesRemaining)
		assert.Equal(t, side, TradeTypeSell)
	})
}
