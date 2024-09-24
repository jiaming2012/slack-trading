package models

type BacktesterOrderStatus string

const (
	BacktesterOrderStatusOpen            BacktesterOrderStatus = "open"
	BacktesterOrderStatusPartiallyFilled BacktesterOrderStatus = "partially_filled"
	BacktesterOrderStatusFilled          BacktesterOrderStatus = "filled"
	BacktesterOrderStatusExpired         BacktesterOrderStatus = "expired"
	BacktesterOrderStatusCancelled       BacktesterOrderStatus = "cancelled"
	BacktesterOrderStatusRejected        BacktesterOrderStatus = "rejected"
)

func (status BacktesterOrderStatus) IsTradeable() bool {
	return status == BacktesterOrderStatusOpen || status == BacktesterOrderStatusPartiallyFilled
}
