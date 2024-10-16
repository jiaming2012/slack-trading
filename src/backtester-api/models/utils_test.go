package models

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func TestSortPositionsByQuantityDesc(t *testing.T) {
	t.Run("empty positions", func(t *testing.T) {
		positions := map[eventmodels.Instrument]*Position{}

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positions)

		assert.Len(t, sortedInstruments, 0)
		assert.Len(t, sortedPositions, 0)
	})

	t.Run("1 position", func(t *testing.T) {
		positions := map[eventmodels.Instrument]*Position{
			eventmodels.NewStockSymbol("ABC"): {Quantity: 1.0, CostBasis: 1.0},
		}

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positions)

		assert.Len(t, sortedInstruments, 1)
		assert.Len(t, sortedPositions, 1)
		assert.Equal(t, eventmodels.NewStockSymbol("ABC"), sortedInstruments[0])
		assert.Equal(t, 1.0, sortedPositions[0].Quantity)
	})

	t.Run("2 positions", func(t *testing.T) {
		positions := map[eventmodels.Instrument]*Position{
			eventmodels.NewStockSymbol("ABC"): {Quantity: 1.0, CostBasis: 2.0},
			eventmodels.NewStockSymbol("DEF"): {Quantity: -5.0, CostBasis: 1.0},
		}

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positions)

		assert.Len(t, sortedInstruments, 2)
		assert.Len(t, sortedPositions, 2)
		assert.Equal(t, eventmodels.NewStockSymbol("DEF"), sortedInstruments[0])
		assert.Equal(t, -5.0, sortedPositions[0].Quantity)
		assert.Equal(t, eventmodels.NewStockSymbol("ABC"), sortedInstruments[1])
		assert.Equal(t, 1.0, sortedPositions[1].Quantity)
	})

	t.Run("3 positions", func(t *testing.T) {
		positions := map[eventmodels.Instrument]*Position{
			eventmodels.NewStockSymbol("ABC"): {Quantity: 1.0, CostBasis: 1.0},
			eventmodels.NewStockSymbol("DEF"): {Quantity: -5.0, CostBasis: 1.0},
			eventmodels.NewStockSymbol("GHI"): {Quantity: 3.0, CostBasis: 2.0},
		}

		sortedInstruments, sortedPositions := sortPositionsByQuantityDesc(positions)

		assert.Len(t, sortedInstruments, 3)
		assert.Len(t, sortedPositions, 3)
		assert.Equal(t, eventmodels.NewStockSymbol("GHI"), sortedInstruments[0])
		assert.Equal(t, 3.0, sortedPositions[0].Quantity)
		assert.Equal(t, eventmodels.NewStockSymbol("DEF"), sortedInstruments[1])
		assert.Equal(t, -5.0, sortedPositions[1].Quantity)
		assert.Equal(t, eventmodels.NewStockSymbol("ABC"), sortedInstruments[2])
		assert.Equal(t, 1.0, sortedPositions[2].Quantity)
	})
}
