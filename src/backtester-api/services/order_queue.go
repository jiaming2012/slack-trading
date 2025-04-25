package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/jinzhu/copier"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func UpdatePendingMarginOrders(dbService models.IDatabaseService) error {
	seekFromPlayground := true

	pendingOrders, err := dbService.FetchPendingOrders([]models.LiveAccountType{models.LiveAccountTypeMargin, models.LiveAccountTypePaper, models.LiveAccountTypeMock}, seekFromPlayground)
	if err != nil {
		return fmt.Errorf("UpdatePendingMarginOrders: failed to fetch orders: %v", err)
	}

	var newTrades []*models.TradeRecord
	var joinedErr error

	for _, order := range pendingOrders {
		trades, err := dbService.FetchTradesFromReconciliationOrders(order.ID, seekFromPlayground)
		if err != nil {
			e := fmt.Errorf("UpdatePendingMarginOrders: failed to fetch trades: %v", err)
			joinedErr = errors.Join(joinedErr, e)
			log.Error(e)
			continue
		}

		if len(trades) == 0 {
			// check if the reconciliation order is cancelled or rejected
			orders, err := dbService.FetchReconciliationOrders(order.ID, seekFromPlayground)
			if err != nil {
				e := fmt.Errorf("UpdatePendingMarginOrders: failed to fetch reconciliation orders: %v", err)
				joinedErr = errors.Join(joinedErr, e)
				log.Error(e)
				continue
			}

			// create a copy of the orders
			ordersCopy := make([]*models.OrderRecord, len(orders))
			for i := 0; i < len(orders); i++ {
				c := &models.OrderRecord{}
				if err := copier.Copy(c, orders[i]); err != nil {
					log.Fatalf("UpdatePendingMarginOrders: failed to copy order: %v", err)
				}
				ordersCopy[i] = c
			}

			shouldUpdate := false
			for _, o := range ordersCopy {
				if o.Status == models.OrderRecordStatusCanceled {
					order.Cancel()
					log.Infof("UpdatePendingMarginOrders: order %d was cancelled", order.ID)
					shouldUpdate = true
				} else if o.Status == models.OrderRecordStatusRejected {
					rejectReason := "unknown"
					if o.RejectReason != nil {
						rejectReason = *o.RejectReason
					}

					err := fmt.Errorf(rejectReason)
					order.Reject(err)
					log.Infof("UpdatePendingMarginOrders: order %d status is %s", order.ID, o.Status)
					shouldUpdate = true
				}
			}

			if shouldUpdate {
				if err := dbService.SaveOrderRecords(ordersCopy, false); err != nil {
					e := fmt.Errorf("UpdatePendingMarginOrders: failed to save order record: %v", err)
					joinedErr = errors.Join(joinedErr, e)
					log.Error(e)
					continue
				}

				// copy the orders to the original slice
				for i := range orders {
					if err := copier.Copy(orders[i], &ordersCopy[i]); err != nil {
						log.Fatalf("UpdatePendingMarginOrders: failed to copy order: %v", err)
					}
				}
			}
		}

		playground, err := dbService.FetchPlayground(order.PlaygroundID)
		if err != nil {
			e := fmt.Errorf("UpdatePendingMarginOrders: failed to fetch playground: %v", err)
			joinedErr = errors.Join(joinedErr, e)
			log.Error(e)
			continue
		}

		if playground.ReconcilePlayground == nil {
			e := fmt.Errorf("UpdatePendingMarginOrders: reconcile playground not found for order: %v", order)
			joinedErr = errors.Join(joinedErr, e)
			log.Error(e)
			continue
		}

		for _, trade := range trades {
			found := false
			for _, o := range order.Trades {
				if o.ID == trade.ID {
					found = true
					break
				}
			}

			if !found {
				if _, err := fillPendingOrder(playground, order, models.ExecutionFillRequest{
					ReconcilePlayground: playground.ReconcilePlayground,
					OrderRecord:         order,
					Trade:               trade,
				}, dbService); err != nil {
					e := fmt.Errorf("UpdatePendingMarginOrders: failed to fill pending order: %v", err)
					joinedErr = errors.Join(joinedErr, e)
					log.Error(e)
					continue
				}

				newTrades = append(newTrades, trade)
				log.Infof("UpdatePendingMarginOrders: filled pending order: %v", trade)
			}
		}

		for _, trade := range newTrades {
			log.Tracef("UpdatePendingMarginOrders: playground %v enqueuing trade #%v", playground.GetId().String(), trade.ID)
			playground.GetNewTradesQueue().Enqueue(trade)
		}
	}

	if joinedErr == nil {
		newOrder, e := dbService.FetchNewOrder()
		if e != nil {
			err = fmt.Errorf("handleLiveOrders: failed to fetch new orders: %w", e)
			joinedErr = errors.Join(joinedErr, err)
			return joinedErr
		}

		if newOrder != nil {
			playground, e := dbService.GetPlayground(newOrder.PlaygroundID)
			if e != nil {
				err = fmt.Errorf("handleLiveOrders: failed to get playground: %w", e)
				joinedErr = errors.Join(joinedErr, err)
				return joinedErr
			}

			// remove order from new orders queue
			order := playground.PopNewOrdersQueue(newOrder.ID)

			// add order to pending orders
			// playground.AddToPendingOrdersQueue(order)

			// if e := dbService.SaveOrderRecord(order, nil, false); e != nil {
			// 	err = fmt.Errorf("handleLiveOrders: failed to save order record: %w", e)
			// 	joinedErr = errors.Join(joinedErr, err)
			// 	return joinedErr
			// }

			// place order in the playground
			playgroundChanges, e := playground.PlaceOrder(order)
			if e != nil {
				err = fmt.Errorf("handleLiveOrders: failed to place order: %w", e)
				joinedErr = errors.Join(joinedErr, err)
				return joinedErr
			}

			for _, change := range playgroundChanges {
				if e := change.Commit(); e != nil {
					err = fmt.Errorf("handleLiveOrders: failed to commit change: %w", e)
					joinedErr = errors.Join(joinedErr, err)
					return joinedErr
				}
			}

			log.Debugf("handleLiveOrders: order placed from new orders queue: %v", order)
		} else {
			log.Debugf("handleLiveOrders: no new open orders")
		}
	}

	return joinedErr
}

