package models

import (
	"fmt"
	"time"

)

type SupportBreakSignal struct {
	Symbol           string
	Timeframe        time.Duration
	Price            float64
	PriceActionEvent string
}

type ResistanceBreakSignal struct {
	Symbol           string
	Timeframe        time.Duration
	Price            float64
	PriceActionEvent string
}
type TrendlineBreakSignal struct {
	Symbol           string
	Timeframe        time.Duration
	Price            float64
	Direction        Direction
	PriceActionEvent string
	isSatisfied      bool
}

func NewTrendlineBreakSignal(symbol string, timeframe time.Duration, price float64, direction Direction, priceActionEvent string) *TrendlineBreakSignal {
	return &TrendlineBreakSignal{
		Symbol:           symbol,
		Timeframe:        timeframe,
		Price:            price,
		Direction:        direction,
		PriceActionEvent: priceActionEvent,
		isSatisfied:      false,
	}
}

func (s TrendlineBreakSignal) IsSatisfied(ticks []Tick, trades Trades) bool {
	if s.isSatisfied {
		return true
	}

	// Adapted from legacy bid/ask ticks to the canonical single-price Tick model
	// (reconcile-models-packages: bid/ask -> Price).
	if s.Direction == Up {
		for _, t := range ticks {
			if t.Price >= s.Price {
				s.isSatisfied = true
				return true
			}
		}
	}

	if s.Direction == Down {
		for _, t := range ticks {
			if t.Price <= s.Price {
				s.isSatisfied = true
				return true
			}
		}
	}

	return false
}

func (s TrendlineBreakSignal) String() string {
	return fmt.Sprintf("%s - %.0f - %.2f - %s", s.Symbol, s.Timeframe.Minutes(), s.Price, s.PriceActionEvent)
}
