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
}

func (p *LivePlayground) GetAccount() *LiveAccount {
	return p.account
}

func (p *LivePlayground) GetRepositories() []*CandleRepository {
	return p.playground.GetRepositories()
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

func (p *LivePlayground) FillOrder(order *BacktesterOrder, performChecks bool, orderFillEntry OrderExecutionRequest, positionsMap map[eventmodels.Instrument]*Position) (*BacktesterTrade, error) {
	return p.playground.FillOrder(order, performChecks, orderFillEntry, positionsMap)
}

func (p *LivePlayground) CommitPendingOrderToOrderQueue(order *BacktesterOrder, startingPositions map[eventmodels.Instrument]*Position, orderFillEntry OrderExecutionRequest, performChecks bool) error {
	return p.playground.CommitPendingOrderToOrderQueue(order, startingPositions, orderFillEntry, performChecks)
}

func (p *LivePlayground) CommitPendingOrders(positions map[eventmodels.Instrument]*Position, orderFillPricesMap map[uint]OrderExecutionRequest, performChecks bool) (newTrades []*BacktesterTrade, invalidOrders []*BacktesterOrder, err error) {
	newTrades, invalidOrders, err = p.playground.CommitPendingOrders(positions, orderFillPricesMap, performChecks)
	return
}

func (p *LivePlayground) PlaceOrder(order *BacktesterOrder) (*PlaceOrderChanges, error) {
	placeOrderChanges, err := p.playground.PlaceOrder(order)

	if err != nil {
		return nil, fmt.Errorf("failed to place order in live playground: %w", err)
	}

	ticker := order.Symbol.GetTicker()
	qty := int(order.AbsoluteQuantity)
	req := NewPlaceEquityOrderRequest(ticker, qty, order.Side, order.Type, order.Tag, false)

	resp, err := p.account.Broker.PlaceOrder(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order in live playground: %w", err)
	}

	if orderMap, ok := resp["order"]; ok {
		result, ok := orderMap.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("LivePlayground.PlaceOrder: failed to cast response to order id map")
		}

		if orderID, ok := result["id"]; ok {
			if id, ok := orderID.(float64); ok {
				order.ID = uint(id)
			} else {
				return nil, fmt.Errorf("LivePlayground.PlaceOrder: failed to cast order id to int")
			}
		} else {
			return nil, fmt.Errorf("LivePlayground.PlaceOrder: order id not found in response")
		}
	}

	return placeOrderChanges, nil
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

func NewLivePlayground(playgroundID *uuid.UUID, account *LiveAccount, repositories []*CandleRepository, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle], orders []*BacktesterOrder, now time.Time) (*LivePlayground, error) {
	source := &PlaygroundSource{
		Broker:     account.Source.GetBroker(),
		ApiKeyName: account.Source.GetApiKeyName(),
		AccountID:  account.Source.GetAccountID(),
	}

	playground, err := NewPlayground(playgroundID, account.Balance, nil, orders, PlaygroundEnvironmentLive, source, now, repositories...)
	if err != nil {
		return nil, fmt.Errorf("NewLivePlayground: failed to create playground: %w", err)
	}

	return &LivePlayground{
		playground:      playground,
		account:         account,
		newCandlesQueue: newCandlesQueue,
	}, nil
}
