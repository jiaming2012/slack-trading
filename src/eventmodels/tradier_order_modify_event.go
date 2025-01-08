package eventmodels

type TradierOrderModifyEvent struct {
	OrderID uint
	Field   string
	Old     interface{}
	New     interface{}
}