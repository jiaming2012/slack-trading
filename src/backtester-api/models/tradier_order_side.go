package models

import "fmt"

type TradierOrderSide string

const (
	TradierOrderSideBuy         TradierOrderSide = "buy"
	TradierOrderSideSell        TradierOrderSide = "sell"
	TradierOrderSideBuyToCover  TradierOrderSide = "buy_to_cover"
	TradierOrderSideSellShort   TradierOrderSide = "sell_short"
	TradierOrderSideBuyToOpen   TradierOrderSide = "buy_to_open"
	TradierOrderSideBuyToClose  TradierOrderSide = "buy_to_close"
	TradierOrderSideSellToOpen  TradierOrderSide = "sell_to_open"
	TradierOrderSideSellToClose TradierOrderSide = "sell_to_close"
)

func (s TradierOrderSide) IsLongOpen() bool {
	switch s {
	case TradierOrderSideBuy, TradierOrderSideBuyToOpen:
		return true
	default:
		return false
	}
}

func (s TradierOrderSide) Validate(class OrderRecordClass) error {
	switch class {
	case OrderRecordClassEquity:
		switch s {
		case TradierOrderSideBuy, TradierOrderSideSell, TradierOrderSideBuyToCover, TradierOrderSideSellShort:
			return nil
		default:
			return fmt.Errorf("invalid order side for class %s: %s", class, s)
		}
	case OrderRecordClassOption:
		switch s {
		case TradierOrderSideBuyToOpen, TradierOrderSideBuyToClose, TradierOrderSideSellToOpen, TradierOrderSideSellToClose:
			return nil
		default:
			return fmt.Errorf("invalid order side for class %s: %s", class, s)
		}
	default:
		return fmt.Errorf("invalid order class: %s", class)
	}
}
