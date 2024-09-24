package models

type BacktesterOrderDuration string

const (
	Day        BacktesterOrderDuration = "day"
	GTC        BacktesterOrderDuration = "gtc"
	PreMarket  BacktesterOrderDuration = "pre"
	PostMarket BacktesterOrderDuration = "post"
)
