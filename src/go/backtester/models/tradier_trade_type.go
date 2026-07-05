package models

type TradierOrderType string

const (
	TradierOrderTypeMarket TradierOrderType = "market"
	TradierOrderTypeStop   TradierOrderType = "stop"
	TradierOrderTypeDebit  TradierOrderType = "debit"
	TradierOrderTypeCredit TradierOrderType = "credit"
	TradierOrderTypeEven   TradierOrderType = "even"
)
