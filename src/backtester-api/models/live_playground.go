package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type LivePlayground struct {
	playground      *Playground
	liveAccount     *LiveAccount
	newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]
	newTradesQueue  *eventmodels.FIFOQueue[*TradeRecord]
	requestHash     *string
}

func (p *LivePlayground) GetClientId() *string {
	return p.playground.GetClientId()
}

func (p *LivePlayground) GetLiveAccountType() LiveAccountType {
	return p.liveAccount.Source.GetAccountType()
}

func (p *LivePlayground) SetOpenOrdersCache() error {
	return p.playground.SetOpenOrdersCache()
}

func (p *LivePlayground) GetNewTradeQueue() *eventmodels.FIFOQueue[*TradeRecord] {
	return p.newTradesQueue
}

func (p *LivePlayground) GetRequestHash() *string {
	return p.requestHash
}

// func (p *LivePlayground) GetAccount() *LiveAccount {
// 	return p.account
// }

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

func (p *LivePlayground) GetPosition(symbol eventmodels.Instrument, checkExists bool) (Position, error) {
	return p.playground.GetPosition(symbol, checkExists)
}

func (p *LivePlayground) GetPositions() (map[eventmodels.Instrument]*Position, error) {
	return p.playground.GetPositions()
}

func (p *LivePlayground) GetCandle(symbol eventmodels.Instrument, period time.Duration) (*eventmodels.PolygonAggregateBarV2, error) {
	return p.playground.GetCandle(symbol, period)
}

func (p *LivePlayground) GetFreeMargin() (float64, error) {
	return p.playground.GetFreeMargin()
}

func (p *LivePlayground) FillOrder(order *BacktesterOrder, performChecks bool, orderFillEntry ExecutionFillRequest, positionsMap map[eventmodels.Instrument]*Position) (*TradeRecord, error) {
	return p.playground.FillOrder(order, performChecks, orderFillEntry, positionsMap)
}

func (p *LivePlayground) SetEquityPlot(plot []*eventmodels.EquityPlot) {
	p.playground.SetEquityPlot(plot)
}

func (p *LivePlayground) GetEquityPlot() []*eventmodels.EquityPlot {
	return p.playground.GetEquityPlot()
}

func (p *LivePlayground) PlaceOrder(order *BacktesterOrder) ([]*PlaceOrderChanges, error) {
	reconcilePlayground := p.liveAccount.GetReconcilePlayground()
	if reconcilePlayground == nil {
		return nil, fmt.Errorf("reconcile playground is not set")
	}

	if reconcilePlayground.GetId() == p.GetId() {
		return nil, fmt.Errorf("cannot place order in the same playground")
	}

	playgroundChanges, err := p.playground.PlaceOrder(order)
	if err != nil {
		return nil, fmt.Errorf("failed to place order in live playground: %w", err)
	}

	reconciliationChanges, err := reconcilePlayground.PlaceOrder(p.liveAccount, order)
	if err != nil {
		return nil, fmt.Errorf("failed to place order in reconcile playground: %w", err)
	}

	changes := append(reconciliationChanges, playgroundChanges...)

	return changes, nil
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

	var newTrades []*TradeRecord

	for {
		trade, ok := p.newTradesQueue.Dequeue()
		if ok {
			newTrades = append(newTrades, trade)
			continue
		}

		break
	}

	currentTime := p.GetCurrentTime()

	equityPlot, err := p.playground.updateAccountStats(currentTime)
	if err != nil {
		return nil, fmt.Errorf("failed to update account stats: %w", err)
	}

	return &TickDelta{
		NewCandles:         newCandles,
		NewTrades:          newTrades,
		Events:             nil,
		CurrentTime:        currentTime.Format(time.RFC3339),
		IsBacktestComplete: false,
		EquityPlot:         equityPlot,
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

func NewLivePlayground(playgroundID *uuid.UUID, clientID *string, liveAccount *LiveAccount, startingBalance float64, repositories []*CandleRepository, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle], newTradesQueue *eventmodels.FIFOQueue[*TradeRecord], orders []*BacktesterOrder, now time.Time, tags []string) (*LivePlayground, error) {
	// account.Broker
	playground, err := NewPlayground(playgroundID, clientID, startingBalance, startingBalance, nil, orders, PlaygroundEnvironmentLive, now, tags, repositories...)
	if err != nil {
		return nil, fmt.Errorf("NewLivePlayground: failed to create playground: %w", err)
	}

	log.Debugf("adding newCandlesQueue(%p) to NewLivePlayground", newCandlesQueue)

	return &LivePlayground{
		playground:      playground,
		liveAccount:     liveAccount,
		newCandlesQueue: newCandlesQueue,
		newTradesQueue:  newTradesQueue,
	}, nil
}
