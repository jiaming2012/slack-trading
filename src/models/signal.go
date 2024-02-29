package models

import (
	"fmt"
	"time"
)

type SignalType int

const (
	SignalTypeEntry SignalType = iota
	SignalTypeExit
	SignalTypeReset
)

type ExitSignalDTO struct {
	Signal      *SignalV2DTO `json:"exitSignal"`
	ResetSignal *ResetSignal `json:"resetSignal"`
}

func (s *ExitSignalDTO) ToExitSignal() *ExitSignal {
	var signal *SignalV2
	if s.Signal != nil {
		signal = s.Signal.ToSignalV2()
	}

	return &ExitSignal{
		Signal:      signal,
		ResetSignal: s.ResetSignal,
	}
}

func (s *ExitSignal) ConvertToDTO() *ExitSignalDTO {
	return &ExitSignalDTO{
		Signal:      s.Signal.ConvertToDTO(),
		ResetSignal: s.ResetSignal,
	}
}

type ExitSignal struct {
	Signal      *SignalV2
	ResetSignal *ResetSignal
}

func NewExitSignal(signal *SignalV2, resetSignal *ResetSignal) *ExitSignal {
	return &ExitSignal{Signal: signal, ResetSignal: resetSignal}
}

func (s *ExitSignal) Update(signalType SignalType) {
	now := time.Now().UTC()

	switch signalType {
	case SignalTypeExit:
		s.Signal.Update(true, now)
	case SignalTypeReset:
		s.ResetSignal.Update(now)
	default:
		return
	}
}

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