func UpdateTradierOrderQueue(sink *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent], dbService models.IDatabaseService, sleepDuration time.Duration) error {
	pendingOrders, err := dbService.FetchPendingOrders([]models.LiveAccountType{models.LiveAccountTypeReconcilation}, false)
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

		if order.IsAdjustment {
			req := models.ExecutionFillRequest{
				ReconcilePlayground: nil,
				OrderRecord:         order,
				Time:                order.Timestamp,
				Price:               order.RequestedPrice,
				Quantity:            order.GetQuantity(),
			}

			trade, err := fillPendingOrder(playground, order, req, dbService)
			if err != nil {
				log.Errorf("UpdateTradierOrderQueue: failed to fill adjustment order: %v", err)
				continue
			}

			if trade != nil {
				log.Infof("UpdateTradierOrderQueue: filled adjustment trade: %v", trade)
			}

			continue
		}

		reconcilePlayground, found, err := dbService.FetchReconcilePlaygroundByOrder(order)
		if err != nil {
			log.Errorf("UpdateTradierOrderQueue: failed to fetch reconcile playground: %v", err)
			continue
		}

		if !found {
			log.Errorf("UpdateTradierOrderQueue: reconcile playground not found for order: %v", order)
			continue
		}

		liveAccountType = playground.Meta.LiveAccountType
		if err := liveAccountType.Validate(); err != nil {
			log.Errorf("UpdateTradierOrderQueue: invalid account type: %v", err)
			continue
		}

		liveAccount := reconcilePlayground.GetLiveAccount()
		if liveAccount == nil {
			log.Errorf("UpdateTradierOrderQueue: live account not found: %v", playground)
			continue
		}

		if order.ExternalOrderID == nil {
			log.Errorf("UpdateTradierOrderQueue: external order id not found: %v", order)
			continue
		}

		var playgroundOrder *models.OrderRecord
		for _, o := range reconcilePlayground.GetOrders() {
			if o.ExternalOrderID != nil && *o.ExternalOrderID == *order.ExternalOrderID {
				playgroundOrder = o
				break
			}
		}

		if playgroundOrder == nil {
			log.Errorf("UpdateTradierOrderQueue: order not found in playground: %v", order)
			continue
		}

		tradierOrder, err := liveAccount.GetBroker().FetchOrder(*playgroundOrder.ExternalOrderID, liveAccountType)
		if err != nil {
			log.Errorf("UpdateTradierOrderQueue: failed to fetch order: %v", err)
			continue
		}

		if tradierOrder.Status == string(models.OrderRecordStatusFilled) {
			rec := playgroundOrder
			sink.Enqueue(&models.TradierOrderUpdateEvent{
				CreateOrder: &models.TradierOrderCreateEvent{
					Order:               tradierOrder,
					OrderRecord:         rec,
					ReconcilePlayground: reconcilePlayground,
				},
			})

			log.Infof("TradierApiWorker.executeOrdersQueueUpdate: order %d is filled by broker", order.ExternalOrderID)
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

			log.Infof("TradierApiWorker.executeOrdersQueueUpdate: order %d is rejected by broker", order.ExternalOrderID)
		} else if tradierOrder.Status == string(models.OrderRecordStatusCanceled) {
			if order.ExternalOrderID == nil {
				log.Errorf("TradierApiWorker.executeOrdersQueueUpdate: external order id not found: %v", order)
				continue
			}

			sink.Enqueue(&models.TradierOrderUpdateEvent{
				ModifyOrder: &models.TradierOrderModifyEvent{
					PlaygroundId:   playground.ID,
					TradierOrderID: *order.ExternalOrderID,
					Field:          "status",
					New:            string(models.OrderRecordStatusCanceled),
				},
			})

			log.Infof("TradierApiWorker.executeOrdersQueueUpdate: order %d is canceled by broker", order.ExternalOrderID)
		} else if tradierOrder.Status == string(models.OrderRecordStatusPending) {
			log.Tracef("TradierApiWorker.executeOrdersQueueUpdate: order %d is pending", order.ExternalOrderID)
			continue
		} else {
			log.Warnf("TradierApiWorker.executeOrdersQueueUpdate: unknown order status: %v", tradierOrder.Status)
			continue
		}

		time.Sleep(sleepDuration)
	}

	log.Tracef("TradierApiWorker.executeOrdersQueueUpdate: fetched %d pending orders", len(pendingOrders))

	return nil
}

