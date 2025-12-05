package models

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type PlaceOrderRequest struct {
	OrderID       *uint
	Symbol        string
	OptionSymbols []string
	Quantities    []int
	Sides         []TradierOrderSide
	OrderType     TradierOrderType
	Class         OrderRecordClass
	Tag           string
	DryRun        bool
}

func (r *PlaceOrderRequest) validate() error {
	if len(r.Quantities) == 0 {
		return fmt.Errorf("Validate: quantities cannot be empty")
	}

	if len(r.Sides) == 0 {
		return fmt.Errorf("Validate: sides cannot be empty")
	}

	switch r.Class {
	case OrderRecordClassMultiLegOption:
		if len(r.OptionSymbols) < 2 {
			return fmt.Errorf("Validate: multi-leg order must have at least two option symbols")
		}

		if len(r.OptionSymbols) != len(r.Quantities) {
			return fmt.Errorf("Validate: option symbols and quantities length mismatch for multi-leg order")
		}

		if len(r.OptionSymbols) != len(r.Sides) {
			return fmt.Errorf("Validate: option symbols and sides length mismatch for multi-leg order")
		}
	case OrderRecordClassEquity, OrderRecordClassOption:
		if len(r.OptionSymbols) > 1 || len(r.Quantities) > 1 || len(r.Sides) > 1 {
			return fmt.Errorf("Validate: single-leg order cannot have multiple option symbols, quantities, or sides")
		}
	default:
		return fmt.Errorf("Validate: unsupported order class: %s", r.Class)
	}

	return nil
}

func NewMultiLegOptionPlaceOrderRequest(instruments []eventmodels.Instrument, quantities []int, sides []TradierOrderSide, tag string, dryRun bool) (*PlaceOrderRequest, error) {
	var underlyingSymbol string
	var optionSymbols []string

	for _, instrument := range instruments {
		var symbol string
		switch i := instrument.(type) {
		case eventmodels.OptionSymbol:
			components, err := i.Components()
			if err != nil {
				return nil, fmt.Errorf("NewPlaceOrderRequest: failed to get option components: %w", err)
			}

			symbol = components.Underlying
			optionSymbols = append(optionSymbols, i.NoPrefix())
		case *eventmodels.OptionContractV3:
			symbol = i.UnderlyingSymbol.GetTicker()
			optionSymbols = append(optionSymbols, i.Symbol.NoPrefix())
		default:
			return nil, fmt.Errorf("NewPlaceOrderRequest: unsupported instrument type: %T", instrument)
		}

		if underlyingSymbol == "" {
			underlyingSymbol = symbol
		} else if underlyingSymbol != symbol {
			return nil, fmt.Errorf("NewPlaceOrderRequest: all options must have the same underlying symbol")
		}
	}

	for _, qty := range quantities {
		if qty <= 0 {
			return nil, fmt.Errorf("NewPlaceOrderRequest: quantities must be positive")
		}
	}

	for _, side := range sides {
		if err := side.Validate(OrderRecordClassOption); err != nil {
			return nil, fmt.Errorf("NewPlaceOrderRequest: invalid order side: %s", side)
		}
	}

	req := &PlaceOrderRequest{
		Symbol:        underlyingSymbol,
		OptionSymbols: optionSymbols,
		Quantities:    quantities,
		Sides:         sides,
		Class:         OrderRecordClassMultiLegOption,
		OrderType:     TradierOrderTypeMarket,
		Tag:           tag,
		DryRun:        dryRun,
	}

	if err := req.validate(); err != nil {
		return nil, fmt.Errorf("NewPlaceOrderRequest: validation failed: %w", err)
	}

	return req, nil
}

func NewPlaceOrderRequest(instrument eventmodels.Instrument, quantity int, side TradierOrderSide, orderType OrderRecordType, tag string, dryRun bool) (*PlaceOrderRequest, error) {
	var symbol string
	var optionSymbols []string
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
		optionSymbols = []string{i.NoPrefix()}
		class = OrderRecordClassOption
	case *eventmodels.OptionContractV3:
		symbol = i.UnderlyingSymbol.GetTicker()
		optionSymbols = []string{i.Symbol.NoPrefix()}
		class = OrderRecordClassOption
	default:
		return nil, fmt.Errorf("NewPlaceOrderRequest: unsupported instrument type: %T", instrument)
	}

	req := &PlaceOrderRequest{
		Symbol:        symbol,
		OptionSymbols: optionSymbols,
		Quantities:    []int{quantity},
		Sides:         []TradierOrderSide{side},
		Class:         class,
		OrderType:     TradierOrderTypeMarket,
		Tag:           tag,
		DryRun:        dryRun,
	}

	if err := req.validate(); err != nil {
		return nil, fmt.Errorf("NewPlaceOrderRequest: validation failed: %w", err)
	}

	return req, nil
}
