package models

import (
	"fmt"
	"math"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterOrder struct {
	ID               uint                    `json:"id"`
	Class            BacktesterOrderClass    `json:"class"`
	Symbol           eventmodels.Instrument  `json:"symbol"`
	Side             BacktesterOrderSide     `json:"side"`
	AbsoluteQuantity float64                 `json:"quantity"`
	Type             BacktesterOrderType     `json:"type"`
	Duration         BacktesterOrderDuration `json:"duration"`
	Price            *float64                `json:"price"`
	StopPrice        *float64                `json:"stop_price"`
	Tag              *string                 `json:"tag"`
	Trades           []*BacktesterTrade      `json:"trades"`
	status           BacktesterOrderStatus
}

func (o *BacktesterOrder) Cancel() {
	o.status = BacktesterOrderStatusCancelled
}

func (o *BacktesterOrder) Reject() {
	o.status = BacktesterOrderStatusRejected
}

func (o *BacktesterOrder) GetQuantity() float64 {
	if o.Side == BacktesterOrderSideSell || o.Side == BacktesterOrderSideSellShort {
		return -o.AbsoluteQuantity
	}

	return o.AbsoluteQuantity
}

func (o *BacktesterOrder) Fill(trade *BacktesterTrade) error {
	if !o.status.IsTradingAllowed() {
		return fmt.Errorf("order is not open or partially filled")
	}

	if trade.Price <= 0 {
		return fmt.Errorf("trade price must be greater than 0")
	}

	filledQuantity := 0.0
	for _, t := range o.Trades {
		filledQuantity += t.Quantity
	}

	if trade.Quantity == 0 {
		return fmt.Errorf("trade quantity must be non-zero")
	}

	if math.Abs(trade.Quantity+filledQuantity) > o.AbsoluteQuantity {
		return fmt.Errorf("trade quantity exceeds order quantity")
	}

	o.Trades = append(o.Trades, trade)

	o.status = BacktesterOrderStatusOpen

	return nil
}

func (o *BacktesterOrder) GetStatus() BacktesterOrderStatus {
	if !o.status.IsTradingAllowed() {
		return o.status
	}

	if len(o.Trades) == 0 {
		return BacktesterOrderStatusOpen
	}

	filledQuantity := o.GetFilledQuantity()

	if filledQuantity == o.GetQuantity() {
		return BacktesterOrderStatusFilled
	}

	return BacktesterOrderStatusPartiallyFilled
}

func (o *BacktesterOrder) GetFilledQuantity() float64 {
	filledQuantity := 0.0
	for _, trade := range o.Trades {
		filledQuantity += trade.Quantity
	}

	return filledQuantity
}

func (o *BacktesterOrder) GetAvgFillPrice() float64 {
	if len(o.Trades) == 0 {
		return 0
	}

	total := 0.0
	for _, trade := range o.Trades {
		total += trade.Price
	}

	return total / float64(len(o.Trades))
}

func NewBacktesterOrder(id uint, class BacktesterOrderClass, symbol eventmodels.Instrument, side BacktesterOrderSide, quantity float64, orderType BacktesterOrderType, duration BacktesterOrderDuration, price, stopPrice *float64, status BacktesterOrderStatus, tag *string) *BacktesterOrder {
	return &BacktesterOrder{
		ID:               id,
		Class:            class,
		Symbol:           symbol,
		Side:             side,
		AbsoluteQuantity: quantity,
		Type:             orderType,
		Duration:         duration,
		Price:            price,
		StopPrice:        stopPrice,
		Tag:              tag,
		status:           status,
	}
}
