package models

type StateChange struct {
	NewTrades          []*BacktesterTrade
	IsBacktestComplete bool
}
