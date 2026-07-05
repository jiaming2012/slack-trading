package costmodel

// EVResult carries both the cost-blind and cost-adjusted expected-value
// figures for a set of closed trades.
//
// NetEV is the ONLY field downstream EV-weight and retirement-signal logic
// (e.g. the ev-tracker aggregation pipeline) is contractually permitted to
// use for decisions. GrossEV is retained solely for display and diagnostic
// comparison against NetEV — using it to drive a retirement or sizing
// decision would silently ignore real trading costs.
type EVResult struct {
	// GrossEV is the expected value computed from each trade's cost-blind
	// P&L and win/loss classification. Diagnostic/display only.
	GrossEV float64
	// NetEV is the expected value computed from each trade's cost-adjusted
	// P&L (gross P&L - TradeCost) and win/loss classification. This is the
	// only value downstream decision logic may use.
	NetEV float64
}

// ComputeEV computes GrossEV and NetEV over a set of closed trades. Win/loss
// classification is performed independently for each: a trade is classified
// off its gross P&L for GrossEV, and off its net P&L (gross P&L - TradeCost)
// for NetEV. A trade that is gross-profitable but net-unprofitable therefore
// counts as a win toward GrossEV and as a loss toward NetEV. An empty trade
// slice returns a zero-valued EVResult without dividing by zero.
func ComputeEV(trades []TradeInput, params CostParams) EVResult {
	grossPnLs := make([]float64, len(trades))
	netPnLs := make([]float64, len(trades))

	for i, trade := range trades {
		gross := trade.GrossPnL()
		grossPnLs[i] = gross
		netPnLs[i] = gross - TradeCost(trade, params)
	}

	return EVResult{
		GrossEV: expectedValue(grossPnLs),
		NetEV:   expectedValue(netPnLs),
	}
}

// expectedValue computes win_rate * avg_win - loss_rate * avg_loss over a
// slice of signed P&L values, classifying a value > 0 as a win and a value
// <= 0 as a loss. Returns 0 for an empty slice. This is arithmetically
// equivalent to the mean of the signed P&L values.
func expectedValue(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}

	var winSum, lossSum float64
	var winCount, lossCount int

	for _, pnl := range pnls {
		if pnl > 0 {
			winSum += pnl
			winCount++
		} else {
			lossSum += -pnl // loss magnitude, stored positive
			lossCount++
		}
	}

	n := float64(len(pnls))
	winRate := float64(winCount) / n
	lossRate := float64(lossCount) / n

	var avgWin, avgLoss float64
	if winCount > 0 {
		avgWin = winSum / float64(winCount)
	}
	if lossCount > 0 {
		avgLoss = lossSum / float64(lossCount)
	}

	return winRate*avgWin - lossRate*avgLoss
}
