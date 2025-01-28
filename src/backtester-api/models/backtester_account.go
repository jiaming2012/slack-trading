package models

import (
	"sync"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterAccount struct {
	mutex         *sync.Mutex
	OrderNonce    uint // Used to generate unique order IDs
	TradeNounce   uint // Used to generate unique trade IDs
	Balance       float64
	Orders        []*BacktesterOrder
	PendingOrders []*BacktesterOrder
	EquityPlot    []*eventmodels.EquityPlot
}

func (a *BacktesterAccount) NextOrderID() uint {
	a.OrderNonce++
	return a.OrderNonce - 1
}

func (a *BacktesterAccount) GetActiveOrders() []*BacktesterOrder {
	result := make([]*BacktesterOrder, 0)
	for _, order := range a.Orders {
		status := order.GetStatus()
		if status == BacktesterOrderStatusOpen || status == BacktesterOrderStatusPartiallyFilled {
			result = append(result, order)
		}
	}
	return result
}

func NewBacktesterAccount(balance float64, orders []*BacktesterOrder) *BacktesterAccount {
	var pendingOrders []*BacktesterOrder
	var activeOrders []*BacktesterOrder

	for _, order := range orders {
		if order.Status == BacktesterOrderStatusPending {
			pendingOrders = append(pendingOrders, order)
		} else {
			activeOrders = append(activeOrders, order)
		}
	}

	return &BacktesterAccount{
		mutex:         &sync.Mutex{},
		Balance:       balance,
		Orders:        activeOrders,
		PendingOrders: pendingOrders,
	}
}
