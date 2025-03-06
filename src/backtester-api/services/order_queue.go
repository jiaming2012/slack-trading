package services

import (
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func UpdateTradierOrderQueue(sink *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent], dbService models.IDatabaseService, sleepDuration time.Duration) error {
	pendingOrders, err := dbService.FetchPendingOrders(models.LiveAccountTypeReconcilation)
	if err != nil {
		return fmt.Errorf("UpdateTradierOrderQueue: failed to fetch orders: %v", err)
	}

	for _, order := range pendingOrders {
		var liveAccountType models.LiveAccountType

		playground, err := dbService.FetchPlayground(order.PlaygroundID)
		if err != nil {
			log.Errorf("UpdateTradierOrderQueue: failed to fetch playground: %v", err)
			continue
		}

		liveAccountType = models.LiveAccountType(playground.AccountType)
		if err := liveAccountType.Validate(); err != nil {
			log.Errorf("UpdateTradierOrderQueue: invalid account type: %v", err)
			continue
		}

		liveAccount := playground.GetReconcilePlayground().GetLiveAccount()
		if liveAccount == nil {
			log.Errorf("UpdateTradierOrderQueue: live account not found: %v", playground)
			continue
		}

		if order.ExternalOrderID == nil {
			log.Errorf("UpdateTradierOrderQueue: external order id not found: %v", order)
			continue
		}

		tradierOrder, err := liveAccount.GetBroker().FetchOrder(*order.ExternalOrderID, liveAccountType)
		if err != nil {
			log.Errorf("UpdateTradierOrderQueue: failed to fetch order: %v", err)
			continue
		}

		if tradierOrder.Status == string(models.OrderRecordStatusFilled) {
			rec := order
			sink.Enqueue(&models.TradierOrderUpdateEvent{
				CreateOrder: &models.TradierOrderCreateEvent{
					Order:       tradierOrder,
					OrderRecord: rec,
				},
			})

			log.Debugf("TradierApiWorker.executeOrdersQueueUpdate: order %d is filled", order.ExternalOrderID)
		} else if tradierOrder.Status == string(models.OrderRecordStatusRejected) {
			reason := "rejected by broker"
			if tradierOrder.ReasonDescription != nil {
				reason = *tradierOrder.ReasonDescription
			}

			if order.ExternalOrderID == nil {
				log.Errorf("TradierApiWorker.executeOrdersQueueUpdate: external order id not found: %v", order)
				continue
			}

			sink.Enqueue(&models.TradierOrderUpdateEvent{
				ModifyOrder: &models.TradierOrderModifyEvent{
					PlaygroundId:   playground.ID,
					TradierOrderID: *order.ExternalOrderID,
					Field:          "status",
					New:            reason,
				},
			})
		}

		time.Sleep(sleepDuration)
	}

	log.Tracef("TradierApiWorker.executeOrdersQueueUpdate: fetched %d pending orders", len(pendingOrders))

	return nil
}

func fillPendingOrder(playground *models.Playground, order *models.OrderRecord, orderFillEntry models.ExecutionFillRequest, tradierOrder uint, cache *models.OrderCache, database models.IDatabaseService) (*models.TradeRecord, error) {
	performChecks := false

	positions, err := playground.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("handleLiveOrders: failed to get positions: %w", err)
	}

	newTrade, invalidOrder, err := playground.CommitPendingOrder(order, positions, orderFillEntry, performChecks)
	if err != nil {
		if errors.Is(err, models.ErrTradingNotAllowed) {
			log.Debugf("handleLiveOrders: removing order from cache: %v", tradierOrder)
			cache.Remove(tradierOrder, false)
			return nil, nil
		}

		return nil, fmt.Errorf("handleLiveOrders: failed to commit pending orders: %w", err)
	}

	if invalidOrder != nil {
		return nil, fmt.Errorf("handleLiveOrders: invalid order: %v", invalidOrder)
	}

	// Resave the order to update the status and close_id
	balance := playground.GetBalance()
	if err := database.SaveOrderRecord(order, &balance, false); err != nil {
		if errors.Is(err, models.ErrDbOrderIsNotOpenOrPending) {
			log.Warnf("handleLiveOrders: order is not open or pending: %v", err)
			cache.Remove(tradierOrder, false)
			return newTrade, nil
		}

		return nil, fmt.Errorf("handleLiveOrders: failed to save order record: %v", err)
	}

	return newTrade, nil
}

