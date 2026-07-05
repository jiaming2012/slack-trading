package riskoverlay

import (
	"fmt"
	"math"
)

// LimitType is the typed identity of a limit family. Every LimitBreach carries
// exactly one.
type LimitType string

const (
	LimitGrossExposure       LimitType = "gross_exposure"
	LimitNetExposure         LimitType = "net_exposure"
	LimitSectorConcentration LimitType = "sector_concentration"
	LimitDrawdownBreaker     LimitType = "drawdown_breaker"
	LimitStrategyAllocation  LimitType = "strategy_allocation"
	LimitCrowding            LimitType = "crowding"
)

// LimitBreach records a single limit violation with a human-readable reason.
type LimitBreach struct {
	Type   LimitType
	Reason string
}

// Decision is the result of Evaluate. Allowed is true if and only if Breaches
// is empty.
type Decision struct {
	Allowed  bool
	Breaches []LimitBreach
}

// HasBreach reports whether the decision contains a breach of the given type.
func (d Decision) HasBreach(t LimitType) bool {
	for _, b := range d.Breaches {
		if b.Type == t {
			return true
		}
	}
	return false
}

// Evaluate is the pure limit-evaluation engine. Given a portfolio snapshot, one
// proposed order, the configured limits, and the current scan cycle's crowding
// view, it returns a Decision with every breach the order would cause. It
// performs no I/O of any kind.
//
// Reduction orders short-circuit to Allowed before any limit is checked — a
// risk-reducing order is never blocked, even while the drawdown circuit breaker
// is halting entries. For entries, every limit family is checked and all
// breaches are accumulated (the engine never early-returns on the first
// breach), so a caller sees the complete set of reasons.
func Evaluate(state PortfolioState, order ProposedOrder, limits RiskLimits, crowding CrowdingView) (Decision, error) {
	// Invariant: risk-reducing orders ALWAYS pass. This must precede every
	// limit check, including the drawdown circuit breaker.
	if order.IsReduction {
		return Decision{Allowed: true}, nil
	}

	var breaches []LimitBreach

	// Current exposures over existing positions.
	currentGross := 0.0
	currentNet := 0.0
	for _, p := range state.Positions {
		currentGross += math.Abs(p.SignedNotional)
		currentNet += p.SignedNotional
	}

	orderAbs := math.Abs(order.SignedNotional)
	resultingGross := currentGross + orderAbs
	resultingNet := currentNet + order.SignedNotional

	// Gross exposure: resulting gross strictly above the cap is rejected;
	// exactly at the cap is allowed.
	if resultingGross > limits.MaxGrossExposure {
		breaches = append(breaches, LimitBreach{
			Type:   LimitGrossExposure,
			Reason: fmt.Sprintf("resulting gross exposure %.2f exceeds max %.2f", resultingGross, limits.MaxGrossExposure),
		})
	}

	// Net exposure: absolute resulting net strictly above the cap is rejected.
	if math.Abs(resultingNet) > limits.MaxNetExposure {
		breaches = append(breaches, LimitBreach{
			Type:   LimitNetExposure,
			Reason: fmt.Sprintf("resulting net exposure %.2f exceeds max %.2f", math.Abs(resultingNet), limits.MaxNetExposure),
		})
	}

	// Sector concentration: the order's sector share of resulting gross
	// exposure strictly above the limit is rejected. Only the order's sector
	// can newly breach — an entry increases exactly one sector's numerator and
	// the shared denominator, so every other sector's share strictly decreases.
	if resultingGross > 0 && order.Sector != "" {
		sectorAbs := orderAbs
		for _, p := range state.Positions {
			if p.Sector == order.Sector {
				sectorAbs += math.Abs(p.SignedNotional)
			}
		}
		sectorSharePct := sectorAbs / resultingGross * 100
		if sectorSharePct > limits.MaxSectorConcentrationPct {
			breaches = append(breaches, LimitBreach{
				Type:   LimitSectorConcentration,
				Reason: fmt.Sprintf("sector %q share %.2f%% exceeds max %.2f%%", order.Sector, sectorSharePct, limits.MaxSectorConcentrationPct),
			})
		}
	}

	// Drawdown circuit breaker: halt entries when trailing drawdown strictly
	// exceeds the limit. Reductions never reach here (short-circuited above).
	if dd, ok := drawdownPct(state.EquitySeries); ok && dd > limits.MaxDrawdownPct {
		breaches = append(breaches, LimitBreach{
			Type:   LimitDrawdownBreaker,
			Reason: fmt.Sprintf("trailing drawdown %.2f%% exceeds max %.2f%%; entries halted", dd, limits.MaxDrawdownPct),
		})
	}

	// Per-strategy EV-weighted allocation cap. Skipped entirely when no
	// EV-weight data participates (empty set or non-positive total), which
	// avoids a divide-by-zero and treats "no EV data" as unconstrained. When
	// data participates, a strategy absent from the set has weight zero and
	// therefore a zero cap, so any positive deployment is rejected.
	if sumWeights := sumWeights(state.EvWeights); sumWeights > 0 {
		weight := state.EvWeights[order.StrategyID] // zero if absent
		allocCap := limits.DeployableCapital * weight / sumWeights
		resultingDeployed := state.StrategyDeployed[order.StrategyID] + orderAbs
		if resultingDeployed > allocCap {
			breaches = append(breaches, LimitBreach{
				Type:   LimitStrategyAllocation,
				Reason: fmt.Sprintf("strategy %q resulting allocation %.2f exceeds cap %.2f (ev weight %.4f of %.4f)", order.StrategyID, resultingDeployed, allocCap, weight, sumWeights),
			})
		}
	}

	// Crowding consumption: reject an entry into a flagged-crowded ticker when
	// configured to do so.
	if limits.RejectCrowdedEntries && crowding.IsFlagged(order.Ticker) {
		breaches = append(breaches, LimitBreach{
			Type:   LimitCrowding,
			Reason: fmt.Sprintf("ticker %q is flagged crowded for the current scan cycle", order.Ticker),
		})
	}

	return Decision{Allowed: len(breaches) == 0, Breaches: breaches}, nil
}

// drawdownPct computes (peak-current)/peak*100 over the trailing equity series,
// where peak is the maximum equity in the window and current is the last
// element. The second return is false when drawdown cannot be computed (empty
// series or a non-positive peak), in which case the breaker never trips.
func drawdownPct(equity []float64) (float64, bool) {
	if len(equity) == 0 {
		return 0, false
	}
	peak := equity[0]
	for _, e := range equity {
		if e > peak {
			peak = e
		}
	}
	if peak <= 0 {
		return 0, false
	}
	current := equity[len(equity)-1]
	return (peak - current) / peak * 100, true
}

// sumWeights totals the EV weights. Negative individual weights are possible in
// the schema (decaying EV), but the sum is only used as a normalizer when
// strictly positive.
func sumWeights(weights map[string]float64) float64 {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	return total
}
