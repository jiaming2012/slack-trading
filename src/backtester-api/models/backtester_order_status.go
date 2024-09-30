package models

type BacktesterOrderStatus string

const (
	BacktesterOrderStatusOpen            BacktesterOrderStatus = "open"
	BacktesterOrderStatusPending         BacktesterOrderStatus = "pending"
	BacktesterOrderStatusPartiallyFilled BacktesterOrderStatus = "partially_filled"
	BacktesterOrderStatusFilled          BacktesterOrderStatus = "filled"
	BacktesterOrderStatusExpired         BacktesterOrderStatus = "expired"
	BacktesterOrderStatusCancelled       BacktesterOrderStatus = "cancelled"
	BacktesterOrderStatusRejected        BacktesterOrderStatus = "rejected"
)

func (status BacktesterOrderStatus) IsTradingAllowed() bool {
	return status == BacktesterOrderStatusPending || status == BacktesterOrderStatusOpen || status == BacktesterOrderStatusPartiallyFilled
}

func (status BacktesterOrderStatus) IsFilled() bool {
	return status == BacktesterOrderStatusFilled || status == BacktesterOrderStatusPartiallyFilled
}