func CommitPendingOrders(cache *models.OrderCache, database models.IDatabaseService) error {
	orderCache, unlockFn := cache.GetMap()

	defer unlockFn()

	for tradierOrder, orderFillEntry := range orderCache {
		playground, order, err := database.FindOrder(orderFillEntry.PlaygroundId, tradierOrder)

		if err != nil {
			log.Errorf("handleLiveOrders: failed to find reconciled order: %v", err)
			continue
		}

		if _, err = fillPendingOrder(playground, order, orderFillEntry, tradierOrder, cache, database); err != nil {
			log.Errorf("handleLiveOrders: failed to fill reconciled order: %v", err)
			continue
		}

		// Update live order that was reconciled
		for _, o := range order.Reconciles {
			// liveOrder := o
			// livePlayground := o.Playground

			p, err := database.FetchPlayground(o.PlaygroundID)
			if err != nil {
				log.Errorf("handleLiveOrders: failed to fetch playground for reconciled order: %v", err)
				continue
			}

			trade, err := fillPendingOrder(p, o, orderFillEntry, tradierOrder, cache, database)
			if err != nil {
				log.Errorf("handleLiveOrders: failed to fill live order: %v", err)
				continue
			}

			// if p, ok := livePlayground.(*models.LivePlayground); ok {
			if p.ReconcilePlayground != nil {
				p.GetNewTradesQueue().Enqueue(trade)
			} else {
				log.Errorf("handleLiveOrders: playground is not live: %v", playground)
			}

			log.Infof("handleLiveOrders: opened trade: %v", trade)
		}

		cache.Remove(tradierOrder, false)
	}

	return nil
}

func DrainTradierOrderQueue(source *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent], cache *models.OrderCache, database models.IDatabaseService) (hasUpdates bool, err error) {
	hasUpdates = false

	for {
		event, ok := source.Dequeue()
		if !ok {
			break
		}

		hasUpdates = true
		if event.CreateOrder != nil {
			if event.CreateOrder.Order.Status == string(models.OrderRecordStatusFilled) {
				cache.Add(event.CreateOrder.Order, models.ExecutionFillRequest{
					PlaygroundId: event.CreateOrder.OrderRecord.PlaygroundID,
					Time:         event.CreateOrder.Order.CreateDate,
					Price:        event.CreateOrder.Order.AvgFillPrice,
					Quantity:     event.CreateOrder.Order.GetLastFillQuantity(),
				})

				log.Debugf("handleLiveOrders: order filled: %v", event.CreateOrder.Order)
			} else if event.CreateOrder.Order.Status == string(models.OrderRecordStatusPending) {
				log.Debugf("handleLiveOrders: order pending: %v", event.CreateOrder.Order)
			} else {
				log.Fatalf("handleLiveOrders: unknown order status: %v", event.CreateOrder.Order.Status)
			}

		} else if event.ModifyOrder != nil {
			if event.ModifyOrder.Field == "status" {
				// todo: remove once all orders have links to playground, after PlaygroundSession refactor
				playground, order, err := database.FindOrder(event.ModifyOrder.PlaygroundId, event.ModifyOrder.TradierOrderID)

				if err == nil {
					reason, ok := event.ModifyOrder.New.(string)
					if !ok {
						log.Errorf("handleLiveOrders: failed to convert reason to string: %v", event.ModifyOrder.New)
						continue
					}

					if err := playground.RejectOrder(order, reason, database); err != nil {
						log.Errorf("handleLiveOrders: failed to reject order: %v", err)
					}

					if err := database.SaveOrderRecord(order, nil, false); err != nil {
						log.Fatalf("handleLiveOrders: failed to save order record: %v", err)
					}
				} else {
					log.Warnf("handleLiveOrders: order not found: %v", event.ModifyOrder.TradierOrderID)
				}
			}
		} else if event.DeleteOrder != nil {
			// pass
		} else {
			log.Warnf("handleLiveOrders: unknown event type: %v", event)
		}
	}

	return
}
