package costmodel

// TradeSideLong and TradeSideShort are the two allowed values for
// TradeInput.Side.
const (
	TradeSideLong  = "long"
	TradeSideShort = "short"
)

// TradeInput describes a single closed trade in the terms TradeCost and
// ComputeEV need: which direction it was traded, its entry/exit prices,
// size, and how long the position was held.
type TradeInput struct {
	// Side is "long" or "short". Only "short" trades accrue a borrow cost.
	Side string
	// EntryPrice is the fill price at trade entry.
	EntryPrice float64
	// ExitPrice is the fill price at trade exit.
	ExitPrice float64
	// Quantity is the number of shares traded (always positive).
	Quantity float64
	// HoldDays is the number of days the position was held, used for the
	// short-borrow accrual (hold_days / 360, an Actual-360 convention).
	HoldDays float64
}

// GrossPnL returns the trade's cost-blind profit/loss: (exit - entry) *
// quantity for a long trade, (entry - exit) * quantity for a short trade.
func (t TradeInput) GrossPnL() float64 {
	if t.Side == TradeSideShort {
		return (t.EntryPrice - t.ExitPrice) * t.Quantity
	}
	return (t.ExitPrice - t.EntryPrice) * t.Quantity
}

// TradeCost computes the total estimated transaction cost of a single closed
// trade as the sum of a commission cost, a spread cost, and — only for short
// trades — a borrow cost. For long trades the borrow cost component is
// exactly zero.
func TradeCost(trade TradeInput, params CostParams) float64 {
	commissionCost := params.CommissionPerTrade + params.CommissionPerShare*trade.Quantity
	spreadCost := (params.AvgSpreadBps / 10000) * trade.EntryPrice * trade.Quantity

	var borrowCost float64
	if trade.Side == TradeSideShort {
		borrowCost = (params.BorrowRateBpsAnnual / 10000) * trade.EntryPrice * trade.Quantity * (trade.HoldDays / 360)
	}

	return commissionCost + spreadCost + borrowCost
}
