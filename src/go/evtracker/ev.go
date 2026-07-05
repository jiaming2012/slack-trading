package evtracker

// EVStats holds the expected-value breakdown for a set of closed trades. Counts
// classify each trade as a win (pnl > 0), loss (pnl < 0), or breakeven
// (pnl == 0). Breakeven trades are excluded from Decided, WinRate, LossRate,
// AvgWin, and AvgLoss.
type EVStats struct {
	WinRate   float64 // wins / decided (0 when no decided trades)
	AvgWin    float64 // mean pnl of winning trades (0 when no winners)
	LossRate  float64 // losses / decided (0 when no decided trades)
	AvgLoss   float64 // mean absolute pnl of losing trades, a positive magnitude
	EV        float64 // (WinRate * AvgWin) - (LossRate * AvgLoss)
	Wins      int
	Losses    int
	Breakeven int
	Decided   int // Wins + Losses
}

// ComputeEV computes the expected value of a set of closed trades using
// EV = (win_rate * avg_win) - (loss_rate * avg_loss). Winners have pnl > 0,
// losers pnl < 0, and breakeven (pnl == 0) trades are excluded from the decided
// population. avg_loss is a positive magnitude. When there are no decided trades
// all rates and EV are zero. The function is pure: identical input yields
// identical output with no external state.
func ComputeEV(trades []TradeOutcome) EVStats {
	var stats EVStats
	var sumWin, sumLossAbs float64

	for _, t := range trades {
		switch {
		case t.PnL > 0:
			stats.Wins++
			sumWin += t.PnL
		case t.PnL < 0:
			stats.Losses++
			sumLossAbs += -t.PnL
		default:
			stats.Breakeven++
		}
	}

	stats.Decided = stats.Wins + stats.Losses
	if stats.Decided == 0 {
		return stats
	}

	decided := float64(stats.Decided)
	stats.WinRate = float64(stats.Wins) / decided
	stats.LossRate = float64(stats.Losses) / decided

	if stats.Wins > 0 {
		stats.AvgWin = sumWin / float64(stats.Wins)
	}
	if stats.Losses > 0 {
		stats.AvgLoss = sumLossAbs / float64(stats.Losses)
	}

	stats.EV = (stats.WinRate * stats.AvgWin) - (stats.LossRate * stats.AvgLoss)
	return stats
}
