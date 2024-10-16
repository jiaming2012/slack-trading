package models

import (
	"math"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

// CalculateMaintenanceRequirement calculates the maintenance requirement based on stock price and shares sold short
func calculateMaintenanceRequirement(stockQuantity, stockPrice float64) float64 {
	if stockQuantity > 0 {
		return stockQuantity * stockPrice
	} else if stockQuantity < 0 {
		sharesSoldShort := -stockQuantity

		var maintenanceRequirementPerShare float64

		// Determine maintenance requirement based on stock price
		if stockPrice >= 5.0 {
			// For stocks trading at $5 or higher
			maintenanceRequirementPerShare = max(1.50*stockPrice, 5.0)
		} else {
			// For stocks trading below $5
			maintenanceRequirementPerShare = max(stockPrice, 2.5)
		}

		// Total maintenance requirement
		totalMaintenance := maintenanceRequirementPerShare * sharesSoldShort
		return totalMaintenance
	}

	return 0
}

func sortPositionsByQuantityDesc(positions map[eventmodels.Instrument]*Position) ([]eventmodels.Instrument, []*Position) {
	sortedSymbols := make([]eventmodels.Instrument, 0)
	sortedPositions := make([]*Position, 0)

	for symbol, position := range positions {
		if len(sortedSymbols) == 0 {
			sortedSymbols = append(sortedSymbols, symbol)
			sortedPositions = append(sortedPositions, position)
			continue
		}

		insertPositionSize := math.Abs(position.Quantity) * position.CostBasis

		foundInsertionPoint := false
		for i := range sortedSymbols {
			sortedPosition := sortedPositions[i]
			sortedPositionSize := math.Abs(sortedPosition.Quantity) * sortedPosition.CostBasis

			if insertPositionSize > sortedPositionSize {
				sortedSymbols = append(sortedSymbols[:i], append([]eventmodels.Instrument{symbol}, sortedSymbols[i:]...)...)
				sortedPositions = append(sortedPositions[:i], append([]*Position{position}, sortedPositions[i:]...)...)
				foundInsertionPoint = true
				break
			}
		}

		if !foundInsertionPoint {
			sortedSymbols = append(sortedSymbols, symbol)
			sortedPositions = append(sortedPositions, position)
		}
	}

	return sortedSymbols, sortedPositions
}
