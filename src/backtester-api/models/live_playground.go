package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type LivePlayground struct {
	Playground   *Playground
	Account      *LiveAccount
	Repositories []*LiveCandleRepository
}

func (p *LivePlayground) GetMeta() *PlaygroundMeta {
	return p.Playground.GetMeta()
}

func (p *LivePlayground) GetId() uuid.UUID {
	return p.Playground.GetId()
}

func (p *LivePlayground) GetBalance() float64 {
	return p.Playground.GetBalance()
}

func (p *LivePlayground) GetEquity(positions map[eventmodels.Instrument]*Position) float64 {
	return p.Playground.GetEquity(positions)
}

func (p *LivePlayground) GetOrders() []*BacktesterOrder {
	return p.Playground.GetOrders()
}

func (p *LivePlayground) GetPosition(symbol eventmodels.Instrument) Position {
	return p.Playground.GetPosition(symbol)
}

func (p *LivePlayground) GetPositions() map[eventmodels.Instrument]*Position {
	return p.Playground.GetPositions()
}

func (p *LivePlayground) GetCandle(symbol eventmodels.Instrument, period time.Duration) (*eventmodels.PolygonAggregateBarV2, error) {
	return p.Playground.GetCandle(symbol, period)
}

func (p *LivePlayground) GetFreeMargin() float64 {
	return p.Playground.GetFreeMargin()
}

func (p *LivePlayground) PlaceOrder(order *BacktesterOrder) error {
	return p.Playground.PlaceOrder(order)
}

func (p *LivePlayground) Tick(duration time.Duration, isPreview bool) (*TickDelta, error) {
	return p.Playground.Tick(duration, isPreview)
}

func (p *LivePlayground) GetFreeMarginFromPositionMap(positions map[eventmodels.Instrument]*Position) float64 {
	return p.Playground.GetFreeMarginFromPositionMap(positions)
}

func (p *LivePlayground) GetOpenOrders(symbol eventmodels.Instrument) []*BacktesterOrder {
	return p.Playground.GetOpenOrders(symbol)
}

func (p *LivePlayground) GetCurrentTime() time.Time {
	return p.Playground.GetCurrentTime()
}

func (p *LivePlayground) NextOrderID() uint {
	return p.Playground.NextOrderID()
}

func (p *LivePlayground) FetchCandles(symbol eventmodels.Instrument, period time.Duration, from, to time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	return p.Playground.FetchCandles(symbol, period, from, to)
}

func NewLivePlayground(account *LiveAccount, repositories []*LiveCandleRepository) (*LivePlayground, error) {
	playground, err := NewPlayground(account.Balance, nil, PlaygroundEnvironmentLive)
	if err != nil {
		return nil, err
	}
	
	return &LivePlayground{
		Playground:   playground,
		Account:      account,
		Repositories: repositories,
	}, nil
}
