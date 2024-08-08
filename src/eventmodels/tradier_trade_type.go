package eventmodels

type TradierTradeType string

const (
	TradierTradeTypeMarket TradierTradeType = "market"
	TradierTradeTypeDebit  TradierTradeType = "debit"
	TradierTradeTypeCredit TradierTradeType = "credit"
	TradierTradeTypeEven   TradierTradeType = "even"
)
