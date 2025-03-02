package models

import (
	"fmt"
	"math"

	"github.com/google/uuid"
)

type ReconcilePlayground struct {
	playground *Playground
	// newTradesQueue *eventmodels.FIFOQueue[*TradeRecord]
}

func (r *ReconcilePlayground) GetOrders() []*BacktesterOrder {
	return r.playground.GetOrders()
}

func (r *ReconcilePlayground) SetBroker(broker IBroker) error {
	if r.playground.Broker != nil {
		if broker == r.playground.Broker {
			return nil
		}

		return fmt.Errorf("ReconcilePlayground: cannot change broker once set")
	}

	r.playground.SetBroker(broker)
	return nil
}

// todo: remove this??
func (r *ReconcilePlayground) GetPlayground() *Playground {
	return r.playground
}

func (r *ReconcilePlayground) GetId() uuid.UUID {
	return r.playground.GetId()
}

// func (r *ReconcilePlayground) CommitPendingOrders(orderFillMap map[uint]ExecutionFillRequest) (newTrades []*TradeRecord, invalidOrders []*BacktesterOrder, err error) {
// 	performChecks := false
// 	positionMap, err := r.playground.GetPositions()
// 	if err != nil {
// 		return nil, nil, fmt.Errorf("ReconcilePlayground: failed to get positions: %w", err)
// 	}

// 	return r.playground.CommitPendingOrders(positionMap, orderFillMap, performChecks)
// }

func (r *ReconcilePlayground) PlaceOrder(liveAccount ILiveAccount, order *BacktesterOrder) ([]*PlaceOrderChanges, []*BacktesterOrder, error) {
	position, err := r.playground.GetPosition(order.Symbol, false)
	if err != nil {
		return nil, nil, fmt.Errorf("ReconcilePlayground: failed to get position: %w", err)
	}

	// create reconciliation orders
	hasOppositeSides := position.Quantity*order.GetQuantity() < 0

	var orders []*BacktesterOrder
	if hasOppositeSides {
		if position.Quantity >= 0 {
			switch order.Side {
			case TradierOrderSideSell, TradierOrderSideSellShort:
				sell_qty := math.Min(position.Quantity, order.AbsoluteQuantity)
				if sell_qty > 0 {
					o1 := CopyBacktesterOrder(order)
					o1.AbsoluteQuantity = sell_qty
					o1.Side = TradierOrderSideSell
					orders = append(orders, o1)
				}

				if remaining_qty := order.AbsoluteQuantity - sell_qty; remaining_qty > 0 {
					o2 := CopyBacktesterOrder(order)
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
					o1 := CopyBacktesterOrder(order)
					o1.AbsoluteQuantity = buy_qty
					o1.Side = TradierOrderSideBuyToCover
					orders = append(orders, o1)
				}

				if remaining_qty := order.AbsoluteQuantity - buy_qty; remaining_qty > 0 {
					o2 := CopyBacktesterOrder(order)
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
		o := CopyBacktesterOrder(order)

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
		chg, err := r.playground.PlaceOrder(o)
		if err != nil {
			return nil, nil, fmt.Errorf("ReconcilePlayground: failed to place order in playground: %w", err)
		}

		changes = append(changes, chg...)
	}

	// send reconciliation orders to market
	for _, o := range orders {
		err = liveAccount.PlaceOrder(o)
		if err != nil {
			return nil, nil, fmt.Errorf("ReconcilePlayground: failed to place order: %w", err)
		}
	}

	return changes, orders, nil
}

func NewReconcilePlayground(playground *Playground) (*ReconcilePlayground, error) {
	return &ReconcilePlayground{
		playground: playground,
		// newTradesQueue: newTradesQueue,
	}, nil
}
