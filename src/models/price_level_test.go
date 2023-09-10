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
					Type:           TradeTypeBuy,
					ExecutedVolume: 1.0,
				},
				{
					Type:           TradeTypeClose,
					ExecutedVolume: -1.0,
				},
			},
		}

		tradesRemaining, side := priceLevel.NewTradesRemaining()
		assert.Equal(t, 5, tradesRemaining)
		assert.Equal(t, TradeTypeNone, side)

		priceLevel.Trades.Add(&Trade{
			Type:           TradeTypeSell,
			ExecutedVolume: -1.0,
		})

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		assert.Equal(t, 4, tradesRemaining)
		assert.Equal(t, TradeTypeSell, side)
	})
}
