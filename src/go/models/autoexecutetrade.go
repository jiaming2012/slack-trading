package models

type AutoExecuteTrade struct {
	BaseRequestEvent
	Trade *Trade
}
