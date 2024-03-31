package eventmodels

type AutoExecuteTrade struct {
	BaseRequestEvent2
	Trade *Trade
}
