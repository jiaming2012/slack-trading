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

func NewLivePlayground(account *LiveAccount, repositories []*CandleRepository, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]) (*LivePlayground, error) {
	playground, err := NewPlayground(account.Balance, nil, PlaygroundEnvironmentLive, repositories...)
	if err != nil {
		return nil, err
	}

	return &LivePlayground{
		playground:      playground,
		account:         account,
		newCandlesQueue: newCandlesQueue,
	}, nil
}
