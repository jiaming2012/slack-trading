package scanner

import (
	"fmt"
	"math"
	"strings"
)

const (
	minAvgDailyVolume20d = 500_000.0
	minPrice             = 5.0
	minMarketCap         = 300_000_000.0

	// earningsWindowDays is the number of calendar days (before or after
	// scanned_at) within which a scheduled earnings announcement disqualifies
	// a ticker from the data quality gate.
	earningsWindowDays = 3
)

// Layer1Result aggregates the outcome of all three Layer 1 gates. It never
// short-circuits: Reasons contains every failing gate's reason, not just the
// first one encountered.
type Layer1Result struct {
	Pass    bool
	Reasons []string
}

// LiquidityGate admits a ticker only when all three conditions hold strictly:
// 20-day average daily volume > 500,000 shares, current price > $5, and
// market capitalization > $300,000,000.
func LiquidityGate(input ScanInput) (bool, string) {
	var reasons []string

	if !(input.AvgDailyVolume20d > minAvgDailyVolume20d) {
		reasons = append(reasons, fmt.Sprintf(
			"avg_daily_volume_20d %.0f does not exceed floor %.0f", input.AvgDailyVolume20d, minAvgDailyVolume20d))
	}
	if !(input.Price > minPrice) {
		reasons = append(reasons, fmt.Sprintf(
			"price %.2f does not exceed floor %.2f", input.Price, minPrice))
	}
	if !(input.MarketCap > minMarketCap) {
		reasons = append(reasons, fmt.Sprintf(
			"market_cap %.0f does not exceed floor %.0f", input.MarketCap, minMarketCap))
	}

	if len(reasons) > 0 {
		return false, strings.Join(reasons, "; ")
	}
	return true, ""
}

// DataQualityGate admits a ticker only when all three conditions hold: no
// earnings announcement is scheduled within earningsWindowDays calendar days
// of scanned_at (before or after), the ticker has no recent trading halt
// flagged, and an options chain is confirmed to exist.
func DataQualityGate(input ScanInput) (bool, string) {
	var reasons []string

	if input.DaysToNextEarnings != nil {
		days := *input.DaysToNextEarnings
		if math.Abs(float64(days)) <= float64(earningsWindowDays) {
			reasons = append(reasons, fmt.Sprintf(
				"earnings within %d days (days_to_next_earnings=%d)", earningsWindowDays, days))
		}
	}
	if input.RecentHalt {
		reasons = append(reasons, "recent trading halt flagged")
	}
	if !input.HasOptionsChain {
		reasons = append(reasons, "no options chain available")
	}

	if len(reasons) > 0 {
		return false, strings.Join(reasons, "; ")
	}
	return true, ""
}

// RegimeFilter applies an ATR% ceiling and a price-vs-50-day-moving-average
// bound whose thresholds vary by RegimeTag, using the built-in default
// thresholds in regimeThresholdsByTag. A ticker whose RegimeTag is empty or
// unrecognized is not rejected -- the filter is a no-op when no regime
// classification is available.
func RegimeFilter(input ScanInput) (bool, string) {
	thresholds, ok := regimeThresholdsByTag[input.RegimeTag]
	if !ok {
		return true, ""
	}

	var reasons []string

	if atrPct := ComputeATRPct(input.Candles, input.Price); atrPct != nil && *atrPct > thresholds.ATRPctCeiling {
		reasons = append(reasons, fmt.Sprintf(
			"atr_pct %.4f exceeds %s ceiling %.4f", *atrPct, input.RegimeTag, thresholds.ATRPctCeiling))
	}

	closes := closesFromCandles(input.Candles)
	if priceVs50MA := ComputePriceVs50MA(closes, input.Price); priceVs50MA != nil {
		if math.Abs(*priceVs50MA) > thresholds.PriceVs50MABound {
			reasons = append(reasons, fmt.Sprintf(
				"price_vs_50ma %.2f exceeds %s bound +/-%.2f", *priceVs50MA, input.RegimeTag, thresholds.PriceVs50MABound))
		}
	}

	if len(reasons) > 0 {
		return false, strings.Join(reasons, "; ")
	}
	return true, ""
}

// RunLayer1 runs all three Layer 1 gates and aggregates every failure reason
// -- it never short-circuits after the first failing gate.
func RunLayer1(input ScanInput) Layer1Result {
	var reasons []string

	if pass, reason := LiquidityGate(input); !pass {
		reasons = append(reasons, reason)
	}
	if pass, reason := DataQualityGate(input); !pass {
		reasons = append(reasons, reason)
	}
	if pass, reason := RegimeFilter(input); !pass {
		reasons = append(reasons, reason)
	}

	return Layer1Result{
		Pass:    len(reasons) == 0,
		Reasons: reasons,
	}
}
