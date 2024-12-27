package models

import "sync"

type BacktesterAccount struct {
	mutex         *sync.Mutex
	OrderNonce    uint // Used to generate unique order IDs
	TradeNounce   uint // Used to generate unique trade IDs
	Balance       float64
	Orders        []*BacktesterOrder
	PendingOrders []*BacktesterOrder
}

func (a *BacktesterAccount) NextOrderID() uint {
	a.OrderNonce++
	return a.OrderNonce - 1
}

func (a *BacktesterAccount) NextTradeID() uint {
	a.TradeNounce++
	return a.TradeNounce - 1
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
