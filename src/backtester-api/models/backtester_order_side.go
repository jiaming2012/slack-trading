package models

import "fmt"

type BacktesterOrderSide string

const (
	BacktesterOrderSideBuy        BacktesterOrderSide = "buy"
	BacktesterOrderSideSell       BacktesterOrderSide = "sell"
	BacktesterOrderSideBuyToCover BacktesterOrderSide = "buy_to_cover"
	BacktesterOrderSideSellShort  BacktesterOrderSide = "sell_short"
)

func (s BacktesterOrderSide) Validate() error {
	switch s {
	case BacktesterOrderSideBuy, BacktesterOrderSideSell, BacktesterOrderSideBuyToCover, BacktesterOrderSideSellShort:
		return nil
	default:
		return fmt.Errorf("invalid order side: %s", s)
	}
}
