package costmodel

import "math"

// EstimateCapacity estimates the maximum position size, in shares, a
// strategy can carry before its own square-root market impact consumes
// MaxImpactFractionOfEdge of its expected per-share edge.
//
// Market impact is modeled as:
//
//	impact_coefficient * price * sqrt(shares / avg_daily_volume)
//
// per share. EstimateCapacity solves for the share count at which that
// impact equals max_impact_fraction_of_edge * edge_per_share, using the
// closed-form solution:
//
//	max_shares = avg_daily_volume * (max_impact_fraction_of_edge * edge_per_share / (impact_coefficient * price))^2
//
// EstimateCapacity returns 0 — never an error, never a negative number —
// whenever edgePerShare, price, or avgDailyVolume is non-positive.
func EstimateCapacity(edgePerShare, price, avgDailyVolume float64, params CostParams) float64 {
	if edgePerShare <= 0 || price <= 0 || avgDailyVolume <= 0 {
		return 0
	}

	ratio := (params.MaxImpactFractionOfEdge * edgePerShare) / (params.ImpactCoefficient * price)
	return avgDailyVolume * math.Pow(ratio, 2)
}
