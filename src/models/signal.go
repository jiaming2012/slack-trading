package models

import "fmt"

type TrendLineBreakSignal struct {
	Name      string
	Price     float64
	Direction Direction
}

func (s TrendLineBreakSignal) String() string {
	return fmt.Sprintf("TrendLineBreakSignal - %v: %v @ %.2f", s.Name, s.Direction, s.Price)
}

func (s TrendLineBreakSignal) IsSatisfied(prices []Tick, trades Trades) bool {
	return lineBreakSignalIsSatisfied(s.Direction, s.Price, prices, trades)
}

type MovingAverageBreakSignal struct {
	Name      string
	Price     float64
	Direction Direction
}

func (s MovingAverageBreakSignal) String() string {
	return fmt.Sprintf("MovingAverageBreakSignal - %v: %v @ %.2f", s.Name, s.Direction, s.Price)
}

func (s MovingAverageBreakSignal) IsSatisfied(prices []Tick, trades Trades) bool {
	return lineBreakSignalIsSatisfied(s.Direction, s.Price, prices, trades)
}

func lineBreakSignalIsSatisfied(direction Direction, targetPrice float64, prices []Tick, trades Trades) bool {
	switch direction {
	case Up:
		for _, p := range prices {
			if p.Bid >= targetPrice {
				return true
			}
		}
	case Down:
		for _, p := range prices {
			if p.Ask <= targetPrice {
				return true
			}
		}
	default:
		return false
	}

	return false
}

type Signal interface {
	fmt.Stringer
	IsSatisfied(ticks []Tick, trades Trades) bool
}