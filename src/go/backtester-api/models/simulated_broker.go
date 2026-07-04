package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// SimulatedBroker is the Broker adapter for Simulation mode: fills are
// decided by the internal fill engine against the playground's candle
// repositories, and the playground clock advances per tick.
type SimulatedBroker struct{}

func (SimulatedBroker) PlaceOrder(p *Playground, order *OrderRecord) ([]*PlaceOrderChanges, error) {
	return p.placeOrder(order)
}

func (SimulatedBroker) Tick(p *Playground, d time.Duration, isPreview bool) (*TickDelta, error) {
	delta, err := p.simulateTick(d, isPreview)
	if err != nil {
		return nil, fmt.Errorf("error simulating tick: %w", err)
	}

	return delta, nil
}

func (p *Playground) simulateTick(d time.Duration, isPreview bool) (*TickDelta, error) {
	if isPreview {
		nextTick := p.clock.GetNext(p.clock.CurrentTime, d)

		var newCandles []*BacktesterCandle
		for instrument, periodRepoMap := range p.repos.Iter() {
			for period, repo := range periodRepoMap {
				newCandle, err := repo.FetchCandlesAtOrAfter(nextTick)
				if err != nil {
					log.Warnf("repo.FetchCandlesAtOrAfter [%s]: %v", instrument, err)
					return nil, fmt.Errorf("backtest complete: no more ticks")
				}

				if newCandle != nil {
					newCandles = append(newCandles, &BacktesterCandle{
						Symbol: instrument,
						Period: period,
						Bar:    newCandle,
					})
				}
			}
		}

		isBacktestComplete := p.clock.IsTimeExpired(nextTick)

		return &TickDelta{
			NewCandles:         newCandles,
			CurrentTime:        nextTick.Format(time.RFC3339),
			IsBacktestComplete: isBacktestComplete,
		}, nil
	}

	// Update the account
	p.account.mutex.Lock()
	defer p.account.mutex.Unlock()

	orderExecutionRequests := make(map[*OrderRecord]ExecutionFillRequest)
	for _, order := range p.account.PendingOrders {
		price, err := p.FetchCurrentPrice(context.Background(), order.GetInstrument())
		if err != nil {
			if errors.Is(err, ErrCurrentPriceNotSet) {
				log.Warn("current price not set")
				continue
			}

			if errors.Is(err, models.ErrNoCandlesFound) {
				order.Reject(err)
				if telemetry.ShouldEmitOrderTelemetry(string(p.Meta.Environment)) {
					log.WithFields(log.Fields{
						"event":         "data_gap",
						"playground_id": p.Meta.PlaygroundId,
						"symbol":        order.GetInstrument().GetTicker(),
						"timeframe":     d.String(),
						"timestamp":     p.clock.CurrentTime.Format(time.RFC3339),
						"environment":   string(p.Meta.Environment),
					}).Warn("simulateTick: no candles found")
				}
				continue
			}

			return nil, fmt.Errorf("error fetching price: %w", err)
		}

		orderExecutionRequests[order] = ExecutionFillRequest{
			Price:    price,
			Time:     p.clock.CurrentTime,
			Quantity: order.GetQuantity(),
		}
	}

	newTrades, invalidOrdersDTO, positionCache, err := p.CommitOrderQueue(orderExecutionRequests)
	if err != nil {
		return nil, fmt.Errorf("error updating order queue: %w", err)
	}

	// Check for liquidations
	liquidationEvents, err := p.checkForLiquidations(positionCache)
	if err != nil {
		return nil, fmt.Errorf("error checking for liquidations: %w", err)
	}

	var tickDeltaEvents []*TickDeltaEvent
	if liquidationEvents != nil {
		tickDeltaEvents = append(tickDeltaEvents, liquidationEvents)
	}

	// Update the clock
	if !p.clock.IsExpired() {
		p.clock.Add(d)
	}

	if p.clock.IsExpired() {
		if p.isBacktestComplete {
			return nil, fmt.Errorf("backtest complete: clock expired")
		}

		p.isBacktestComplete = true

		log.Infof("setting status -> backtest complete: clock expired")

		return &TickDelta{
			IsBacktestComplete: true,
		}, nil
	}

	// Update prices in candle repos
	var newCandles []*BacktesterCandle
	for instrument, periodRepoMap := range p.repos.Iter() {
		for period, repo := range periodRepoMap {
			newCandle, err := repo.Update(p.clock.CurrentTime)

			if err != nil {
				if errors.Is(err, models.ErrOptionContractIsExpired) {

				} else {
					log.Warnf("repo.Next [%s]: %v", instrument, err)
					return nil, fmt.Errorf("backtest complete: no more ticks")
				}
			}

			if newCandle != nil {
				newCandles = append(newCandles, &BacktesterCandle{
					Symbol: instrument,
					Period: period,
					Bar:    newCandle,
				})
			}
		}
	}

	// Drain clock-gated signals from repository
	var newSignals []*eventmodels.TradeSignal
	if p.signalRepo != nil {
		newSignals = p.signalRepo.ReadPending(p.clock.CurrentTime)
	}

	// Emit signal consumption telemetry
	for _, sig := range newSignals {
		if telemetry.SignalsConsumed != nil {
			telemetry.SignalsConsumed.Add(context.Background(), 1,
				metric.WithAttributes(
					attribute.String("signal_name", string(sig.Name)),
					attribute.String("symbol", string(sig.Symbol)),
				))
		}
	}

	// update option contracts
	for instrument := range p.repos.Iter() {
		switch s := instrument.(type) {
		case *eventmodels.OptionContractV3:
			isExpired := s.Expiration.Before(p.clock.CurrentTime) || s.Expiration.Equal(p.clock.CurrentTime)
			if isExpired {
				currentPrice, err := p.getPriceAt(s.UnderlyingSymbol, s.Expiration)
				if err != nil {
					log.Warnf("error getting current prices for %s: %v", s.UnderlyingSymbol, err)
					continue
				}

				tickDeltaEvents = append(tickDeltaEvents, &TickDeltaEvent{
					Type: TickDeltaEventTypeOptionExpired,
					OptionExpirationEvent: &OptionExpirationEvent{
						Symbol:                  s.Symbol,
						UnderlyingPriceAtExpiry: currentPrice,
						Timestamp:               p.clock.CurrentTime,
					},
				})
			}
		case eventmodels.OptionSymbol:
			log.Fatal("option symbols not supported in simulateTick")
		}
	}

	// check option assignments
	exerciseOptionsRequests := p.exerciseOptionsRequestQueue.Drain()
	for _, req := range exerciseOptionsRequests {
		order := req.Order.(*OrderRecord)

		tickDeltaEvents = append(tickDeltaEvents, &TickDeltaEvent{
			Type: TickDeltaEventTypeOptionAssigned,
			OptionAssignmentEvent: &OptionAssignmentEvent{
				OrderId:          order.ID,
				Symbol:           order.GetInstrument(),
				AssignedQuantity: req.AssignedQuantity,
				AssignedPrice:    req.AssignmentPrice,
				Timestamp:        p.clock.CurrentTime,
			},
		})
	}

	if _, err := p.updateAccountStats(p.GetCurrentTime()); err != nil {
		return nil, fmt.Errorf("error updating account stats: %w", err)
	}

	return &TickDelta{
		NewTrades:     newTrades,
		NewCandles:    newCandles,
		NewSignals:    newSignals,
		CurrentTime:   p.clock.CurrentTime.Format(time.RFC3339),
		InvalidOrders: invalidOrdersDTO,
		Events:        tickDeltaEvents,
	}, nil
}
