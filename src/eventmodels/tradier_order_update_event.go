package eventmodels

type TradierOrderUpdateEvent struct {
	OrderID uint
	Field   string
	Old     interface{}
	New     interface{}
}
