package models

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

type MockBroker struct {
	requests []*PlaceOrderRequest
	orders   []*eventmodels.TradierOrder
	orderId  uint
	source   ILiveAccountSource
}

func (b *MockBroker) ExerciseOption(ctx context.Context, req *eventmodels.ExerciseOptionRequest) error {
	return fmt.Errorf("not implemented")
}

func (b *MockBroker) GetSource() ILiveAccountSource {
	return b.source
}

func (b *MockBroker) FetchEquity() (*eventmodels.FetchAccountEquityResponse, error) {
	return &eventmodels.FetchAccountEquityResponse{
		Equity: 10000000.00,
	}, nil
}

func (b *MockBroker) FetchPositions() ([]eventmodels.TradierPositionDTO, error) {
	return nil, nil
}

func (b *MockBroker) fillPlaceEquityTradeRequest(req *PlaceOrderRequest) {
	o := &eventmodels.TradierOrder{
		Symbol:                    req.Symbol,
		AbsoluteQuantity:          float64(req.Quantities[0]),
		Side:                      string(req.Sides[0]),
		Type:                      string(req.OrderType),
		Status:                    string(OrderRecordStatusPending),
		AvgFillPrice:              0,
		LastFillPrice:             0,
		AbsoluteRemainingQuantity: float64(req.Quantities[0]),
	}

	// need to get the external order id. Maybe place it on the live order?
	if req.OrderID != nil {
		o.ID = *req.OrderID
		b.orderId = uint(math.Max(float64(b.orderId), float64(*req.OrderID+1)))
	} else {
		o.ID = uint(b.orderId)
		b.orderId++
	}

	b.orders = append(b.orders, o)
}

func (b *MockBroker) FillOrder(orderId uint, price float64, status string) error {
	switch status {
	case string(OrderRecordStatusFilled):
	case string(OrderRecordStatusRejected):
	case string(OrderRecordStatusCanceled):
	default:
		return fmt.Errorf("invalid status: %s", status)
	}

	if price <= 0 {
		return fmt.Errorf("invalid price: %f", price)
	}

	for _, o := range b.orders {
		if o.ID == orderId {
			o.Status = status
			o.AvgFillPrice = price
			o.LastFillPrice = price
			o.AbsoluteExecQuantity = o.AbsoluteRemainingQuantity
			o.AbsoluteLastFillQuantity = o.AbsoluteRemainingQuantity
			o.AbsoluteRemainingQuantity = 0
			o.CreateDate = time.Now()
			return nil
		}
	}

	return fmt.Errorf("order not found")
}

func (b *MockBroker) PlaceOrder(ctx context.Context, req *PlaceOrderRequest) (map[string]interface{}, error) {
	b.requests = append(b.requests, req)
	resp := map[string]interface{}{
		"order": map[string]interface{}{
			"id": float64(b.orderId),
		},
	}

	b.fillPlaceEquityTradeRequest(req)

	return resp, nil
}

func (b *MockBroker) FetchOrders(ctx context.Context) ([]*eventmodels.TradierOrder, error) {
	return b.orders, nil
}

func (b *MockBroker) FetchBalances(url string, token string) (eventmodels.FetchTradierBalancesResponseDTO, error) {
	return eventmodels.FetchTradierBalancesResponseDTO{}, nil
}

func (b *MockBroker) FetchQuotes(ctx context.Context, symbols []eventmodels.Instrument) ([]*TradierQuoteDTO, error) {
	var quotes []*TradierQuoteDTO
	for _, symbol := range symbols {
		quotes = append(quotes, &TradierQuoteDTO{
			Symbol: symbol.GetTicker(),
			Type:   "stock",
			Last:   150.0,
		})
	}

	return quotes, nil
}

func (b *MockBroker) FetchOrder(orderId uint, accountType LiveAccountType) (*eventmodels.TradierOrder, error) {
	for _, o := range b.orders {
		if o.ID == orderId {
			return o, nil
		}
	}

	return nil, fmt.Errorf("order not found")
}

func NewMockBroker(orderIdStartIndex uint, existingOrders []*PlaceOrderRequest) *MockBroker {
	source := NewMockLiveAccountSource()

	broker := &MockBroker{
		requests: make([]*PlaceOrderRequest, 0),
		orders:   make([]*eventmodels.TradierOrder, 0),
		orderId:  orderIdStartIndex,
		source:   source,
	}

	for _, req := range existingOrders {
		broker.fillPlaceEquityTradeRequest(req)
	}

	return broker
}
