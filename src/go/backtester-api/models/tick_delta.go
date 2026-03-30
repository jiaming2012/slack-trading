package models

import (
	"time"

	"github.com/jiaming2012/slack-trading/src/go/eventmodels"
)

type TickDelta struct {
	NewTrades          []*TradeRecord          `json:"new_trades,omitempty"`
	NewCandles         []*BacktesterCandle     `json:"new_candles,omitempty"`
	NewSignals         []*eventmodels.TradeSignal `json:"new_signals,omitempty"`
	InvalidOrders      []*OrderRecord          `json:"invalid_orders,omitempty"`
	Events             []*TickDeltaEvent       `json:"events,omitempty"`
	EquityPlot         *eventmodels.EquityPlot `json:"equity_plot,omitempty"`
	CurrentTime        string                  `json:"current_time"`
	IsBacktestComplete bool                    `json:"is_backtest_complete"`
	// Embedded account state to avoid separate GetAccount RPC per tick.
	Balance    float64              `json:"balance"`
	Equity     float64              `json:"equity"`
	FreeMargin float64              `json:"free_margin"`
	Positions  map[string]*Position `json:"positions,omitempty"`
}

type TickDeltaEvent struct {
	Type                  TickDeltaEventType     `json:"type"`
	LiquidationEvent      *LiquidationEvent      `json:"liquidation_event,omitempty"`
	OptionExpirationEvent *OptionExpirationEvent `json:"expired_option_contract_event,omitempty"`
	OptionAssignmentEvent *OptionAssignmentEvent `json:"option_assignment_event,omitempty"`
}

type TickDeltaEventType string

const (
	TickDeltaEventTypeLiquidation    TickDeltaEventType = "liquidation"
	TickDeltaEventTypeOptionExpired  TickDeltaEventType = "option_expired"
	TickDeltaEventTypeOptionAssigned TickDeltaEventType = "option_assigned"
)

type LiquidationEvent struct {
	OrdersPlaced []*OrderRecord `json:"orders_placed"`
}

type OptionExpirationEvent struct {
	Symbol                  eventmodels.OptionSymbol `json:"symbol"`
	UnderlyingPriceAtExpiry float64                  `json:"underlying_price_at_expiry"`
	Timestamp               time.Time                `json:"timestamp"`
}

type OptionAssignmentEvent struct {
	OrderId          uint                   `json:"order_id"`
	Symbol           eventmodels.Instrument `json:"symbol"`
	AssignedQuantity float64                `json:"assigned_quantity"`
	AssignedPrice    float64                `json:"assignment_price"`
	Timestamp        time.Time              `json:"timestamp"`
}
