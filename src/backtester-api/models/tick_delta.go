package models

type TickDelta struct {
	NewTrades          []*BacktesterTrade  `json:"new_trades,omitempty"`
	NewCandles         []*BacktesterCandle `json:"new_candles,omitempty"`
	InvalidOrders      []*BacktesterOrder  `json:"invalid_orders,omitempty"`
	Events             []*TickDeltaEvent   `json:"events,omitempty"`
	CurrentTime        string              `json:"current_time"`
	IsBacktestComplete bool                `json:"is_backtest_complete"`
}

type TickDeltaEvent struct {
	Type             TickDeltaEventType `json:"type"`
	LiquidationEvent *LiquidationEvent  `json:"liquidation_event,omitempty"`
}

type TickDeltaEventType string

const (
	TickDeltaEventTypeLiquidation TickDeltaEventType = "liquidation"
)

type LiquidationEvent struct {
	OrdersPlaced []*BacktesterOrder `json:"orders_placed"`
}
