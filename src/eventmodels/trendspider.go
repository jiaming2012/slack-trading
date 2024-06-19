package eventmodels

import (
	"fmt"
	"time"

	"github.com/jiaming2012/slack-trading/src/models"
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
	Direction        models.Direction
	PriceActionEvent string
	isSatisfied      bool
}

func NewTrendlineBreakSignal(symbol string, timeframe time.Duration, price float64, direction models.Direction, priceActionEvent string) *TrendlineBreakSignal {
	return &TrendlineBreakSignal{
		Symbol:           symbol,
		Timeframe:        timeframe,
		Price:            price,
		Direction:        direction,
		PriceActionEvent: priceActionEvent,
		isSatisfied:      false,
	}
}

func (s TrendlineBreakSignal) IsSatisfied(ticks []models.Tick, trades models.Trades) bool {
	if s.isSatisfied {
		return true
	}

	if s.Direction == models.Up {
		for _, t := range ticks {
			if t.Bid >= s.Price {
				s.isSatisfied = true
				return true
			}
		}
	}

	if s.Direction == models.Down {
		for _, t := range ticks {
			if t.Ask <= s.Price {
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
