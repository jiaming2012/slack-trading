package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestSortPositionsByQuantityDesc(t *testing.T) {
	t.Run("empty positions", func(t *testing.T) {
		positionCache := NewPositionCache()

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positionCache)

		require.Len(t, sortedInstruments, 0)
		require.Len(t, sortedPositions, 0)
	})

	t.Run("1 position", func(t *testing.T) {
		positionCache := NewPositionCache()
		positionCache.Add(eventmodels.NewStockSymbol("ABC"), &TradeRecord{
			Quantity: 1.0,
			Price:    1.0,
		})

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positionCache)

		require.Len(t, sortedInstruments, 1)
		require.Len(t, sortedPositions, 1)
		require.Equal(t, eventmodels.NewStockSymbol("ABC"), sortedInstruments[0])
		require.Equal(t, 1.0, sortedPositions[0].Quantity)
	})

	t.Run("2 positions", func(t *testing.T) {
		positionCache := NewPositionCache()
		positionCache.Add(eventmodels.NewStockSymbol("ABC"), &TradeRecord{
			Quantity: 1.0,
			Price:    2.0,
		})
		positionCache.Add(eventmodels.NewStockSymbol("DEF"), &TradeRecord{
			Quantity: -5.0,
			Price:    1.0,
		})

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positionCache)

		require.Len(t, sortedInstruments, 2)
		require.Len(t, sortedPositions, 2)
		require.Equal(t, eventmodels.NewStockSymbol("DEF"), sortedInstruments[0])
		require.Equal(t, -5.0, sortedPositions[0].Quantity)
		require.Equal(t, eventmodels.NewStockSymbol("ABC"), sortedInstruments[1])
		require.Equal(t, 1.0, sortedPositions[1].Quantity)
	})

	t.Run("3 positions", func(t *testing.T) {
		positionCache := NewPositionCache()
		positionCache.Add(eventmodels.NewStockSymbol("ABC"), &TradeRecord{
			Quantity: 1.0,
			Price:    1.0,
		})
		positionCache.Add(eventmodels.NewStockSymbol("DEF"), &TradeRecord{
			Quantity: -5.0,
			Price:    1.0,
		})
		positionCache.Add(eventmodels.NewStockSymbol("GHI"), &TradeRecord{
			Quantity: 3.0,
			Price:    2.0,
		})

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positionCache)

		require.Len(t, sortedInstruments, 3)
		require.Len(t, sortedPositions, 3)
		require.Equal(t, eventmodels.NewStockSymbol("GHI"), sortedInstruments[0])
		require.Equal(t, 3.0, sortedPositions[0].Quantity)
		require.Equal(t, eventmodels.NewStockSymbol("DEF"), sortedInstruments[1])
		require.Equal(t, -5.0, sortedPositions[1].Quantity)
		require.Equal(t, eventmodels.NewStockSymbol("ABC"), sortedInstruments[2])
		require.Equal(t, 1.0, sortedPositions[2].Quantity)
	})
}
