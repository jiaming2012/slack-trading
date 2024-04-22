package eventmodels

type TradierOrderUpdateEvent struct {
	OrderID uint64
	Field   string
	Old     interface{}
	New     interface{}
}
