package eventmodels

import (
	"fmt"
	"math"
)

func CalculateUnrealizedPL(vwap Vwap, vol Volume, tick Tick) float64 {
	if vol > 0 {
		return (tick.Price - float64(vwap)) * float64(vol)
	} else if vol < 0 {
		return (float64(vwap) - tick.Price) * math.Abs(float64(vol))
	} else {
		return 0
	}
}

func PriceLevelProfitLossAboveZeroConstraint(priceLevel *PriceLevel, _ *ExitCondition, params map[string]interface{}) (bool, error) {
	tick := params["tick"].(Tick)
	stats, err := priceLevel.Trades.GetTradeStats(tick)
	if err != nil {
		return false, fmt.Errorf("PriceLevelProfitLossAboveZeroConstraint: failed to get trade stats: %w", err)
	}

	pl := stats.RealizedPL + stats.FloatingPL

	return pl > 0, nil
}