func fillPendingOrder(playground *models.Playground, order *models.OrderRecord, orderFillEntry models.ExecutionFillRequest, database models.IDatabaseService) (*models.TradeRecord, error) {
	performChecks := false

	positionCache, err := playground.UpdatePricesAndGetPositionCache()
	if err != nil {
		return nil, fmt.Errorf("handleLiveOrders: failed to get positions: %w", err)
	}

	if !order.IsPending() {
		log.Warnf("handleLiveOrders: order is not pending: %v", order)

		var commits []func() error
		if err := database.CreateTransaction(func(tx *gorm.DB) error {
			var dbCommits []func() error
			var messages []string

			if commit, dbCommit, msg := playground.ResetStatusToPending(order, database); commit != nil {
				commits = append(commits, commit)
				dbCommits = append(dbCommits, dbCommit)
				messages = append(messages, msg)
			}

			for _, o := range order.Reconciles {
				p, err := database.FetchPlayground(o.PlaygroundID)
				if err != nil {
					return fmt.Errorf("handleLiveOrders: failed to fetch playground: %v", err)
				}

				if commit, dbCommit, msg := p.ResetStatusToPending(o, database); commit != nil {
					commits = append(commits, commit)
					dbCommits = append(dbCommits, dbCommit)
					messages = append(messages, msg)
				}
			}

			if len(commits) != len(dbCommits) || len(commits) != len(messages) {
				return fmt.Errorf("handleLiveOrders: mismatched commits and dbCommits: %d != %d", len(commits), len(dbCommits))
			}

			for i := 0; i < len(dbCommits); i++ {
				log.Warnf(messages[i])

				if err := dbCommits[i](); err != nil {
					return fmt.Errorf("handleLiveOrders: failed to db commit: %v", err)
				}
			}

			return nil
		}); err != nil {
			return nil, fmt.Errorf("handleLiveOrders: failed to create transaction: %v", err)
		}

		for _, commit := range commits {
			if err := commit(); err != nil {
				return nil, fmt.Errorf("handleLiveOrders: failed to commit: %v", err)
			}
		}
	}

	newOrder, newTrade, invalidOrder, err := playground.CommitPendingOrder(order, positionCache, orderFillEntry, performChecks)
	if err != nil {
		return nil, fmt.Errorf("handleLiveOrders: failed to commit pending orders: %w", err)
	}

	var resultErr error = nil
	if invalidOrder != nil {
		resultErr = fmt.Errorf("handleLiveOrders: invalid order: %v", invalidOrder)
	}

	if newOrder != nil {
		order = newOrder
	}

	// Resave the order to update the status and close_id
	balance := playground.GetBalance()
	if err := database.SaveOrderRecord(order, &balance, false); err != nil {
		if errors.Is(err, models.ErrDbOrderIsNotOpenOrPending) {
			log.Warnf("handleLiveOrders: order is not open or pending: %v", err)
			return newTrade, nil
		}

		return nil, fmt.Errorf("handleLiveOrders: failed to save order record: %v", err)
	}

	return newTrade, resultErr
}

