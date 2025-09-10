package models

import "github.com/jiaming2012/slack-trading/src/eventmodels"

type TickDelta struct {
	NewTrades          []*TradeRecord          `json:"new_trades,omitempty"`
	NewCandles         []*BacktesterCandle     `json:"new_candles,omitempty"`
	InvalidOrders      []*OrderRecord          `json:"invalid_orders,omitempty"`
	Events             []*TickDeltaEvent       `json:"events,omitempty"`
	EquityPlot         *eventmodels.EquityPlot `json:"equity_plot,omitempty"`
	CurrentTime        string                  `json:"current_time"`
	IsBacktestComplete bool                    `json:"is_backtest_complete"`
}

type TickDeltaEvent struct {
	Type                       TickDeltaEventType          `json:"type"`
	LiquidationEvent           *LiquidationEvent           `json:"liquidation_event,omitempty"`
	ExpiredOptionContractEvent *ExpiredOptionContractEvent `json:"expired_option_contract_event,omitempty"`
}

type TickDeltaEventType string

const (
	TickDeltaEventTypeLiquidation   TickDeltaEventType = "liquidation"
	TickDeltaEventTypeOptionExpired TickDeltaEventType = "option_expired"
)

type LiquidationEvent struct {
	OrdersPlaced []*OrderRecord `json:"orders_placed"`
}

type ExpiredOptionContractEvent struct {
	Symbol                  eventmodels.OptionSymbol `json:"symbol"`
	UnderlyingPriceAtExpiry float64                  `json:"underlying_price_at_expiry"`
}
