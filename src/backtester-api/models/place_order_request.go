package models

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type PlaceOrderRequest struct {
	OrderID      *uint
	Symbol       string
	OptionSymbol *string
	Quantity     int
	Side         TradierOrderSide
	OrderType    TradierOrderType
	Class        OrderRecordClass
	Tag          string
	DryRun       bool
}

func NewPlaceOrderRequest(instrument eventmodels.Instrument, quantity int, side TradierOrderSide, orderType OrderRecordType, tag string, dryRun bool) (*PlaceOrderRequest, error) {
	var symbol string
	var optionSymbol *string
	var class OrderRecordClass

	switch i := instrument.(type) {
	case eventmodels.StockSymbol:
		symbol = instrument.GetTicker()
		class = OrderRecordClassEquity
	case eventmodels.OptionSymbol:
		components, err := i.Components()
		if err != nil {
			return nil, fmt.Errorf("NewPlaceOrderRequest: failed to get option components: %w", err)
		}

		symbol = components.Underlying
		oSym := i.NoPrefix()
		optionSymbol = &oSym
		class = OrderRecordClassOption
	case *eventmodels.OptionContractV3:
		symbol = i.UnderlyingSymbol.GetTicker()
		oSym := i.Symbol.NoPrefix()
		optionSymbol = &oSym
		class = OrderRecordClassOption
	default:
		return nil, fmt.Errorf("NewPlaceOrderRequest: unsupported instrument type: %T", instrument)
	}

	return &PlaceOrderRequest{
		Symbol:       symbol,
		OptionSymbol: optionSymbol,
		Quantity:     quantity,
		Side:         side,
		Class:        class,
		OrderType:    TradierOrderTypeMarket,
		Tag:          tag,
		DryRun:       dryRun,
	}, nil
}
