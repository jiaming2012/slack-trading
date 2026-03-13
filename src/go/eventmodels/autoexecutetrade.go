package eventmodels

type AutoExecuteTrade struct {
	BaseRequestEvent
	Trade *Trade
}
