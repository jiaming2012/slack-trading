package models

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"math"
	"strings"
)

type PriceLevels struct {
	Bands []*PriceLevel
}

func (levels PriceLevels) GetByIndex(index int) (*PriceLevel, error) {
	if index < 0 {
		return nil, fmt.Errorf("Strategy.GetPriceLevelByIndex: Found %v: %w", index, InvalidPriceLevelIndexErr)
	}

	if index >= len(levels.Bands) {
		return nil, fmt.Errorf("Strategy.GetPriceLevelByIndex: index must be less than total number of price levels of %v", len(levels.Bands))
	}

	return levels.Bands[index], nil
}

func (levels PriceLevels) String() string {
	display := &strings.Builder{}
	p := message.NewPrinter(language.English)

	table := tablewriter.NewWriter(display)

	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetColumnSeparator("")
	display.WriteString("Price Levels:\n")

	for _, lvl := range levels.Bands {
		price := fmt.Sprintf("$%s", p.Sprintf("%.2f", lvl.Price))
		noOfTrades := fmt.Sprintf("%d trades", lvl.MaxNoOfTrades)
		allocPercentage := fmt.Sprintf("%.0f%%", lvl.AllocationPercent*100)

		table.Append([]string{price, noOfTrades, allocPercentage})
	}

	table.Render()
	return display.String()
}

func (levels *PriceLevels) Validate(direction Direction) error {
	if len(levels.Bands) < 2 {
		return MinimumNumberOfPriceLevelsNotMetErr
	}

	total := 0.0
	//for _, lvl := range levels.Bands {
	//	total += lvl.AllocationPercent
	//}
	for i := 0; i < len(levels.Bands)-1; i++ {
		switch direction {
		case Up:
			sl := levels.Bands[i].StopLoss
			if sl <= 0 {
				return fmt.Errorf("levels.Validation: invalid sl (%v), where price level direction %v: %w", sl, direction, PriceLevelStopLossMustBeOutsideLowerAndUpperRangeErr)
			}

			if sl >= levels.Bands[i].Price {
				return fmt.Errorf("levels.Validation: sl (%v) >= levels.Bands[%v] price (%v), where price level direction is %v", sl, i, levels.Bands[i].Price, direction)
			}
		case Down:
			sl := levels.Bands[i+1].StopLoss
			if sl < levels.Bands[i+1].Price {
				return fmt.Errorf("levels.Validation: sl (%v) < levels.Bands[%v] price (%v), where price level direction is %v: %w", sl, i+1, levels.Bands[i+1].Price, direction, PriceLevelStopLossMustBeOutsideLowerAndUpperRangeErr)
			}
		default:
			return fmt.Errorf("levels.Validation: invalid direction %v", direction)
		}

		total += levels.Bands[i].AllocationPercent
	}

	total += levels.Bands[len(levels.Bands)-1].AllocationPercent

	if math.Abs(1-total) > 0.001 {
		return fmt.Errorf("%w: allocation total is %v", PriceLevelsAllocationErr, total)
	}

	return nil
}

func NewPriceLevels(levels []*PriceLevel, direction Direction) (*PriceLevels, error) {
	for _, l := range levels {
		if err := l.Validate(); err != nil {
			return nil, fmt.Errorf("NewPriceLevels (%v): price level validation failed: %w", direction, err)
		}
	}

	if len(levels) < 2 {
		return nil, MinimumNumberOfPriceLevelsNotMetErr
	}

	switch direction {
	case Up:
		if levels[len(levels)-1].AllocationPercent != 0 {
			return nil, fmt.Errorf("NewPriceLevels (%v): %w", Up, PriceLevelsLastAllocationErr)
		}
	case Down:
		if levels[0].AllocationPercent != 0 {
			return nil, fmt.Errorf("NewPriceLevels (%v): %w", Down, PriceLevelsLastAllocationErr)
		}
	}

	prevPrice := 0.0
	for _, level := range levels {
		if level.Price <= 0 {
			return nil, fmt.Errorf("NewPriceLevels: invalid price level, %v", level.Price)
		}

		if level.Price < prevPrice {
			return nil, PriceLevelsNotSortedErr
		}

		if level.AllocationPercent > 0 && level.MaxNoOfTrades <= 0 {
			return nil, NoOfTradeMustBeNonzeroErr
		}

		if level.AllocationPercent == 0 && level.MaxNoOfTrades > 0 {
			return nil, NoOfTradesMustBeZeroErr
		}

		level.Trades = &Trades{}

		prevPrice = level.Price
	}

	obj := &PriceLevels{
		Bands: levels,
	}

	if err := obj.Validate(direction); err != nil {
		return nil, fmt.Errorf("NewPriceLevels: PriceLevels validation failed: %w", err)
	}

	return obj, nil
}
