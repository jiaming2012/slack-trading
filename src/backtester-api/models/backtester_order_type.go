package models

type BacktesterOrderType string

const (
	Market    BacktesterOrderType = "market"
	Limit     BacktesterOrderType = "limit"
	Stop      BacktesterOrderType = "stop"
	StopLimit BacktesterOrderType = "stop_limit"
)
