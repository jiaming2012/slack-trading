package models

type BacktesterAccount struct {
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