func commitPendingOrders(database models.IDatabaseService, orderFillEntry models.ExecutionFillRequest) error {
	reconcilePlayground := orderFillEntry.ReconcilePlayground
	order := orderFillEntry.OrderRecord

	if _, err := fillPendingOrder(reconcilePlayground.GetPlayground(), order, orderFillEntry, database); err != nil {
		return fmt.Errorf("handleLiveOrders: failed to fill reconciled order: %v", err)
	}

	return nil
}

func DrainTradierOrderQueue(source *eventmodels.FIFOQueue[*models.TradierOrderUpdateEvent], database models.IDatabaseService) (hasUpdates bool, err error) {
	for {
		event, ok := source.Dequeue()
		if !ok {
			break
		}

		if event.CreateOrder != nil {
			if event.CreateOrder.OrderRecord.Status == models.OrderRecordStatusFilled {
				log.Debugf("handleLiveOrders: order already filled: %v", event.CreateOrder.OrderRecord)
				continue
			}

			hasUpdates = true

			if event.CreateOrder.Order.Status == string(models.OrderRecordStatusFilled) {
				if err := commitPendingOrders(database, models.ExecutionFillRequest{
					ReconcilePlayground: event.CreateOrder.ReconcilePlayground,
					OrderRecord:         event.CreateOrder.OrderRecord,
					Time:                event.CreateOrder.Order.CreateDate,
					Price:               event.CreateOrder.Order.AvgFillPrice,
					Quantity:            event.CreateOrder.Order.GetExecFillQuantity(),
				}); err != nil {
					log.Errorf("handleLiveOrders: failed to commit pending orders: %v", err)
					continue
				}

				log.Debugf("handleLiveOrders: order filled: %v", event.CreateOrder.Order)
			} else if event.CreateOrder.Order.Status == string(models.OrderRecordStatusPending) {
				log.Debugf("handleLiveOrders: order pending: %v", event.CreateOrder.Order)
			} else {
				log.Fatalf("handleLiveOrders: unknown order status: %v", event.CreateOrder.Order.Status)
			}

		} else if event.ModifyOrder != nil {
			hasUpdates = true

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
