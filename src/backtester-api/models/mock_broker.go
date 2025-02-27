package models

import (
	"context"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockBroker struct {
	requests []*PlaceEquityTradeRequest
	orders   []*eventmodels.TradierOrder
}

func (b *MockBroker) PlaceOrder(ctx context.Context, req *PlaceEquityTradeRequest) (map[string]interface{}, error) {
	b.requests = append(b.requests, req)
	resp := map[string]interface{}{
		"order": map[string]interface{}{
			"id": float64(123),
		},
	}

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

func NewMockBroker() *MockBroker {
	return &MockBroker{
		requests: make([]*PlaceEquityTradeRequest, 0),
		orders:   make([]*eventmodels.TradierOrder, 0),
	}
}
