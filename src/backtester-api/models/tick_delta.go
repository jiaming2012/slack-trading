package models

type TickDelta struct {
	NewTrades          []*BacktesterTrade  `json:"new_trades,omitempty"`
	NewCandles         []*BacktesterCandle `json:"new_candles,omitempty"`
	InvalidOrders      []*BacktesterOrder  `json:"invalid_orders,omitempty"`
	CurrentTime        string              `json:"current_time"`
	IsBacktestComplete bool                `json:"is_backtest_complete"`
}
