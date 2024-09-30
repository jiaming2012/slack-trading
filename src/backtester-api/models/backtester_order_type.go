package models

import "fmt"

type BacktesterOrderType string

const (
	Market    BacktesterOrderType = "market"
	Limit     BacktesterOrderType = "limit"
	Stop      BacktesterOrderType = "stop"
	StopLimit BacktesterOrderType = "stop_limit"
)

func (t BacktesterOrderType) Validate() error {
	switch t {
	case Market, Limit, Stop, StopLimit:
		return nil
	default:
		return fmt.Errorf("invalid order type: %s", t)
	}
}
