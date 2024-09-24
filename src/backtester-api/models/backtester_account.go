package models

type BacktesterAccount struct {
	Balance       float64
	Orders        []*BacktesterOrder
	PendingOrders []*BacktesterOrder
}
