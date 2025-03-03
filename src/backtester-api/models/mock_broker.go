package models

import (
	"context"
	"fmt"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockBroker struct {
	requests     []*PlaceEquityTradeRequest
	orders       []*eventmodels.TradierOrder
	orderId      uint
	executePrice float64
}

func (b *MockBroker) SetFillOrderExecutionPrice(price float64) {
	b.executePrice = price
}

func (b *MockBroker) fillPlaceEquityTradeRequest(req *PlaceEquityTradeRequest) {
	b.orders = append(b.orders, &eventmodels.TradierOrder{
		ID:                       uint(b.orderId),
		Symbol:                   req.Symbol,
		AbsoluteQuantity:         float64(req.Quantity),
		Side:                     string(req.Side),
		Type:                     string(req.OrderType),
		Status:                   string(BacktesterOrderStatusFilled),
		AvgFillPrice:             b.executePrice,
		AbsoluteLastFillQuantity: float64(req.Quantity),
	})
}

func (b *MockBroker) PlaceOrder(ctx context.Context, req *PlaceEquityTradeRequest) (map[string]interface{}, error) {
	b.requests = append(b.requests, req)
	resp := map[string]interface{}{
		"order": map[string]interface{}{
			"id": float64(b.orderId),
		},
	}

	b.fillPlaceEquityTradeRequest(req)

	b.orderId++

	return resp, nil
}

func (b *MockBroker) FetchOrders(ctx context.Context) ([]*eventmodels.TradierOrder, error) {
	return b.orders, nil
}

func (b *MockBroker) FetchQuotes(ctx context.Context, symbols []eventmodels.Instrument) ([]*TradierQuoteDTO, error) {
	return []*TradierQuoteDTO{
		{
			Symbol: "AAPL",
			Type:   "stock",
			Last:   150.0,
		},
	}, nil
}

func (b *MockBroker) FetchOrder(orderId uint, accountType LiveAccountType) (*eventmodels.TradierOrder, error) {
	for _, o := range b.orders {
		if o.ID == orderId {
			return o, nil
		}
	}

	return nil, fmt.Errorf("order not found")
}

func NewMockBroker(orderIdStartIndex uint) *MockBroker {
	return &MockBroker{
		requests: make([]*PlaceEquityTradeRequest, 0),
		orders:   make([]*eventmodels.TradierOrder, 0),
		orderId:  orderIdStartIndex,
	}
}
