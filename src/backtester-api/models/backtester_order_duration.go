package models

import "fmt"

type BacktesterOrderDuration string

const (
	Day        BacktesterOrderDuration = "day"
	GTC        BacktesterOrderDuration = "gtc"
	PreMarket  BacktesterOrderDuration = "pre"
	PostMarket BacktesterOrderDuration = "post"
)

func (d BacktesterOrderDuration) Validate() error {
	switch d {
	case Day, GTC, PreMarket, PostMarket:
		return nil
	default:
		return fmt.Errorf("invalid order duration: %s", d)
	}
}
