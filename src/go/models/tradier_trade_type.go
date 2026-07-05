package models

type TradierOrderType string

const (
	TradierOrderTypeMarket TradierOrderType = "market"
	TradierOrderTypeDebit  TradierOrderType = "debit"
	TradierOrderTypeCredit TradierOrderType = "credit"
	TradierOrderTypeEven   TradierOrderType = "even"
)
