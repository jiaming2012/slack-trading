package costmodel

import "testing"

// fixtureParams mirrors the CostParams used in the spec's trade-cost
// scenarios: commission_per_trade=1.00, commission_per_share=0.01,
// avg_spread_bps=20, borrow_rate_bps_annual=360.
func fixtureParams() CostParams {
	return CostParams{
		CommissionPerTrade:  1.00,
		CommissionPerShare:  0.01,
		AvgSpreadBps:        20,
		BorrowRateBpsAnnual: 360,
	}
}

// TestTradeCost_LongHasNoBorrowComponent pins the spec's long-trade
// scenario: entry 100, quantity 100, hold 2 days -> commission 2.00 +
// spread 20.00 + borrow 0.00 = 22.00.
func TestTradeCost_LongHasNoBorrowComponent(t *testing.T) {
	trade := TradeInput{
		Side:       TradeSideLong,
		EntryPrice: 100,
		ExitPrice:  105, // exit price is irrelevant to TradeCost, only to GrossPnL
		Quantity:   100,
		HoldDays:   2,
	}

	got := TradeCost(trade, fixtureParams())
	want := 22.00
	if got != want {
		t.Fatalf("TradeCost(long) = %v, want %v", got, want)
	}
}

// TestTradeCost_ShortAccruesBorrowOverHoldDays pins the spec's short-trade
// scenario: entry 10.00, quantity 1000, hold 300 days -> commission 11.00 +
// spread 20.00 + borrow 300.00 = 331.00.
func TestTradeCost_ShortAccruesBorrowOverHoldDays(t *testing.T) {
	trade := TradeInput{
		Side:       TradeSideShort,
		EntryPrice: 10.00,
		ExitPrice:  9.9,
		Quantity:   1000,
		HoldDays:   300,
	}

	got := TradeCost(trade, fixtureParams())
	want := 331.00
	if got != want {
		t.Fatalf("TradeCost(short) = %v, want %v", got, want)
	}
}

// TestTradeCost_ZeroHoldDaysShort asserts the boundary case where a short
// trade is held zero days: the borrow component must be exactly zero even
// though the trade is short, since hold_days / 360 == 0.
func TestTradeCost_ZeroHoldDaysShort(t *testing.T) {
	trade := TradeInput{
		Side:       TradeSideShort,
		EntryPrice: 10.00,
		ExitPrice:  9.9,
		Quantity:   1000,
		HoldDays:   0,
	}

	got := TradeCost(trade, fixtureParams())
	want := 31.00 // commission 11.00 + spread 20.00 + borrow 0.00
	if got != want {
		t.Fatalf("TradeCost(short, hold_days=0) = %v, want %v", got, want)
	}
}
