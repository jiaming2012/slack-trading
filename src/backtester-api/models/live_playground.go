package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type LivePlayground struct {
	playground      *Playground
	account         *LiveAccount
	newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]
	pendingOrders   map[uint]*BacktesterOrder
}

func (p *LivePlayground) GetAccount() *LiveAccount {
	return p.account
}

func (p *LivePlayground) GetMeta() *PlaygroundMeta {
	return p.playground.GetMeta()
}

func (p *LivePlayground) GetId() uuid.UUID {
	return p.playground.GetId()
}

func (p *LivePlayground) GetBalance() float64 {
	return p.playground.GetBalance()
}

func (p *LivePlayground) GetEquity(positions map[eventmodels.Instrument]*Position) float64 {
	return p.playground.GetEquity(positions)
}

func (p *LivePlayground) GetOrders() []*BacktesterOrder {
	return p.playground.GetOrders()
}

func (p *LivePlayground) GetPosition(symbol eventmodels.Instrument) Position {
	return p.playground.GetPosition(symbol)
}

func (p *LivePlayground) GetPositions() map[eventmodels.Instrument]*Position {
	return p.playground.GetPositions()
}

func (p *LivePlayground) GetCandle(symbol eventmodels.Instrument, period time.Duration) (*eventmodels.PolygonAggregateBarV2, error) {
	return p.playground.GetCandle(symbol, period)
}

func (p *LivePlayground) GetFreeMargin() float64 {
	return p.playground.GetFreeMargin()
}

func (p *LivePlayground) CommitPendingOrders(positions map[eventmodels.Instrument]*Position, orderFillPricesMap map[*BacktesterOrder]OrderFillEntry, performChecks bool) (newTrades []*BacktesterTrade, invalidOrders []*BacktesterOrder, err error) {
	newTrades, invalidOrders, err = p.playground.CommitPendingOrders(positions, orderFillPricesMap, performChecks)
	return
}

func (p *LivePlayground) PlaceOrder(order *BacktesterOrder) error {
	if err := p.playground.PlaceOrder(order); err != nil {
		return fmt.Errorf("failed to place order in live playground: %w", err)
	}

	ticker := order.Symbol.GetTicker()
	qty := int(order.AbsoluteQuantity)
	req := NewPlaceEquityOrderRequest(ticker, qty, order.Side, order.Type, order.Tag, false)

	resp, err := p.account.Broker.PlaceOrder(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to place order in live playground: %w", err)
	}

	if orderMap, ok := resp["order"]; ok {
		result, ok := orderMap.(map[string]interface{})
		if !ok {
			return fmt.Errorf("LivePlayground.PlaceOrder: failed to cast response to order id map")
		}

		if orderID, ok := result["id"]; ok {
			if id, ok := orderID.(float64); ok {
				order.ID = uint(id)
			} else {
				return fmt.Errorf("LivePlayground.PlaceOrder: failed to cast order id to int")
			}
		} else {
			return fmt.Errorf("LivePlayground.PlaceOrder: order id not found in response")
		}
	}

	p.pendingOrders[order.ID] = order

	return nil
}

func (p *LivePlayground) Tick(duration time.Duration, isPreview bool) (*TickDelta, error) {
	if isPreview {
		return nil, fmt.Errorf("live playground does not support preview")
	}

	var newCandles []*BacktesterCandle

	for {
		candle, ok := p.newCandlesQueue.Dequeue()
		if ok {
			newCandles = append(newCandles, candle)
			continue
		}

		break
	}

	return &TickDelta{
		NewCandles:         newCandles,
		Events:             nil,
		CurrentTime:        p.GetCurrentTime().Format(time.RFC3339),
		IsBacktestComplete: false,
	}, nil
}

func (p *LivePlayground) GetFreeMarginFromPositionMap(positions map[eventmodels.Instrument]*Position) float64 {
	return p.playground.GetFreeMarginFromPositionMap(positions)
}

func (p *LivePlayground) GetOpenOrders(symbol eventmodels.Instrument) []*BacktesterOrder {
	return p.playground.GetOpenOrders(symbol)
}

func (p *LivePlayground) GetCurrentTime() time.Time {
	return time.Now()
}

func (p *LivePlayground) NextOrderID() uint {
	return p.playground.NextOrderID()
}

func (p *LivePlayground) FetchCandles(symbol eventmodels.Instrument, period time.Duration, from, to time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	return p.playground.FetchCandles(symbol, period, from, to)
}

func (p *LivePlayground) RejectOrder(order *BacktesterOrder, reason string) error {
	return p.playground.RejectOrder(order, reason)
}

func NewLivePlayground(account *LiveAccount, repositories []*CandleRepository, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle], backfillOrders []*eventmodels.TradierOrder) (*LivePlayground, error) {
	playground, err := NewPlayground(account.Balance, nil, PlaygroundEnvironmentLive, repositories...)
	if err != nil {
		return nil, fmt.Errorf("NewLivePlayground: failed to create playground: %w", err)
	}

	orderFillPriceMap := make(map[*BacktesterOrder]OrderFillEntry)

	for _, order := range backfillOrders {
		if order.Status == string(BacktesterOrderStatusFilled) {
			price := order.AvgFillPrice

			o := &BacktesterOrder{
				ID:               order.ID,
				Type:             BacktesterOrderType(order.Type),
				Symbol:           eventmodels.NewStockSymbol(order.Symbol),
				Side:             TradierOrderSide(order.Side),
				AbsoluteQuantity: order.Quantity,
				Class:            BacktesterOrderClass(order.Class),
				Duration:         BacktesterOrderDuration(order.Duration),
				Price:            &price,
				Tag:              order.Tag,
				CreateDate:       order.CreateDate,
			}

			if err := playground.PlaceOrder(o); err != nil {
				return nil, fmt.Errorf("NewLivePlayground: failed to place backfill: %w", err)
			}

			orderFillPriceMap[o] = OrderFillEntry{
				Time: order.CreateDate,
				Price: price,
			}
		}
	}

	performChecks := false
	positions := playground.GetPositions()

	_, invalid_orders, err := playground.CommitPendingOrders(positions, orderFillPriceMap, performChecks)
	if err != nil {
		return nil, fmt.Errorf("NewLivePlayground: failed to backfill order: %w", err)
	}

	if len(invalid_orders) > 0 {
		return nil, fmt.Errorf("NewLivePlayground: invalid backfill orders: %v", invalid_orders)
	}

	return &LivePlayground{
		playground:      playground,
		account:         account,
		newCandlesQueue: newCandlesQueue,
		pendingOrders:   map[uint]*BacktesterOrder{},
	}, nil
}
