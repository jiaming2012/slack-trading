package models

type StateChange struct {
	NewTrades          []*BacktesterTrade
	NewCandles         []*BacktesterCandle
	IsBacktestComplete bool
}
