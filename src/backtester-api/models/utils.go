package models

import (
	"math"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func isOptionSymbol(symbol string) bool {
	_, err := eventmodels.NewOptionSymbolFromString(symbol)
	return err == nil
}

// calculateMaintenanceRequirement calculates the maintenance requirement based on stock price and shares sold short
func calculateMaintenanceRequirement(stockQuantity, stockPrice float64) float64 {
	if stockQuantity < 0 {
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

	return stockQuantity * stockPrice * 0.5
}

// CalculateMaintenanceRequirement calculates the initial margin requirement based on stock price and shares sold short
func calculateInitialMarginRequirement(stockQuantity, stockPrice float64) float64 {
	if stockQuantity > 0 {
		return stockQuantity * stockPrice * 0.5
	} else if stockQuantity < 0 {
		sharesSoldShort := -stockQuantity

		var marginRequirementPerShare float64

		// Determine initial margina requirement based on stock price
		if stockPrice >= 5.0 {
			// For stocks trading at $5 or higher
			marginRequirementPerShare = max(1.50*stockPrice, 5.0)
		} else {
			// For stocks trading below $5
			marginRequirementPerShare = max(stockPrice, 2.5)
		}

		// Total initial margin requirement
		totalMargin := marginRequirementPerShare * sharesSoldShort
		return totalMargin
	}

	return 0
}

func GetInstrument(symbol string) eventmodels.Instrument {
	if _, err := eventmodels.NewOptionSymbolFromString(symbol); err == nil {
		return eventmodels.OptionSymbol(symbol)
	} else {
		return eventmodels.StockSymbol(symbol)
	}
}

func sortPositionsByQuantityDesc(positionCache *PositionsCache) ([]eventmodels.Instrument, []*Position) {
	sortedInstruments := make([]eventmodels.Instrument, 0)
	sortedPositions := make([]*Position, 0)

	instruments, positions := positionCache.List()
	for i, instrument := range instruments {
		position := positions[i]

		if len(sortedInstruments) == 0 {
			sortedInstruments = append(sortedInstruments, instrument)
			sortedPositions = append(sortedPositions, position)
			continue
		}

		insertPositionSize := math.Abs(position.Quantity) * position.CostBasis

		foundInsertionPoint := false
		for i := range sortedInstruments {
			sortedPosition := sortedPositions[i]
			sortedPositionSize := math.Abs(sortedPosition.Quantity) * sortedPosition.CostBasis

			if insertPositionSize > sortedPositionSize {
				sortedInstruments = append(sortedInstruments[:i], append([]eventmodels.Instrument{instrument}, sortedInstruments[i:]...)...)
				sortedPositions = append(sortedPositions[:i], append([]*Position{position}, sortedPositions[i:]...)...)
				foundInsertionPoint = true
				break
			}
		}

		if !foundInsertionPoint {
			sortedInstruments = append(sortedInstruments, instrument)
			sortedPositions = append(sortedPositions, position)
		}
	}

	return sortedInstruments, sortedPositions
}

func GetClass(symbol eventmodels.Instrument) OrderRecordClass {
	switch symbol.(type) {
	case eventmodels.StockSymbol:
		return OrderRecordClassEquity
	case eventmodels.OptionSymbol, *eventmodels.OptionContractV3:
		return OrderRecordClassOption
	default:
		return OrderRecordClassUnknown
	}
}
