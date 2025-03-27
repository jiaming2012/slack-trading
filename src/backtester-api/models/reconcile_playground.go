package models

import (
	"fmt"
	"math"

	"github.com/google/uuid"
)

type ReconcilePlayground struct {
	playground  *Playground
	liveAccount ILiveAccount
}

func (r *ReconcilePlayground) GetPlayground() *Playground {
	return r.playground
}

func (r *ReconcilePlayground) GetLiveAccount() ILiveAccount {
	return r.liveAccount
}

func (r *ReconcilePlayground) GetOrders() []*OrderRecord {
	return r.playground.GetAllOrders()
}

func (r *ReconcilePlayground) SetBroker(broker IBroker) error {
	r.liveAccount.SetBroker(broker)
	return nil
}

func (r *ReconcilePlayground) GetId() uuid.UUID {
	return r.playground.GetId()
}

// func (r *ReconcilePlayground) CommitPendingOrders(orderFillMap map[uint]ExecutionFillRequest) (newTrades []*TradeRecord, invalidOrders []*OrderRecord, err error) {
// 	performChecks := false
// 	positionMap, err := r.playground.GetPositions()
// 	if err != nil {
// 		return nil, nil, fmt.Errorf("ReconcilePlayground: failed to get positions: %w", err)
// 	}

// 	return r.playground.CommitPendingOrders(positionMap, orderFillMap, performChecks)
// }

func (r *ReconcilePlayground) PlaceOrder(order *OrderRecord) ([]*PlaceOrderChanges, []*OrderRecord, error) {
	position, err := r.playground.GetPosition(order.GetInstrument(), false)
	if err != nil {
		return nil, nil, fmt.Errorf("ReconcilePlayground: failed to get position: %w", err)
	}

	// create reconciliation orders
	hasOppositeSides := position.Quantity*order.GetQuantity() < 0

	var orders []*OrderRecord
	if hasOppositeSides {
		if position.Quantity >= 0 {
			switch order.Side {
			case TradierOrderSideSell, TradierOrderSideSellShort:
				sell_qty := math.Min(position.Quantity, order.AbsoluteQuantity)
				if sell_qty > 0 {
					o1 := CopyOrderRecord(r.GetId(), 0, order, LiveAccountTypeReconcilation)
					o1.AbsoluteQuantity = sell_qty
					o1.Side = TradierOrderSideSell
					orders = append(orders, o1)
				}

				if remaining_qty := order.AbsoluteQuantity - sell_qty; remaining_qty > 0 {
					o2 := CopyOrderRecord(r.GetId(), 0, order, LiveAccountTypeReconcilation)
					o2.AbsoluteQuantity = remaining_qty
					o2.Side = TradierOrderSideSellShort
					orders = append(orders, o2)
				}
			default:
				return nil, nil, fmt.Errorf("ReconcilePlayground: invalid order side: %s, with position: %.2f", order.Side, position.Quantity)
			}
		} else {
			switch order.Side {
			case TradierOrderSideBuy, TradierOrderSideBuyToCover:
				buy_qty := math.Min(-position.Quantity, order.AbsoluteQuantity)
				if buy_qty > 0 {
					o1 := CopyOrderRecord(r.GetId(), 0, order, LiveAccountTypeReconcilation)
					o1.AbsoluteQuantity = buy_qty
					o1.Side = TradierOrderSideBuyToCover
					orders = append(orders, o1)
				}

				if remaining_qty := order.AbsoluteQuantity - buy_qty; remaining_qty > 0 {
					o2 := CopyOrderRecord(r.GetId(), 0, order, LiveAccountTypeReconcilation)
					o2.AbsoluteQuantity = remaining_qty
					o2.Side = TradierOrderSideBuy
					orders = append(orders, o2)
				}
			default:
				return nil, nil, fmt.Errorf("ReconcilePlayground: invalid order side: %s, with position: %.2f", order.Side, position.Quantity)
			}
		}
	} else {
		// both position and order quantity have the same sign
		o := CopyOrderRecord(r.GetId(), 0, order, LiveAccountTypeReconcilation)

		switch order.Side {
		case TradierOrderSideBuy, TradierOrderSideSellShort:
			break
		case TradierOrderSideSell:
			o.Side = TradierOrderSideSellShort
		case TradierOrderSideBuyToCover:
			o.Side = TradierOrderSideBuy
		default:
			return nil, nil, fmt.Errorf("ReconcilePlayground: invalid order side: %s, with position: %.2f", order.Side, position.Quantity)
		}

		orders = append(orders, o)
	}

	// place reconciliation orders in playground
	var changes []*PlaceOrderChanges
	for _, o := range orders {
		chg, err := r.playground.placeOrder(o)
		if err != nil {
			return nil, nil, fmt.Errorf("ReconcilePlayground: failed to place order in playground: %w", err)
		}

		changes = append(changes, chg...)
	}

	// send reconciliation orders to market
	for _, o := range orders {
		err = r.liveAccount.PlaceOrder(o)
		if err != nil {
			return nil, nil, fmt.Errorf("ReconcilePlayground: failed to place order: %w", err)
		}
	}

	changes = append(changes, &PlaceOrderChanges{
		Commit: func() error {
			for _, o := range orders {
				o.Reconciles = append(o.Reconciles, order)
			}

			return nil
		},
		Info: "Add ReconciledBy field to orders",
	})

	return changes, orders, nil
}

func NewReconcilePlayground(playground *Playground, liveAccount ILiveAccount) (*ReconcilePlayground, error) {
	reconcilePlayground := &ReconcilePlayground{
		playground:  playground,
		liveAccount: liveAccount,
	}

	playground.SetReconcilePlayground(reconcilePlayground)

	return reconcilePlayground, nil
}
