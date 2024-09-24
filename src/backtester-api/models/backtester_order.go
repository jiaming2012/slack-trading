package models

import "fmt"

type BacktesterOrder struct {
	ID        uint                    `json:"id"`
	Class     BacktesterOrderClass    `json:"class"`
	Symbol    string                  `json:"symbol"`
	Side      string                  `json:"side"`
	Quantity  float64                 `json:"quantity"`
	Type      BacktesterOrderType     `json:"type"`
	Duration  BacktesterOrderDuration `json:"duration"`
	Price     *float64                `json:"price"`
	StopPrice *float64                `json:"stop_price"`
	Tag       *string                 `json:"tag"`
	Trades    []*BacktesterTrade      `json:"trades"`
	status    *BacktesterOrderStatus
}

func (o *BacktesterOrder) Cancel() {
	status := BacktesterOrderStatusCancelled
	o.status = &status
}

func (o *BacktesterOrder) Reject() {
	status := BacktesterOrderStatusRejected
	o.status = &status
}

func (o *BacktesterOrder) Fill(trade *BacktesterTrade) error {
	if o.status != nil {
		if !(*o.status).IsTradeable() {
			return fmt.Errorf("order is not open or partially filled")
		}
	}

	if trade.Price <= 0 {
		return fmt.Errorf("trade price must be greater than 0")
	}

	if trade.Quantity <= 0 {
		return fmt.Errorf("trade quantity must be greater than 0")
	}

	filledQuantity := 0.0
	for _, t := range o.Trades {
		filledQuantity += t.Quantity
	}

	if trade.Quantity+filledQuantity > o.Quantity {
		return fmt.Errorf("trade quantity exceeds order quantity")
	}

	o.Trades = append(o.Trades, trade)

	return nil
}

func (o *BacktesterOrder) GetStatus() BacktesterOrderStatus {
	if o.status != nil {
		return *o.status
	}

	if len(o.Trades) == 0 {
		return BacktesterOrderStatusOpen
	}

	filledQuantity := 0.0
	for _, trade := range o.Trades {
		filledQuantity += trade.Quantity
	}

	if filledQuantity == o.Quantity {
		return BacktesterOrderStatusFilled
	}

	return BacktesterOrderStatusPartiallyFilled
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

func NewBacktesterOrder(id uint, class BacktesterOrderClass, symbol, side string, quantity float64, orderType BacktesterOrderType, duration BacktesterOrderDuration, price, stopPrice *float64, tag *string) *BacktesterOrder {
	return &BacktesterOrder{
		ID:        id,
		Class:     class,
		Symbol:    symbol,
		Side:      side,
		Quantity:  quantity,
		Type:      orderType,
		Duration:  duration,
		Price:     price,
		StopPrice: stopPrice,
		Tag:       tag,
	}
}
