package models

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// LiveBroker is the Broker adapter for Paper and Margin modes: orders are
// netted through the reconcile playground and sent to the real broker via
// ILiveAccount; ticks drain fills and candles that arrive asynchronously
// from the live feed.
type LiveBroker struct{}

func (LiveBroker) PlaceOrder(p *Playground, order *OrderRecord) ([]*PlaceOrderChanges, error) {
	return p.placeLiveOrder(order)
}

func (LiveBroker) Tick(p *Playground, d time.Duration, isPreview bool) (*TickDelta, error) {
	delta, err := p.liveTick(d, isPreview)
	if err != nil {
		return nil, fmt.Errorf("error in live tick: %w", err)
	}

	return delta, nil
}

func (p *Playground) liveTick(duration time.Duration, isPreview bool) (*TickDelta, error) {
	if isPreview {
		return nil, fmt.Errorf("live playground does not support preview")
	}

	var newCandles []*BacktesterCandle

	for {
		candle, ok := p.GetNewCandlesQueue().Dequeue()
		if ok {
			newCandles = append(newCandles, candle)

			if telemetry.CandlesProcessed != nil {
				telemetry.CandlesProcessed.Add(context.Background(), 1,
					telemetry.PlaygroundAttrs(string(p.Meta.Environment), string(p.Meta.LiveAccountType), telemetry.ClientIDOrEmpty(p.GetClientId())),
					metric.WithAttributes(
						attribute.String("symbol", candle.Symbol.GetTicker()),
					))
			}

			continue
		}

		break
	}

	var newTrades []*TradeRecord

	for {
		trade, ok := p.GetNewTradesQueue().Dequeue()
		if ok {
			newTrades = append(newTrades, trade)
			continue
		}

		break
	}

	var invalidOrders []*OrderRecord

	for {
		order, ok := p.GetInvalidOrdersQueue().Dequeue()
		if ok {
			invalidOrders = append(invalidOrders, order)
			continue
		}

		break
	}

	currentTime := p.GetCurrentTime()

	equityPlot, err := p.updateAccountStats(currentTime)
	if err != nil {
		log.Warnf("failed to update account stats: %v", err)
	}

	return &TickDelta{
		NewCandles:         newCandles,
		NewTrades:          newTrades,
		InvalidOrders:      invalidOrders,
		Events:             nil,
		CurrentTime:        currentTime.Format(time.RFC3339),
		IsBacktestComplete: false,
		EquityPlot:         equityPlot,
	}, nil
}

func (p *Playground) placeLiveOrder(order *OrderRecord) ([]*PlaceOrderChanges, error) {
	var changes []*PlaceOrderChanges

	pendingOrders := p.GetPendingOrders()
	for i := len(pendingOrders) - 1; i >= 0; i-- {
		o := pendingOrders[i]
		if o.ID == order.ID {
			pendingOrders = append(pendingOrders[:i], pendingOrders[i+1:]...)
		}
	}

	if len(pendingOrders) > 0 {
		o := pendingOrders[0]
		cliReqID := ""
		if o.ClientRequestID != nil {
			cliReqID = *o.ClientRequestID
		}

		log.Infof("placeLiveOrder: pending (order %d, cliReqID=%s) already exists, placing order %d into new orders queue", o.ID, cliReqID, order.ID)

		changes = append(changes, &PlaceOrderChanges{
			Commit: func(tx *gorm.DB) error {
				p.AddToNewOrdersQueue(order)
				return nil
			},
			Info: fmt.Sprintf("adding order %d to new orders queue", order.ID),
		})
	} else {
		// no pending orders, place the order
		if p.ReconcilePlayground.GetLiveAccount() == nil {
			return nil, fmt.Errorf("live account is not set")
		}

		reconcilePlayground := p.ReconcilePlayground
		if reconcilePlayground == nil {
			return nil, fmt.Errorf("reconcile playground is not set")
		}

		if reconcilePlayground.GetId() == p.GetId() {
			return nil, fmt.Errorf("cannot place order in the same playground")
		}

		// todo: place all changes inside of a single transaction
		playgroundChanges, err := p.placeOrder(order) // remove from new queue and place into pending
		if err != nil {
			return nil, fmt.Errorf("failed to place order in live playground: %w", err)
		}

		// todo: place all changes inside of a single transaction
		reconciliationChanges, reconciliationOrders, err := reconcilePlayground.PlaceOrder(order)
		if err != nil {
			return nil, fmt.Errorf("failed to place order in reconcile playground: %w", err)
		}

		changes = append(changes, reconciliationChanges...)
		changes = append(changes, playgroundChanges...)

		for i, o := range reconciliationOrders {
			changes = append(changes, &PlaceOrderChanges{
				Commit: func(tx *gorm.DB) error {
					_order := o
					forceNew := true
					if _order.ID > 0 {
						forceNew = false
					}

					if err := p.ReconcilePlayground.GetLiveAccount().GetDatabase().SaveOrderRecordTx(tx, _order, forceNew); err != nil {
						return fmt.Errorf("failed to save reconciliation order record: %w", err)
					}

					return nil
				},
				Info: fmt.Sprintf("iteration %d - save reconciliation order record %d", i+1, order.ID),
			})
		}
	}

	changes = append(changes, &PlaceOrderChanges{
		Commit: func(tx *gorm.DB) error {
			forceNew := true
			if order.ID > 0 {
				forceNew = false
			}

			if err := p.GetLiveAccount().GetDatabase().SaveOrderRecordTx(tx, order, forceNew); err != nil {
				return fmt.Errorf("failed to update live order record: %w", err)
			}

			return nil
		},
		Info: "update live order record",
	})

	return changes, nil
}
