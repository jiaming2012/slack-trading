package eventmodels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTradesRemaining(t *testing.T) {
	t.Run("test trades remaining for sell", func(t *testing.T) {
		tr1 := &Trade{
			Type:           TradeTypeSell,
			ExecutedVolume: 1.0,
		}

		priceLevel := PriceLevel{
			MaxNoOfTrades: 1,
			Trades: &Trades{
				tr1,
				{
					Type:           TradeTypeClose,
					ExecutedVolume: -1.0,
					Offsets: []*Trade{
						tr1,
					},
				},
			},
		}

		tradesRemaining, side := priceLevel.NewTradesRemaining()
		require.Equal(t, 1, tradesRemaining)
		require.Equal(t, TradeTypeNone, side)

		tr2 := &Trade{
			Type:           TradeTypeBuy,
			ExecutedVolume: -1.0,
		}
		priceLevel.Trades.Add(tr2)

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		require.Equal(t, 0, tradesRemaining)
		require.Equal(t, TradeTypeBuy, side)

		tr3 := &Trade{
			Type:           TradeTypeBuy,
			ExecutedVolume: -1.0,
		}
		priceLevel.Trades.Add(tr3)

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		require.Equal(t, 0, tradesRemaining)
		require.Equal(t, TradeTypeBuy, side)

		priceLevel.Trades.Add(&Trade{
			Type:           TradeTypeClose,
			ExecutedVolume: -1.0,
			Offsets: []*Trade{
				tr2,
			},
		})

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		require.Equal(t, 0, tradesRemaining)
		require.Equal(t, TradeTypeBuy, side)

		priceLevel.Trades.Add(&Trade{
			Type:           TradeTypeClose,
			ExecutedVolume: -1.0,
			Offsets: []*Trade{
				tr3,
			},
		})

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		require.Equal(t, 1, tradesRemaining)
		require.Equal(t, TradeTypeNone, side)
	})

	t.Run("test trades remaining for buy", func(t *testing.T) {
		tr1 := &Trade{
			Type:           TradeTypeBuy,
			ExecutedVolume: 1.0,
		}

		priceLevel := PriceLevel{
			MaxNoOfTrades: 2,
			Trades: &Trades{
				tr1,
				{
					Type:           TradeTypeClose,
					ExecutedVolume: -1.0,
					Offsets: []*Trade{
						tr1,
					},
				},
			},
		}

		tradesRemaining, side := priceLevel.NewTradesRemaining()
		require.Equal(t, 2, tradesRemaining)
		require.Equal(t, TradeTypeNone, side)

		priceLevel.Trades.Add(&Trade{
			Type:           TradeTypeSell,
			ExecutedVolume: -1.0,
		})

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		require.Equal(t, 1, tradesRemaining)
		require.Equal(t, TradeTypeSell, side)

		priceLevel.Trades.Add(&Trade{
			Type:           TradeTypeSell,
			ExecutedVolume: -1.0,
		})

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		require.Equal(t, 0, tradesRemaining)
		require.Equal(t, TradeTypeSell, side)

		priceLevel.Trades.Add(&Trade{
			Type:           TradeTypeSell,
			ExecutedVolume: -1.0,
		})

		tradesRemaining, side = priceLevel.NewTradesRemaining()
		require.Equal(t, 0, tradesRemaining)
		require.Equal(t, TradeTypeSell, side)
	})
}
