package costmodel

import "testing"

const evEpsilon = 0.005

func approxEqual(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= epsilon
}

// TestComputeEV_GrossWinNetLossFlip pins the spec's three-trade fixture:
//   - trade 1: long win, gross +500, net +478 (entry 100, exit 105, qty 100,
//     hold 2 -> TradeCost 22.00, matching the long-trade cost scenario)
//   - trade 2: long loss, gross -200, net -223 (entry 50, exit 49, qty 200
//     -> TradeCost 23.00: commission 3.00 + spread 20.00)
//   - trade 3: short, gross +100, net -231 (entry 10, exit 9.9, qty 1000,
//     hold 300 -> TradeCost 331.00, matching the short-trade cost scenario)
//
// GrossEV classifies trade 3 as a win (2 wins, 1 loss) -> GrossEV = 133.33.
// NetEV classifies trade 3 as a loss (1 win, 2 losses) -> NetEV = 8.00.
func TestComputeEV_GrossWinNetLossFlip(t *testing.T) {
	params := fixtureParams()

	trades := []TradeInput{
		{Side: TradeSideLong, EntryPrice: 100, ExitPrice: 105, Quantity: 100, HoldDays: 2},
		{Side: TradeSideLong, EntryPrice: 50, ExitPrice: 49, Quantity: 200, HoldDays: 2},
		{Side: TradeSideShort, EntryPrice: 10, ExitPrice: 9.9, Quantity: 1000, HoldDays: 300},
	}

	// Sanity-check the fixture's underlying gross/net P&L values match the
	// spec's stated numbers before asserting on the aggregate EV.
	wantGross := []float64{500, -200, 100}
	wantNet := []float64{478, -223, -231}
	for i, trade := range trades {
		gross := trade.GrossPnL()
		if !approxEqual(gross, wantGross[i], evEpsilon) {
			t.Fatalf("trade %d GrossPnL = %v, want %v", i, gross, wantGross[i])
		}
		net := gross - TradeCost(trade, params)
		if !approxEqual(net, wantNet[i], evEpsilon) {
			t.Fatalf("trade %d net P&L = %v, want %v", i, net, wantNet[i])
		}
	}

	result := ComputeEV(trades, params)

	if !approxEqual(result.GrossEV, 133.33, evEpsilon) {
		t.Fatalf("GrossEV = %v, want 133.33", result.GrossEV)
	}
	if !approxEqual(result.NetEV, 8.00, evEpsilon) {
		t.Fatalf("NetEV = %v, want 8.00", result.NetEV)
	}
}

// TestComputeEV_EmptyTradeSetIsZero asserts an empty trade slice returns
// GrossEV = 0 and NetEV = 0 without panicking or dividing by zero.
func TestComputeEV_EmptyTradeSetIsZero(t *testing.T) {
	result := ComputeEV(nil, fixtureParams())

	if result.GrossEV != 0 {
		t.Fatalf("GrossEV = %v, want 0", result.GrossEV)
	}
	if result.NetEV != 0 {
		t.Fatalf("NetEV = %v, want 0", result.NetEV)
	}
}

// TestComputeEV_ZeroPnLTradeClassifiedAsLoss is a boundary-condition test:
// a trade with exactly zero P&L (the win/loss threshold) must be classified
// as a loss, not a win, per expectedValue's documented convention.
func TestComputeEV_ZeroPnLTradeClassifiedAsLoss(t *testing.T) {
	params := CostParams{} // zero-cost params so gross == net exactly

	trades := []TradeInput{
		{Side: TradeSideLong, EntryPrice: 100, ExitPrice: 100, Quantity: 10, HoldDays: 0}, // gross P&L == 0
	}

	result := ComputeEV(trades, params)

	// A single zero-P&L trade classified as a loss contributes:
	// win_rate=0, loss_rate=1, avg_loss=0 -> EV = 0*0 - 1*0 = 0.
	if result.GrossEV != 0 {
		t.Fatalf("GrossEV = %v, want 0 (zero-P&L trade classified as loss)", result.GrossEV)
	}
	if result.NetEV != 0 {
		t.Fatalf("NetEV = %v, want 0", result.NetEV)
	}
}
