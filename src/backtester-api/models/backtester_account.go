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
	Orders        []*OrderRecord
	PendingOrders []*OrderRecord
	NewOrders     []*OrderRecord
	EquityPlot    []*eventmodels.EquityPlot
}

func (a *BacktesterAccount) NextOrderID() uint {
	a.OrderNonce++
	return a.OrderNonce - 1
}

func NewBacktesterAccount(balance float64, orders []*OrderRecord) *BacktesterAccount {
	var pendingOrders []*OrderRecord
	var activeOrders []*OrderRecord
	var newOrders []*OrderRecord

	for _, order := range orders {
		if order.Status == OrderRecordStatusPending {
			pendingOrders = append(pendingOrders, order)
		} else if order.Status == OrderRecordStatusNew {
			newOrders = append(newOrders, order)
		} else {
			activeOrders = append(activeOrders, order)
		}
	}

	return &BacktesterAccount{
		mutex:         &sync.Mutex{},
		Balance:       balance,
		Orders:        activeOrders,
		PendingOrders: pendingOrders,
		NewOrders:     newOrders,
	}
}
