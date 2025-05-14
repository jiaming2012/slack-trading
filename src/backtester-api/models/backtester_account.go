package models

import (
	"sync"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type BacktesterAccount struct {
	mutex       *sync.Mutex
	OrderNonce  uint // Used to generate unique order IDs
	TradeNounce uint // Used to generate unique trade IDs
	Balance     float64
	EquityPlot  []*eventmodels.EquityPlot
}

func (a *BacktesterAccount) NextOrderID() uint {
	a.OrderNonce++
	return a.OrderNonce - 1
}

func NewBacktesterAccount(balance float64) *BacktesterAccount {
	return &BacktesterAccount{
		mutex:   &sync.Mutex{},
		Balance: balance,
	}
}
