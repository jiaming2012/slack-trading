package models

import "fmt"

type TradierOrderSide string

const (
	TradierOrderSideBuy        TradierOrderSide = "buy"
	TradierOrderSideSell       TradierOrderSide = "sell"
	TradierOrderSideBuyToCover TradierOrderSide = "buy_to_cover"
	TradierOrderSideSellShort  TradierOrderSide = "sell_short"
)

func (s TradierOrderSide) Validate() error {
	switch s {
	case TradierOrderSideBuy, TradierOrderSideSell, TradierOrderSideBuyToCover, TradierOrderSideSellShort:
		return nil
	default:
		return fmt.Errorf("invalid order side: %s", s)
	}
}
