package models

import (
	"fmt"

)

type Balance struct {
	Floating float64
	Realized RealizedPL
	Vwap     Vwap
	Volume   Volume
}

func (b Balance) String() string {
	return fmt.Sprintf("Open volume: %.2f BTC\nVWAP: %.2f\nFloating profit: $%.2f\nRealized profit: $%.2f", b.Volume, b.Vwap, b.Floating, b.Realized)
}
