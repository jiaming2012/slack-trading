package models

import (
	"testing"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/stretchr/testify/require"
)

func TestSortPositionsByQuantityDesc(t *testing.T) {
	t.Run("empty positions", func(t *testing.T) {
		positions := map[eventmodels.Instrument]*Position{}

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positions)

		require.Len(t, sortedInstruments, 0)
		require.Len(t, sortedPositions, 0)
	})

	t.Run("1 position", func(t *testing.T) {
		positions := map[eventmodels.Instrument]*Position{
			eventmodels.NewStockSymbol("ABC"): {Quantity: 1.0, CostBasis: 1.0},
		}

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positions)

		require.Len(t, sortedInstruments, 1)
		require.Len(t, sortedPositions, 1)
		require.Equal(t, eventmodels.NewStockSymbol("ABC"), sortedInstruments[0])
		require.Equal(t, 1.0, sortedPositions[0].Quantity)
	})

	t.Run("2 positions", func(t *testing.T) {
		positions := map[eventmodels.Instrument]*Position{
			eventmodels.NewStockSymbol("ABC"): {Quantity: 1.0, CostBasis: 2.0},
			eventmodels.NewStockSymbol("DEF"): {Quantity: -5.0, CostBasis: 1.0},
		}

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positions)

		require.Len(t, sortedInstruments, 2)
		require.Len(t, sortedPositions, 2)
		require.Equal(t, eventmodels.NewStockSymbol("DEF"), sortedInstruments[0])
		require.Equal(t, -5.0, sortedPositions[0].Quantity)
		require.Equal(t, eventmodels.NewStockSymbol("ABC"), sortedInstruments[1])
		require.Equal(t, 1.0, sortedPositions[1].Quantity)
	})

	t.Run("3 positions", func(t *testing.T) {
		positions := map[eventmodels.Instrument]*Position{
			eventmodels.NewStockSymbol("ABC"): {Quantity: 1.0, CostBasis: 1.0},
			eventmodels.NewStockSymbol("DEF"): {Quantity: -5.0, CostBasis: 1.0},
			eventmodels.NewStockSymbol("GHI"): {Quantity: 3.0, CostBasis: 2.0},
		}

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positions)

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
