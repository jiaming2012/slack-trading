package models

import "sync"

type BacktesterAccount struct {
	mutex         *sync.Mutex
	Balance       float64
	Orders        []*BacktesterOrder
	PendingOrders []*BacktesterOrder
}

func (a *BacktesterAccount) GetActiveOrders() []*BacktesterOrder {
	result := make([]*BacktesterOrder, 0)
	for _, order := range a.Orders {
		if order.GetStatus() == BacktesterOrderStatusOpen {
			result = append(result, order)
		}
	}
	return result
}

func NewBacktesterAccount(balance float64) *BacktesterAccount {
	return &BacktesterAccount{
		mutex:   &sync.Mutex{},
		Balance: balance,
	}
}
