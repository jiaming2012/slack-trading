package eventmodels

type TradeDuration string

const (
	TradeDurationDay   TradeDuration = "day"
	TradeDurationGoodTillCancelled  TradeDuration = "gtc"
	TradeDurationPreMarket  TradeDuration = "pre"
	TradeDurationPostMarket  TradeDuration = "post"
)